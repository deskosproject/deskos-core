package compiler_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/plan"
	"github.com/deskosproject/deskos-core/internal/schema"
)

// Every registered provider has a schema, and every kind schema has a provider.
func TestSchemasMatchProviders(t *testing.T) {
	c := newCompiler(t)
	v, err := schema.New()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, p := range v.Paths() {
		if !strings.HasPrefix(p, "common/") {
			have[p] = true
		}
	}
	for _, p := range c.Registry.Providers() {
		if !have[p.Schema()] {
			t.Errorf("%s: schema %s not found", p.GVK(), p.Schema())
		}
		delete(have, p.Schema())
	}
	for p := range have {
		t.Errorf("schema %s has no provider", p)
	}
	// UpdatePolicy became the eleventh kind (ADR 0007), TrustAnchor the
	// twelfth (ADR 0009) and Theme the thirteenth (ADR 0013), each by an
	// explicit decision.
	if n := len(c.Registry.Providers()); n != 13 {
		t.Errorf("v1alpha1 exposes %d kinds, want exactly 13", n)
	}
}

// Unknown spec fields (for example an attempt to add a shell hook) are
// rejected by the JSON Schema and, independently, by typed decoding.
func TestUnknownFieldsRejectedBySchemaAndTypes(t *testing.T) {
	docs := map[string]string{
		"PackageSet":     "apiVersion: software.deskos.org/v1alpha1\nkind: PackageSet\nmetadata: {name: x}\nspec:\n  packages: [git]\n  script: echo hi\n",
		"RpmRepository":  "apiVersion: software.deskos.org/v1alpha1\nkind: RpmRepository\nmetadata: {name: x}\nspec:\n  id: x\n  displayName: x\n  baseURL: https://example.org/x\n  gpgKeyFiles: [keys/x.asc]\n  command: curl | bash\n",
		"GnomeProfile":   "apiVersion: desktop.deskos.org/v1alpha1\nkind: GnomeProfile\nmetadata: {name: x}\nspec:\n  defaults:\n    dconf: {org/gnome/x: y}\n",
		"Profile":        "apiVersion: core.deskos.org/v1alpha1\nkind: Profile\nmetadata: {name: x}\nspec:\n  layer: role\n  resources: []\n  postInstall: x\n",
		"Workstation":    "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata: {name: x}\nspec:\n  platformRef: p\n  profiles: [a]\n  hooks: [x]\n",
		"FlatpakSet":     "apiVersion: software.deskos.org/v1alpha1\nkind: FlatpakSet\nmetadata: {name: x}\nspec:\n  remote: r\n  applications: [{id: org.example.App, shell: x}]\n",
		"BootProfile":    "apiVersion: system.deskos.org/v1alpha1\nkind: BootProfile\nmetadata: {name: x}\nspec:\n  splash: graphical\n  kernelArguments: [init=/bin/sh]\n",
		"TrustAnchor":    "apiVersion: system.deskos.org/v1alpha1\nkind: TrustAnchor\nmetadata: {name: x}\nspec:\n  anchors: [{name: x, file: x.crt, script: echo hi}]\n",
		"BinaryArtifact": "apiVersion: software.deskos.org/v1alpha1\nkind: BinaryArtifact\nmetadata: {name: x}\nspec:\n  version: '1'\n  source: {url: https://example.org/1/x, sha256: " + strings.Repeat("a", 64) + "}\n  archive: none\n  files: [{destination: /usr/local/bin/x}]\n  preInstall: x\n",
	}
	c := newCompiler(t)
	for kind, doc := range docs {
		t.Run(kind, func(t *testing.T) {
			_, err := c.Load([]string{fixture{"x.yaml": doc}.dir(t)})
			mustFail(t, err, "schema validation failed")
		})
	}
	for kind, spec := range map[string]string{
		"PackageSet":   `{"packages":["git"],"script":"x"}`,
		"GnomeProfile": `{"defaults":{"dconf":{}}}`,
		"BootProfile":  `{"splash":"graphical","script":"x"}`,
		"TrustAnchor":  `{"anchors":[{"name":"x","file":"x.crt","script":"x"}]}`,
	} {
		t.Run(kind+" typed", func(t *testing.T) {
			group := "software.deskos.org/v1alpha1"
			switch kind {
			case "GnomeProfile":
				group = "desktop.deskos.org/v1alpha1"
			case "BootProfile", "TrustAnchor":
				group = "system.deskos.org/v1alpha1"
			}
			res := rawResource(group, kind, "x", spec)
			p, err := c.Registry.Lookup(res.GVK())
			if err != nil {
				t.Fatal(err)
			}
			mustFail(t, p.Decode(res), "unknown field")
		})
	}
}

