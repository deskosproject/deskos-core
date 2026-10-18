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
	// twelfth (ADR 0009), Theme the thirteenth (ADR 0013) and AptRepository the
	// fourteenth (ADR 0015), each by an explicit decision.
	if n := len(c.Registry.Providers()); n != 14 {
		t.Errorf("v1alpha1 exposes %d kinds, want exactly 14", n)
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

// coreView is the slice of a Plan this file's cross-platform test compares. Units
// holds *enabled* units; a mask lives in the Plan's SystemdMasks and is asserted
// where a platform differs, not compared here.
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
	// A platform that keeps its vendor's desktop has no dconf database at all, so
	// this view must not assume one (docs/adr/0016).
	if p.Artifact.Dconf != nil {
		for _, x := range p.Artifact.Dconf.Defaults {
			v.Dconf = append(v.Dconf, x.Key+"="+x.Value)
		}
	}
	for _, x := range p.Artifact.Files {
		v.Files = append(v.Files, x.Path+"@"+x.SHA256)
	}
	if p.Artifact.DefaultTarget != nil {
		v.Target = p.Artifact.DefaultTarget.Target
	}
	return v
}

// The same DeskOS Core semantics produce the same planned content on the RPM
// platforms — CentOS Stream 10, RHEL 10, AlmaLinux 10 and Fedora 44, plus the
// Fedora 45 evaluation from its own example root — over exactly the fields viewOf
// compares: package groups (name and members), packages, systemd units, dconf
// default keys, file paths with their checksums, and the default target.
//
// It does not compare the rest of the Plan, and a reader should not read it as
// if it did: viewOf reads the group mapping, the packages, the *enabled* systemd
// units, the dconf defaults, the installed files and the default target. The
// masked units, the scheduled update policy, the boot profile (kernel arguments,
// initramfs and Plymouth), Flatpak remotes and applications, provisioning, and
// per-value provenance all sit outside it, and they are asserted per platform
// where they differ — see TestFedoraDeclaresASystemFlatpakChannel and the mask
// and group assertions below.
//
// Fedora is here because it is an RPM platform whose group mapping and update
// units are the same names as the EL10 family's (measured: Fedora's comps resolves
// `workstation-product-environment` and its base ships
// bootc-fetch-apply-updates.timer/.service). What keeps it honest is that the test
// asserts the *base* differs per platform, so an accident that made two platforms
// share a base fails here. Ubuntu is not compared: its package manager is deb.
func TestCoreComposesIdenticallyOnTheRpmPlatforms(t *testing.T) {
	ws := func(name, platform string) string {
		return "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: " + name + "\nspec:\n  platformRef: " + platform + "\n  profiles: [deskos-core, deskos-gnome]\n"
	}
	dir := fixture{
		"a.yaml": ws("core-centos", "centos-stream-10"),
		"b.yaml": ws("core-rhel", "rhel-10"),
		"c.yaml": ws("core-alma", "almalinux-10"),
		"d.yaml": ws("core-fedora44", "fedora-44"),
		"e.yaml": ws("core-fedora45", "fedora-45"),
	}.dir(t)
	centos := mustPlan(t, "core-centos", resourcesRoot, dir)
	rhel := mustPlan(t, "core-rhel", resourcesRoot, dir)
	alma := mustPlan(t, "core-alma", resourcesRoot, dir)
	fedora44 := mustPlan(t, "core-fedora44", resourcesRoot, dir)
	// Fedora 45 is an evaluation, not a shipped platform, so its Platform resource
	// comes from the example root that carries it.
	fedora45 := mustPlan(t, "core-fedora45", resourcesRoot, fedora45Root, dir)
	for _, p := range []*plan.Plan{rhel, alma, fedora44, fedora45} {
		if !reflect.DeepEqual(viewOf(centos), viewOf(p)) {
			t.Fatalf("DeskOS Core differs from CentOS Stream 10:\ncentos %+v\nother  %+v", viewOf(centos), viewOf(p))
		}
	}
	if centos.Artifact.BaseImage.Image == rhel.Artifact.BaseImage.Image {
		t.Fatal("platforms share a base image")
	}
	if !strings.HasPrefix(rhel.Artifact.BaseImage.Image, "registry.redhat.io/rhel10/rhel-bootc") {
		t.Errorf("RHEL base is %s, want the official rhel-bootc image", rhel.Artifact.BaseImage.Image)
	}
	if !strings.HasPrefix(alma.Artifact.BaseImage.Image, "quay.io/almalinuxorg/almalinux-bootc") {
		t.Errorf("AlmaLinux base is %s, want the vendor's almalinux-bootc image", alma.Artifact.BaseImage.Image)
	}
	for name, p := range map[string]*plan.Plan{"fedora-44": fedora44, "fedora-45": fedora45} {
		if !strings.HasPrefix(p.Artifact.BaseImage.Image, "quay.io/fedora/fedora-silverblue:") {
			t.Errorf("%s base is %s, want the vendor's fedora-silverblue image", name, p.Artifact.BaseImage.Image)
		}
	}
	if fedora44.Artifact.BaseImage.Digest == fedora45.Artifact.BaseImage.Digest {
		t.Error("the two Fedora releases pin the same base digest")
	}
	// The Fedora facts that reach the Plan and are worth keeping: the group id
	// DeskOS asks dnf for, which on Fedora is a comps *environment* resolved through
	// `group install`, and the bootc units it masks so its own staging timer is the
	// only update path. viewOf reads enabled units, so the masks are read here.
	masked := func(p *plan.Plan) []string {
		var out []string
		for _, m := range p.Artifact.SystemdMasks {
			out = append(out, m.Unit)
		}
		return out
	}
	for _, p := range []*plan.Plan{fedora44, fedora45} {
		if got, want := viewOf(p).Groups, []string{"workstation=workstation-product-environment"}; !reflect.DeepEqual(got, want) {
			t.Errorf("group mapping = %v, want %v", got, want)
		}
		if got, want := masked(p), []string{"bootc-fetch-apply-updates.service", "bootc-fetch-apply-updates.timer"}; !reflect.DeepEqual(got, want) {
			t.Errorf("masked units = %v, want %v", got, want)
		}
	}
}

