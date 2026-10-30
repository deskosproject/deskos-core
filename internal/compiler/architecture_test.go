package compiler_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/plan"
	softwares "github.com/deskosproject/deskos-core/internal/providers/software"
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
	// twelfth (ADR 0009), Theme the thirteenth (ADR 0013), AptRepository the
	// fourteenth (ADR 0015) and NetworkProfile the fifteenth (ADR 0027), each by
	// an explicit decision.
	if n := len(c.Registry.Providers()); n != 15 {
		t.Errorf("v1alpha1 exposes %d kinds, want exactly 15", n)
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
	Groups, Packages, Removed, Units, Dconf, Files []string
	Target                                         string
}

func viewOf(p *plan.Plan) coreView {
	var v coreView
	for _, g := range p.Artifact.PackageGroups {
		v.Groups = append(v.Groups, g.Name+"="+strings.Join(g.Members, ","))
	}
	for _, x := range p.Artifact.Packages {
		v.Packages = append(v.Packages, x.Name)
	}
	for _, x := range p.Artifact.RemovePackages {
		v.Removed = append(v.Removed, x.Name)
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

// The same DeskOS Core semantics produce the same planned content on the shipped
// RPM platforms — CentOS Stream 10 and RHEL 10 — over exactly the fields viewOf
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
// The examples (AlmaLinux and Fedora) are not part of this shipped-platform
// comparison; they have their own roots and golden renders. Ubuntu is not
// compared: its package manager is deb.
func TestCoreComposesIdenticallyOnTheRpmPlatforms(t *testing.T) {
	ws := func(name, platform string) string {
		return "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: " + name + "\nspec:\n  platformRef: " + platform + "\n  profiles: [deskos-core, deskos-gnome, deskos-dock]\n"
	}
	dir := fixture{
		"a.yaml": ws("core-centos", "centos-stream-10"),
		"b.yaml": ws("core-rhel", "rhel-10"),
	}.dir(t)
	centos := mustPlan(t, "core-centos", resourcesRoot, dir)
	rhel := mustPlan(t, "core-rhel", resourcesRoot, dir)
	// The tray extension is delivered as the pinned v66 artifact on every shipped
	// platform. Compare the rest of the package list, then assert the delivery
	// exactly instead of hiding it.
	withoutDelivery := func(v coreView) coreView {
		out := v
		out.Packages = nil
		for _, name := range v.Packages {
			if name != "gnome-shell-extension-appindicator" {
				out.Packages = append(out.Packages, name)
			}
		}
		return out
	}
	for _, p := range []*plan.Plan{rhel} {
		if !reflect.DeepEqual(withoutDelivery(viewOf(centos)), withoutDelivery(viewOf(p))) {
			t.Fatalf("DeskOS Core differs from CentOS Stream 10:\ncentos %+v\nother  %+v", viewOf(centos), viewOf(p))
		}
	}
	const trayUUID = "appindicatorsupport@rgcjonas.gmail.com"
	const dockUUID = "dash-to-dock@micxgx.gmail.com"
	for name, p := range map[string]*plan.Plan{"centos-stream-10": centos, "rhel-10": rhel} {
		if slices.Contains(packages(p), "gnome-shell-extension-appindicator") ||
			slices.Contains(packages(p), "gnome-shell-extension-dash-to-dock") {
			t.Errorf("%s installs a DeskOS extension as a package", name)
		}
		got := map[string]bool{}
		for _, e := range p.Artifact.Extensions {
			got[e.UUID] = true
		}
		if len(p.Artifact.Extensions) != 2 || !got[trayUUID] || !got[dockUUID] {
			t.Errorf("%s delivers %+v, want the tray and dock artifacts", name, p.Artifact.Extensions)
		}
	}
	if centos.Artifact.BaseImage.Image == rhel.Artifact.BaseImage.Image {
		t.Fatal("platforms share a base image")
	}
	if !strings.HasPrefix(rhel.Artifact.BaseImage.Image, "registry.redhat.io/rhel10/rhel-bootc") {
		t.Errorf("RHEL base is %s, want the official rhel-bootc image", rhel.Artifact.BaseImage.Image)
	}
	wantRemoved := []string{"gnome-software", "gnome-software-rpm-ostree"}
	for name, p := range map[string]*plan.Plan{"centos-stream-10": centos, "rhel-10": rhel} {
		if got := viewOf(p).Removed; !reflect.DeepEqual(got, wantRemoved) {
			t.Errorf("%s removes %v, want %v", name, got, wantRemoved)
		}
	}
}

// Disabling the tray UUID delivers nothing, whatever the platform's catalog shape:
// no package, no pinned artifact and no enabled default. The decision is a profile's
// and the delivery is the platform's (docs/adr/0021); this is the inverse of the
// delivery assertions above.
func TestDisablingTheTrayExtensionDeliversNothing(t *testing.T) {
	const trayUUID = "appindicatorsupport@rgcjonas.gmail.com"
	for _, platform := range []string{"centos-stream-10", "rhel-10"} {
		t.Run(platform, func(t *testing.T) {
			dir := fixture{
				"ws.yaml":  "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: ws\nspec:\n  platformRef: " + platform + "\n  profiles: [deskos-core, deskos-gnome, off]\n",
				"off.yaml": "apiVersion: core.deskos.org/v1alpha1\nkind: Profile\nmetadata:\n  name: off\nspec:\n  layer: organization\n  resources:\n    - {kind: GnomeProfile, name: off-gnome}\n",
				"g.yaml":   "apiVersion: desktop.deskos.org/v1alpha1\nkind: GnomeProfile\nmetadata:\n  name: off-gnome\nspec:\n  defaults:\n    shell:\n      extensions:\n        - {uuid: " + trayUUID + ", enabled: false}\n",
			}.dir(t)
			p := mustPlan(t, "ws", resourcesRoot, dir)
			if slices.Contains(packages(p), "gnome-shell-extension-appindicator") {
				t.Error("a disabled extension installs the package")
			}
			if len(p.Artifact.Extensions) != 0 {
				t.Errorf("a disabled extension delivers %+v", p.Artifact.Extensions)
			}
			if slices.Contains(p.Artifact.EnabledExtensions, trayUUID) {
				t.Error("a disabled extension is still listed as enabled")
			}
			if v, ok := dconf(p, "/org/gnome/shell/enabled-extensions"); ok && strings.Contains(v, trayUUID) {
				t.Errorf("a disabled extension is still in the default: %s", v)
			}
		})
	}
}

// Fedora's base brings Flatpak with the preinstall mechanism, and the platform
// says so: a FlatpakSet on Fedora compiles to the Flatpak package, the preinstall
// unit and the application. This is the pair the Ubuntu test asserts the absence
// of, and it is the platform fact the RPM comparison above does not cover.
func TestFedoraDeclaresASystemFlatpakChannel(t *testing.T) {
	dir := fixture{
		"p.yaml":  profile("apps", "foundation", "FlatpakSet/deskos-reference-apps"),
		"ws.yaml": "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: fedora-ws\nspec:\n  platformRef: fedora-45\n  profiles: [apps]\n",
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
// platform's reason instead of being dropped or substituted. The requirement is
// a fixture, not the example's: the example no longer carries a package the
// platform cannot supply, and this mechanism must be tested on its own.
func TestAPlatformGapIsReportedWithItsReason(t *testing.T) {
	dir := fixture{
		"p.yaml": profile("needs-virt-manager", "role", "PackageSet/example-virt-manager"),
		"s.yaml": packageSet("example-virt-manager", "virt-manager"),
		"c.yaml": "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: gap-centos\nspec:\n  platformRef: centos-stream-10\n  profiles: [deskos-core, example-organization, example-devops, needs-virt-manager]\n",
		"r.yaml": "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: gap-rhel\nspec:\n  platformRef: rhel-10\n  profiles: [deskos-core, example-organization, example-devops, needs-virt-manager]\n",
	}.dir(t)
	for ws, want := range map[string]string{"gap-centos": "Platform/centos-stream-10: only in the CentOS Stream CRB", "gap-rhel": "Platform/rhel-10: RHEL 10 moved it to the CodeReady Linux Builder"} {
		_, err := planOf(t, ws, resourcesRoot, exampleRoot, dir)
		mustFail(t, err, `package "virt-manager" is not available on `+strings.SplitN(want, ":", 2)[0], strings.SplitN(want, ": ", 2)[1], "PackageSet/example-virt-manager")
	}
	if slices.Contains(packages(mustPlan(t, "example-devops-rhel-10", resourcesRoot, exampleRoot)), "virt-manager") {
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
// A plan that downloads has to know how its base fetches, and a platform that does
// not say fails composition instead of the build failing on a missing command. The
// verifiable artifact the download checks is the point: without it the image cannot
// be built at all (invariant 9, docs/adr/0019).
func TestDownloadsAreThePlatformsFact(t *testing.T) {
	binary := appArtifact("ghostty", "/usr/local/bin/ghostty", "")
	withDownloads := func(block string) string {
		return strings.Replace(platformYAML, "  downloads: {baseFetches: true}\n", block, 1)
	}
	plan := func(platform string) (*plan.Plan, error) {
		dir := fixture{
			"platform.yaml": platform,
			"p.yaml":        profile("core", "foundation", "BinaryArtifact/ghostty"),
			"ws.yaml":       workstation("ws", "core"),
			"a.yaml":        binary,
		}.dir(t)
		return planOf(t, "ws", dir)
	}

	t.Run("a platform that says nothing fails", func(t *testing.T) {
		_, err := plan(strings.Replace(platformYAML, "  downloads: {baseFetches: true}\n", "", 1))
		mustFail(t, err, "does not say whether its base can fetch a verified artifact",
			"Platform/test-platform", "ghostty", "BinaryArtifact/ghostty")
	})

	t.Run("a base that cannot fetch gets the fetcher installed", func(t *testing.T) {
		p, err := plan(withDownloads("  downloads: {baseFetches: false, package: fetch-tool}\n"))
		if err != nil {
			t.Fatal(err)
		}
		// The plan names the package and says which download needed it, so the report
		// answers "why is this installed" without reading the platform resource.
		var found bool
		for _, pkg := range p.Artifact.Packages {
			if pkg.Name != "fetch-tool" {
				continue
			}
			found = true
			if len(pkg.Provenance) == 0 || pkg.Provenance[0].Resource != "BinaryArtifact/ghostty" {
				t.Errorf("the fetcher package does not carry the download's provenance: %+v", pkg.Provenance)
			}
		}
		if !found {
			t.Errorf("the fetcher package is not in the plan: %+v", p.Artifact.Packages)
		}
	})

	t.Run("a base that fetches installs nothing extra", func(t *testing.T) {
		p, err := plan(platformYAML)
		if err != nil {
			t.Fatal(err)
		}
		for _, pkg := range p.Artifact.Packages {
			if pkg.Name == "fetch-tool" || pkg.Name == "curl" {
				t.Errorf("a base that fetches should not install a fetcher: %+v", p.Artifact.Packages)
			}
		}
	})
}

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

// A platform with no system Flatpak channel says so, and the composition believes it: no
// Flatpak package, no preinstall unit, and no Flatpak update timer, even when the
// workstation selects an update policy that schedules image updates. The Ubuntu platform was
// the real case -- it publishes no `flatpak preinstall` and its base carries no third-party
// apt source -- and that platform declares the gap in its own repository now (ADR 0020).
// What is Core's, and what this covers, is the Plan; whether a built image carries Flatpak
// is a measurement of that image, and the Ubuntu repository is where it is made.
func TestAPlatformWithNoFlatpakChannelInstallsNone(t *testing.T) {
	// The real case was the Ubuntu platform, which lives in its own repository now
	// (ADR 0020). The property is Core's, though: a platform that declares no application
	// channel must not pull one in, and a workstation on it must not enable a unit or
	// schedule an update for something that does not exist there.
	// Two platform facts, both required by the schema and both the real shape: an image
	// update unit, which is what lets a policy be applied, and no Flatpak channel, which
	// is how "no channel" is declared (no package, `preinstall: false`) -- what the Ubuntu
	// platform said before it moved (ADR 0020).
	platform := strings.Replace(debPlatform(t),
		"  flatpak: {package: flatpak, preinstall: true}\n",
		"  updates: {imageUpdateUnits: [bootc-fetch-apply-updates.timer]}\n  flatpak: {preinstall: false}\n", 1)
	dir := fixture{
		"platform.yaml": platform,
		"ws.yaml":       workstation("no-flatpak-ws", "core"),
		// The policy matters: without one this test would only prove that a composition
		// which asks for nothing gets nothing. With it, the plan has to schedule the image
		// update Core's policy declares and still schedule nothing for Flatpak.
		"p.yaml": profile("core", "foundation", "UpdatePolicy/core"),
		"u.yaml": updatePolicy("core", "  image: {automatic: true, schedule: daily, requireACPower: true}\n"),
	}.dir(t)
	p := mustPlan(t, "no-flatpak-ws", dir)
	if got := packages(p); slices.Contains(got, "flatpak") {
		t.Errorf("packages = %v, want no Flatpak: the platform declares no system Flatpak channel", got)
	}
	for _, u := range p.Artifact.SystemdUnits {
		if strings.Contains(u.Unit, "flatpak") {
			t.Errorf("unit %s is enabled: the RHEL family's application channel is not this platform's", u.Unit)
		}
	}
	var imageScheduled bool
	for _, s := range p.Artifact.ScheduledUpdates {
		if s.Kind == plan.UpdateFlatpak {
			t.Errorf("scheduled update %s: want none where the platform has no channel", s.Service)
		}
		if s.Kind == plan.UpdateImage {
			imageScheduled = true
		}
	}
	if !imageScheduled {
		t.Errorf("no image update is scheduled: a platform without a Flatpak channel still "+
			"has Core's update policy, and the composition dropped it: %+v", p.Artifact.ScheduledUpdates)
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
	p := mustPlan(t, "deskos-core-centos-stream-10", resourcesRoot)
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
	// Core carries one executable, and it is Core's own: the runtime tool every image
	// ships so a machine can say what it was built from (ADR 0018). The name alone
	// proves nothing, so this audits where it lands and where it comes from.
	executables := 0
	for _, b := range p.Artifact.Binaries {
		// A font is content, not software: the families the platform does not
		// package travel as pinned artifacts, and organization-specific software
		// still cannot (ADR 0018 as amended 2026-10-29).
		if path.Dir(b.Destination) == softwares.FontDir {
			continue
		}
		executables++
		if b.Artifact != "deskos" || b.Destination != "/usr/bin/deskos" ||
			!strings.HasPrefix(b.URL, "https://github.com/deskosproject/deskos-core/releases/download/") {
			t.Errorf("DeskOS Core installs %q to %q from %q; it may carry only its own runtime tool, from this project's releases",
				b.Artifact, b.Destination, b.URL)
		}
	}
	if executables != 1 {
		t.Fatalf("DeskOS Core installs %d executables; it carries exactly its own runtime tool", executables)
	}
	if p.Artifact.Dconf != nil && len(p.Artifact.Dconf.Locks) != 0 {
		t.Error("DeskOS Core locks settings; Core provides defaults only")
	}

	example := mustPlan(t, "example-devops-rhel-10", resourcesRoot, exampleRoot)
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
	cs10, err := containerfile.Render(mustPlan(t, "deskos-core-centos-stream-10", resourcesRoot))
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
	rhel, err := containerfile.Render(mustPlan(t, "example-devops-rhel-10", resourcesRoot, exampleRoot))
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