type coreView struct {
	Groups, Packages, Units, Dconf, Files []string
	Target                                string
}

func viewOf(p *plan.Plan) coreView {
	var v coreView
	for _, g := range p.Artifact.PackageGroups {
		v.Groups = append(v.Groups, g.Name+"="+strings.Join(g.Members, ","))
	}
	for _, x := range p.Artifact.Packages {
		v.Packages = append(v.Packages, x.Name)
	}
	for _, x := range p.Artifact.SystemdUnits {
		v.Units = append(v.Units, x.Unit)
	}
	for _, x := range p.Artifact.Dconf.Defaults {
		v.Dconf = append(v.Dconf, x.Key+"="+x.Value)
	}
	for _, x := range p.Artifact.Files {
		v.Files = append(v.Files, x.Path+"@"+x.SHA256)
	}
	if p.Artifact.DefaultTarget != nil {
		v.Target = p.Artifact.DefaultTarget.Target
	}
	return v
}

// The same DeskOS Core semantics compose to the same planned artifact content
// on CentOS Stream 10 and RHEL 10; only the base image differs. This proves
// composition, not that the RHEL image builds or boots.
func TestCoreComposesIdenticallyOnBothPlatforms(t *testing.T) {
	ws := func(name, platform string) string {
		return "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: " + name + "\nspec:\n  platformRef: " + platform + "\n  profiles: [deskos-core]\n"
	}
	dir := fixture{"a.yaml": ws("core-centos", "centos-stream-10"), "b.yaml": ws("core-rhel", "rhel-10")}.dir(t)
	centos := mustPlan(t, "core-centos", resourcesRoot, dir)
	rhel := mustPlan(t, "core-rhel", resourcesRoot, dir)
	if !reflect.DeepEqual(viewOf(centos), viewOf(rhel)) {
		t.Fatalf("DeskOS Core differs between platforms:\ncentos %+v\nrhel   %+v", viewOf(centos), viewOf(rhel))
	}
	if centos.Artifact.BaseImage.Image == rhel.Artifact.BaseImage.Image {
		t.Fatal("platforms share a base image")
	}
	if !strings.HasPrefix(rhel.Artifact.BaseImage.Image, "registry.redhat.io/rhel10/rhel-bootc") {
		t.Errorf("RHEL base is %s, want the official rhel-bootc image", rhel.Artifact.BaseImage.Image)
	}
}

// An organization requirement the platform cannot meet fails with the
// platform's reason instead of being dropped or substituted.
func TestExampleVirtManagerGapIsReported(t *testing.T) {
	dir := fixture{
		"p.yaml": profile("needs-virt-manager", "role", "PackageSet/example-virt-manager"),
		"c.yaml": "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: gap-centos\nspec:\n  platformRef: centos-stream-10\n  profiles: [deskos-core, example-baseline, example-devops, needs-virt-manager]\n",
		"r.yaml": "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: gap-rhel\nspec:\n  platformRef: rhel-10\n  profiles: [deskos-core, example-baseline, example-devops, needs-virt-manager]\n",
	}.dir(t)
	for ws, want := range map[string]string{"gap-centos": "Platform/centos-stream-10: only in the CentOS Stream CRB", "gap-rhel": "Platform/rhel-10: RHEL 10 moved it to the CodeReady Linux Builder"} {
		_, err := planOf(t, ws, resourcesRoot, exampleRoot, dir)
		mustFail(t, err, `package "virt-manager" is not available on `+strings.SplitN(want, ":", 2)[0], strings.SplitN(want, ": ", 2)[1], "PackageSet/example-virt-manager")
	}
	if slices.Contains(packages(mustPlan(t, "example-devops-rhel10", resourcesRoot, exampleRoot)), "virt-manager") {
		t.Error("virt-manager must not be installed silently")
	}
}