// Fedora's base brings Flatpak with the preinstall mechanism, and the platform
// says so: a FlatpakSet on Fedora compiles to the Flatpak package, the preinstall
// unit and the application. This is the pair the Ubuntu test asserts the absence
// of, and it is the platform fact the RPM comparison above does not cover.
func TestFedoraDeclaresASystemFlatpakChannel(t *testing.T) {
	dir := fixture{
		"p.yaml":  profile("apps", "foundation", "FlatpakSet/deskos-reference-apps"),
		"ws.yaml": "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: fedora-ws\nspec:\n  platformRef: fedora-44\n  profiles: [apps]\n",
	}.dir(t)
	p := mustPlan(t, "fedora-ws", resourcesRoot, dir)
	if got := packages(p); !slices.Contains(got, "flatpak") {
		t.Errorf("packages = %v, want Flatpak: the Fedora platform declares the channel", got)
	}
	if p.Provisioning.FlatpakPreinstallUnit == "" {
		t.Error("no Flatpak preinstall unit on a platform that declares preinstall")
	}
	if n := len(p.Provisioning.FlatpakApplications); n != 1 {
		t.Errorf("compiled %d Flatpak applications, want 1 (the Flathub reference set)", n)
	}
	if len(p.Provisioning.FlatpakRemotes) == 0 {
		t.Error("no Flatpak remote on a platform that declares the channel")
	}
	for _, a := range p.Provisioning.FlatpakRemotes {
		if a.GPGKey == "" {
			t.Errorf("remote %s carries no GPG key", a.Name)
		}
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

// A platform with no icon cache tool says so, instead of shipping a desktop entry
// whose icon never appears: copying icons leaves the theme's cache stale and no
// package trigger runs for files the build writes (invariant 9). Which package
// provides the tool, and how it is invoked, are the platform's facts — so a
// platform that declares neither reports the gap rather than having Core guess a
// name that is another platform's.
func TestDesktopEntryNeedsThePlatformIconCache(t *testing.T) {
	without := strings.Replace(platformYAML,
		"  iconCache: {package: gtk-update-icon-cache, command: \"gtk-update-icon-cache --force --quiet\"}\n", "", 1)
	if without == platformYAML {
		t.Fatal("the fixture no longer declares iconCache, so this test proves nothing")
	}
	files := base.with(fixture{
		"a.yaml":            profile("a", "organization", "BinaryArtifact/ghostty"),
		"ghostty.yaml":      appArtifact("ghostty", "/usr/local/bin/ghostty", ghosttyEntry),
		"icons/ghostty.png": pngOf(t, 128, 128),
		"platform.yaml":     without,
		"ws.yaml":           workstation("ws", "a"),
	})
	_, err := planOf(t, "ws", files.dir(t))
	mustFail(t, err, "declares no iconCache", "Platform/test-platform", "BinaryArtifact/ghostty")
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
	if len(p.Artifact.RpmRepositories) != 0 {
		t.Error("DeskOS Core adds third-party repositories")
	}
	// Core carries exactly one binary, and it is Core's own: the runtime tool every
	// image ships so a machine can say what it was built from (ADR 0018). Anything
	// else here would be a customer's software, which is what this test refuses.
	for _, b := range p.Artifact.Binaries {
		if b.Artifact != "deskos" {
			t.Errorf("DeskOS Core installs the third-party binary %s", b.Artifact)
		}
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