// A repository kind belongs to one package manager's world. Asking a deb platform
// for an RPM repository is a gap to report, not a file to copy where nothing
// reads it.
func TestRpmRepositoryOnADebPlatformIsReported(t *testing.T) {
	deb := strings.Replace(platformYAML, "  packageManager: rpm\n", "  packageManager: deb\n", 1)
	deb = strings.Replace(deb, "rpmGroups: [workstation-product-environment]", "members: [test-desktop]", 1)
	dir := fixture{
		"platform.yaml": deb,
		"p.yaml":        profile("core", "foundation", "RpmRepository/epel"),
		"repo.yaml":     "apiVersion: software.deskos.org/v1alpha1\nkind: RpmRepository\nmetadata:\n  name: epel\nspec:\n  id: epel\n  displayName: EPEL\n  baseURL: https://example.org/epel/\n  gpgCheck: false\n",
		"ws.yaml":       workstation("ws", "core"),
	}.dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, "an RPM repository requires an RPM platform", "Platform/test-platform is deb", "RpmRepository/epel")
}

// ADR 0014 wants the owner and the tier recorded in the Platform resource, and says
// an unnamed owner is not a tier but an omission. The schema types both, so a
// platform cannot name an empty owner or a tier that does not exist.
func TestPlatformOwnerAndTierAreTyped(t *testing.T) {
	with := func(fields string) string {
		return strings.Replace(platformYAML, "  displayName: Test Platform\n",
			"  displayName: Test Platform\n"+fields, 1)
	}
	c := newCompiler(t)
	for name, doc := range map[string]string{
		"empty owner":   with("  owner: \"\"\n  tier: organization\n"),
		"unknown tier":  with("  owner: The Example Team\n  tier: partner\n"),
		"tier no owner": with("  tier: organization\n"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.Load([]string{fixture{"platform.yaml": doc}.dir(t)})
			mustFail(t, err, "schema validation failed")
		})
	}
	if _, err := c.Load([]string{fixture{"platform.yaml": with("  owner: The Example Team\n  tier: organization\n")}.dir(t)}); err != nil {
		t.Errorf("a declared owner and tier should load: %v", err)
	}
}

// A platform that cannot preinstall system Flatpaks says so, and a FlatpakSet
// asked of it fails with that reason instead of compiling something that would
// never install (invariant 9).
func TestFlatpakSetNeedsPreinstallAndThePlatformSaysSo(t *testing.T) {
	noPreinstall := strings.Replace(platformYAML,
		"  flatpak: {package: flatpak, preinstall: true}\n",
		"  flatpak: {preinstall: false}\n", 1)
	dir := fixture{
		"platform.yaml": noPreinstall,
		"p.yaml":        profile("core", "foundation", "FlatpakSet/deskos-reference-apps"),
		"ws.yaml":       workstation("ws", "core"),
	}.dir(t)
	_, err := planOf(t, "ws", resourcesRoot, dir)
	mustFail(t, err, "does not support Flatpak preinstallation", "Platform/test-platform", "FlatpakSet/deskos-reference-apps")
}

// Ubuntu 26.04 offers no system Flatpak channel, and the composition says so: with
// this platform, the Plan asks for no Flatpak package, no preinstall unit and no
// Flatpak update timer. 26.04 publishes no `flatpak preinstall` and the base it
// pins carries no third-party apt source, so the platform declares the gap instead
// of pretending (invariant 9). What this test covers is the Plan; that the built
// image carries no Flatpak is a separate measurement of the image, recorded in the
// validation table of docs/architecture.md.
func TestUbuntuDeclaresNoSystemFlatpakChannel(t *testing.T) {
	dir := fixture{
		"ws.yaml": "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: ubuntu-ws\nspec:\n  platformRef: ubuntu-2604\n  profiles: [deskos-core]\n",
	}.dir(t)
	p := mustPlan(t, "ubuntu-ws", resourcesRoot, dir)
	if got := packages(p); slices.Contains(got, "flatpak") {
		t.Errorf("packages = %v, want no Flatpak: Ubuntu declares no system Flatpak channel", got)
	}
	for _, u := range p.Artifact.SystemdUnits {
		if strings.Contains(u.Unit, "flatpak") {
			t.Errorf("unit %s is enabled: the RHEL family's application channel is not Ubuntu's", u.Unit)
		}
	}
	for _, s := range p.Artifact.ScheduledUpdates {
		if s.Kind == plan.UpdateFlatpak {
			t.Errorf("scheduled update %s: want none on Ubuntu", s.Service)
		}
	}
}

// Organization- and role-specific software never appears in DeskOS Core.
func TestCoreCarriesNoOrganizationalSoftware(t *testing.T) {
	p := mustPlan(t, "deskos-core-centos10", resourcesRoot)
	forbidden := []string{"libvirt", "qemu-kvm", "virt-install", "virt-manager", "gnome-boxes", "terraform",
		"kubectl", "code", "google-chrome-stable", "zerotier-one", "tailscale", "ipa-client", "cockpit-machines"}
	for _, pkg := range packages(p) {
		if slices.Contains(forbidden, pkg) {
			t.Errorf("DeskOS Core installs %s", pkg)
		}
	}
	if len(p.Artifact.RpmRepositories) != 0 || len(p.Artifact.Binaries) != 0 {
		t.Error("DeskOS Core adds third-party repositories or binaries")
	}
	if p.Artifact.Dconf != nil && len(p.Artifact.Dconf.Locks) != 0 {
		t.Error("DeskOS Core locks settings; Core provides defaults only")
	}

	example := mustPlan(t, "example-devops-rhel10", resourcesRoot, exampleRoot)
	for _, want := range []string{"libvirt", "virt-install", "terraform", "kubectl", "code", "google-chrome-stable"} {
		if !slices.Contains(packages(example), want) {
			t.Errorf("the example DevOps workstation is missing %s", want)
		}
	}
	// The DeskOS look is a workstation's own choice, so the floor an organization
	// builds on is Core's foundation, not the reference workstation's look.
	foundation := mustPlan(t, "foundation-ws", resourcesRoot,
		fixture{"ws.yaml": "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: foundation-ws\nspec:\n  platformRef: centos-stream-10\n  profiles: [deskos-core, rhel-family]\n"}.dir(t))
	for _, pkg := range packages(foundation) {
		if pkg != "flatpak" && !slices.Contains(packages(example), pkg) {
			t.Errorf("the example workstation lost Core package %s", pkg)
		}
	}
}

// Structural rules: backends see only the Plan, and no compiler code can
// execute commands or reach the network.
func TestImportBoundaries(t *testing.T) {
	root := "../.."
	rules := []struct {
		dir       string
		forbidden []string
	}{
		{"internal/backends", []string{"/internal/loader", "/internal/compose", "/internal/providers", "/internal/schema", "yaml"}},
		{"internal/plan", []string{"/internal/loader", "/internal/backends", "yaml"}},
		{"internal", []string{"os/exec", "net/http", "net"}},
	}
	for _, r := range rules {
		err := filepath.Walk(filepath.Join(root, r.dir), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, bad := range r.forbidden {
					if path == bad || (strings.Contains(bad, "/") && strings.Contains(path, bad)) || (bad == "yaml" && strings.Contains(path, "yaml")) {
						t.Errorf("%s imports %s", p, path)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// make fmt-check must fail when gofmt cannot run instead of reading its
// empty output as "nothing to format".
func TestMakeFmtCheckFailsWithoutGofmt(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed")
	}
	cmd := exec.Command("make", "-s", "-C", "../..", "fmt-check", "GOFMT=/nonexistent/gofmt")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("fmt-check passed without gofmt:\n%s", out)
	}
}

// CS10 Core leaves out setroubleshoot only through the group exclusion, and no
// rendered file weakens SELinux enforcement or audit logging.
func TestCoreKeepsSELinuxAndAudit(t *testing.T) {
	cs10, err := containerfile.Render(mustPlan(t, "deskos-core-centos10", resourcesRoot))
	if err != nil {
		t.Fatal(err)
	}
	cf := string(fileData(t, cs10, containerfile.ContainerfilePath))
	group, _, _ := strings.Cut(cf[strings.Index(cf, "dnf -y group install"):], "\n\n")
	for _, x := range []string{"setroubleshoot", "setroubleshoot-plugins", "setroubleshoot-server"} {
		if !strings.Contains(group, "'--exclude="+x+"' \\\n") {
			t.Errorf("the CS10 group transaction does not exclude %s", x)
		}
	}
	rhel, err := containerfile.Render(mustPlan(t, "example-devops-rhel10", resourcesRoot, exampleRoot))
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"setenforce", "SELINUX=disabled", "SELINUX=permissive", "selinux=0", "enforcing=0",
		"permissive", "dontaudit", "semodule", "semanage", "auditctl", "audit=0", "auditd", "audit.rules", "/etc/selinux"}
	for _, f := range append(cs10, rhel...) {
		for _, bad := range forbidden {
			if bytes.Contains(bytes.ToLower(f.Data), []byte(strings.ToLower(bad))) || strings.Contains(f.Path, bad) {
				t.Errorf("%s contains %q", f.Path, bad)
			}
		}
	}
}
