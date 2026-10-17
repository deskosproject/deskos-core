package compiler_test

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/compiler"
	"github.com/deskosproject/deskos-core/internal/plan"
)

// fixture is a set of resource files keyed by relative path.
type fixture map[string]string

func (f fixture) dir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "root")
	for name, content := range f {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func (f fixture) with(extra fixture) fixture {
	out := fixture{}
	for k, v := range f {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

const platformYAML = `apiVersion: core.deskos.org/v1alpha1
kind: Platform
metadata:
  name: test-platform
spec:
  displayName: Test Platform
  family: test
  release: "1"
  architectures: [x86_64]
  packageManager: rpm
  bootc:
    image: example.org/test/bootc:1
  distribution: {redistributable: true, requiresSubscription: false}
  packageGroups:
    - {name: workstation, rpmGroups: [workstation-product-environment], graphical: true}
  displayManager: gdm.service
  gnome:
    dconfDatabase: distro
    enabledExtensions: [background-logo@fedorahosted.org]
    extensions:
      - {name: dash-to-dock, package: gnome-shell-extension-dash-to-dock, uuid: dash-to-dock@micxgx.gmail.com}
  flatpak: {package: flatpak, preinstall: true}
  trust: {anchorsDir: /etc/pki/ca-trust/source/anchors, updateCommand: update-ca-trust}
  iconCache: {package: gtk-update-icon-cache, command: "gtk-update-icon-cache --force --quiet"}
`

func workstation(name string, profiles ...string) string {
	return fmt.Sprintf(`apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: %s
spec:
  platformRef: test-platform
  profiles: [%s]
`, name, strings.Join(profiles, ", "))
}

// profile builds a Profile; refs are "Kind/name".
func profile(name, layer string, refs ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: core.deskos.org/v1alpha1\nkind: Profile\nmetadata:\n  name: %s\nspec:\n  layer: %s\n  resources:\n", name, layer)
	if len(refs) == 0 {
		b.WriteString("    []\n")
	}
	for _, r := range refs {
		kind, n, _ := strings.Cut(r, "/")
		fmt.Fprintf(&b, "    - {kind: %s, name: %s}\n", kind, n)
	}
	return b.String()
}

func packageSet(name string, packages ...string) string {
	return fmt.Sprintf("apiVersion: software.deskos.org/v1alpha1\nkind: PackageSet\nmetadata:\n  name: %s\nspec:\n  packages: [%s]\n", name, strings.Join(packages, ", "))
}

func gnome(name, defaults string) string {
	return fmt.Sprintf("apiVersion: desktop.deskos.org/v1alpha1\nkind: GnomeProfile\nmetadata:\n  name: %s\nspec:\n  defaults:\n%s", name, defaults)
}

func repo(name, id, url string) string {
	return fmt.Sprintf("apiVersion: software.deskos.org/v1alpha1\nkind: RpmRepository\nmetadata:\n  name: %s\nspec:\n  id: %s\n  displayName: %s\n  baseURL: %s\n  gpgKeyFiles: [assets/repo-key.asc]\n", name, id, id, url)
}

// testArmoredKey is a minimal ASCII-armored OpenPGP public key block. The
// loader checks the armor frame, not the packet contents.
func testArmoredKey() string {
	return "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n" + strings.Repeat("A", 64) + "\n-----END PGP PUBLIC KEY BLOCK-----\n"
}

func newCompiler(t *testing.T) *compiler.Compiler {
	t.Helper()
	c, err := compiler.New()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func planOf(t *testing.T, ws string, roots ...string) (*plan.Plan, error) {
	t.Helper()
	c := newCompiler(t)
	cat, err := c.Load(roots)
	if err != nil {
		return nil, err
	}
	return c.Plan(cat, ws)
}

func mustPlan(t *testing.T, ws string, roots ...string) *plan.Plan {
	t.Helper()
	p, err := planOf(t, ws, roots...)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustFail(t *testing.T, err error, want ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error containing %q", want)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error does not contain %q:\n%v", w, err)
		}
	}
}

func packages(p *plan.Plan) []string {
	var out []string
	for _, pkg := range p.Artifact.Packages {
		out = append(out, pkg.Name)
	}
	return out
}

func dconf(p *plan.Plan, key string) (string, bool) {
	if p.Artifact.Dconf == nil {
		return "", false
	}
	for _, d := range p.Artifact.Dconf.Defaults {
		if d.Key == key {
			return d.Value, true
		}
	}
	return "", false
}

var base = fixture{"platform.yaml": platformYAML}

func TestUnknownGVK(t *testing.T) {
	dir := base.with(fixture{"future.yaml": "apiVersion: example.org/v1\nkind: FutureThing\nmetadata:\n  name: x\nspec: {}\n"}).dir(t)
	_, err := newCompiler(t).Load([]string{dir})
	mustFail(t, err, "no provider registered for example.org/v1, Kind FutureThing")
}

func TestDuplicateIdentity(t *testing.T) {
	a := fixture{"a.yaml": packageSet("tools", "git")}.dir(t)
	b := fixture{"b.yaml": packageSet("tools", "git")}.dir(t)
	_, err := newCompiler(t).Load([]string{a, b})
	mustFail(t, err, "duplicate resource PackageSet/tools", "a.yaml", "b.yaml")
}

func TestMissingReferences(t *testing.T) {
	t.Run("platform", func(t *testing.T) {
		dir := fixture{
			"ws.yaml": workstation("ws", "p"),
			"p.yaml":  profile("p", "foundation"),
		}.dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, "Workstation/ws references Platform/test-platform, which is not defined")
	})
	t.Run("profile", func(t *testing.T) {
		dir := base.with(fixture{"ws.yaml": workstation("ws", "missing")}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, "Workstation/ws references Profile/missing, which is not defined")
	})
	t.Run("profile resource", func(t *testing.T) {
		dir := base.with(fixture{
			"ws.yaml": workstation("ws", "p"),
			"p.yaml":  profile("p", "foundation", "PackageSet/nope"),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, "Profile/p references PackageSet/nope, which is not defined", "p.yaml")
	})
	t.Run("transitive repository", func(t *testing.T) {
		dir := base.with(fixture{
			"ws.yaml": workstation("ws", "p"),
			"p.yaml":  profile("p", "foundation", "PackageSet/tools"),
			"s.yaml":  packageSet("tools", "terraform") + "  repositories: [hashicorp]\n",
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, "PackageSet/tools references RpmRepository/hashicorp, which is not defined")
	})
	t.Run("profile cannot include a workstation", func(t *testing.T) {
		dir := base.with(fixture{
			"ws.yaml": workstation("ws", "p"),
			"p.yaml":  profile("p", "foundation", "Workstation/ws"),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, "only content resources can be included")
	})
}

func TestPackageSetUnionAndCollapse(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core", "org"),
		"core.yaml": profile("core", "foundation", "PackageSet/a", "PackageSet/b"),
		"org.yaml":  profile("org", "organization", "PackageSet/c"),
		"a.yaml":    packageSet("a", "git", "vim-enhanced"),
		"b.yaml":    packageSet("b", "git", "podman"),
		"c.yaml":    packageSet("c", "podman", "zsh"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if got, want := strings.Join(packages(p), " "), "git podman vim-enhanced zsh"; got != want {
		t.Fatalf("packages = %q, want %q", got, want)
	}
	for _, pkg := range p.Artifact.Packages {
		if pkg.Name == "podman" && len(pkg.Provenance) != 2 {
			t.Errorf("podman should keep provenance from both sets, got %+v", pkg.Provenance)
		}
	}
}

const idle10 = "    session:\n      idle:\n        blankAfter: 10m\n"
const idle5 = "    session:\n      idle:\n        blankAfter: 5m\n"

func TestSameLayerScalarConflict(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "corp", "sec"),
		"corp.yaml": profile("corp", "organization", "GnomeProfile/corporate-desktop"),
		"sec.yaml":  profile("sec", "organization", "GnomeProfile/security-desktop"),
		"g1.yaml":   gnome("corporate-desktop", idle10),
		"g2.yaml":   gnome("security-desktop", idle5),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err,
		`composition conflict for GNOME setting "session.idle.blankAfter"`,
		"organization GnomeProfile/corporate-desktop", "value: 10m",
		"organization GnomeProfile/security-desktop", "value: 5m",
		"g1.yaml", "g2.yaml", "same semantic layer")
}

func TestSameLayerEqualValuesDeduplicate(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml": workstation("ws", "a", "b"),
		"a.yaml":  profile("a", "organization", "GnomeProfile/one"),
		"b.yaml":  profile("b", "organization", "GnomeProfile/two"),
		"g1.yaml": gnome("one", idle10),
		"g2.yaml": gnome("two", "    session:\n      idle:\n        blankAfter: 600s\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if v, _ := dconf(p, "/org/gnome/desktop/session/idle-delay"); v != "uint32 600" {
		t.Fatalf("idle-delay = %q", v)
	}
}

func TestHigherLayerOverride(t *testing.T) {
	buttons := func(b string) string { return "    windows:\n      buttons: [" + b + "]\n" }
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core", "bank"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/deskos-default"),
		"bank.yaml": profile("bank", "organization", "GnomeProfile/bank-default"),
		"g1.yaml":   gnome("deskos-default", buttons("minimize, maximize, close")),
		"g2.yaml":   gnome("bank-default", buttons("close")),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	// The higher layer's buttons win, and the plan carries only what the profile
	// declared: no app menu token is added, and the colon puts the buttons on the
	// right, where the profile did not ask to move them (docs/adr/0016).
	if v, _ := dconf(p, "/org/gnome/desktop/wm/preferences/button-layout"); v != "':close'" {
		t.Fatalf("button-layout = %s", v)
	}
}

func TestProfileOrderIndependence(t *testing.T) {
	files := fixture{
		"core.yaml": profile("core", "foundation", "PackageSet/a", "GnomeProfile/g1"),
		"org.yaml":  profile("org", "organization", "PackageSet/b", "GnomeProfile/g2"),
		"role.yaml": profile("role", "role", "PackageSet/c"),
		"a.yaml":    packageSet("a", "git"),
		"b.yaml":    packageSet("b", "zsh", "git"),
		"c.yaml":    packageSet("c", "podman"),
		"g1.yaml":   gnome("g1", idle10),
		"g2.yaml":   gnome("g2", idle5),
	}
	orders := [][]string{{"core", "org", "role"}, {"role", "org", "core"}, {"org", "core", "role"}}
	var first []byte
	for _, order := range orders {
		p := mustPlan(t, "ws", base.with(files).with(fixture{"ws.yaml": workstation("ws", order...)}).dir(t))
		b, err := p.JSON()
		if err != nil {
			t.Fatal(err)
		}
		// Source paths are identical in each fixture, so the plans must be too.
		if first == nil {
			first = b
		} else if string(b) != string(first) {
			t.Fatalf("plan depends on profile order %v", order)
		}
	}
	// The same resources split across differently named files give the same plan.
	renamed := fixture{"ws.yaml": workstation("ws", "core", "org", "role"), "platform.yaml": platformYAML}
	for name, content := range files {
		renamed["zz-"+name] = content
	}
	p := mustPlan(t, "ws", renamed.dir(t))
	if got := strings.Join(packages(p), " "); got != "git podman zsh" {
		t.Fatalf("packages = %q", got)
	}
}

func TestRpmRepositoryKeyed(t *testing.T) {
	ws := fixture{
		"ws.yaml": workstation("ws", "a", "b"),
		"a.yaml":  profile("a", "organization", "PackageSet/sa"),
		"b.yaml":  profile("b", "role", "PackageSet/sb"),
		"sa.yaml": packageSet("sa", "terraform") + "  repositories: [hashicorp]\n",
		"sb.yaml": packageSet("sb", "vault") + "  repositories: [hashicorp-mirror]\n",
	}
	t.Run("identical definitions deduplicate", func(t *testing.T) {
		dir := base.with(ws).with(fixture{
			"r1.yaml":             repo("hashicorp", "hashicorp", "https://rpm.releases.hashicorp.com/RHEL/$releasever/$basearch/stable"),
			"r2.yaml":             repo("hashicorp-mirror", "hashicorp", "https://rpm.releases.hashicorp.com/RHEL/$releasever/$basearch/stable"),
			"assets/repo-key.asc": testArmoredKey(),
		}).dir(t)
		p := mustPlan(t, "ws", dir)
		if n := len(p.Artifact.RpmRepositories); n != 1 {
			t.Fatalf("got %d repositories, want 1", n)
		}
		if n := len(p.Artifact.RpmRepositories[0].Provenance); n != 2 {
			t.Errorf("deduplicated repository should keep both sources, got %d", n)
		}
	})
	t.Run("different definitions conflict at any layer", func(t *testing.T) {
		dir := base.with(ws).with(fixture{
			"r1.yaml":             repo("hashicorp", "hashicorp", "https://rpm.releases.hashicorp.com/RHEL/$releasever/$basearch/stable"),
			"r2.yaml":             repo("hashicorp-mirror", "hashicorp", "https://mirror.example.org/hashicorp/stable"),
			"assets/repo-key.asc": testArmoredKey(),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, `composition conflict for RPM repository "hashicorp"`, "RpmRepository/hashicorp", "RpmRepository/hashicorp-mirror", "r1.yaml", "r2.yaml")
	})
}

func TestBinaryArtifactValidation(t *testing.T) {
	artifact := func(sha, dest string) string {
		return fmt.Sprintf(`apiVersion: software.deskos.org/v1alpha1
kind: BinaryArtifact
metadata:
  name: tool
spec:
  version: 1.2.3
  source:
    url: https://example.org/tool/1.2.3/tool.tar.gz
%s
  archive: tar.gz
  files:
    - {path: tool, destination: %s}
`, sha, dest)
	}
	const sha = "    sha256: " + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	load := func(doc string) error {
		_, err := newCompiler(t).Load([]string{fixture{"b.yaml": doc}.dir(t)})
		return err
	}
	if err := load(artifact(sha, "/usr/local/bin/tool")); err != nil {
		t.Fatalf("valid artifact rejected: %v", err)
	}
	mustFail(t, load(artifact("", "/usr/local/bin/tool")), "sha256")
	for _, dest := range []string{"/bin/tool", "/usr/local/bin/../../etc/passwd", "/usr/local/bin/sub/tool", "/etc/profile.d/tool"} {
		mustFail(t, load(artifact(sha, dest)), "destination")
	}
	floating := strings.Replace(artifact(sha, "/usr/local/bin/tool"), "1.2.3/tool", "latest/tool", 1)
	mustFail(t, load(floating), "floating location")
	unknown := strings.Replace(artifact(sha, "/usr/local/bin/tool"), "  archive: tar.gz", "  archive: tar.gz\n  postInstall: rm -rf /", 1)
	mustFail(t, load(unknown), "postInstall")
}

func TestBinaryArtifactTypedValidationWithoutSchema(t *testing.T) {
	// The typed decoder enforces the same rules as the schema, so a future
	// schema regression cannot silently weaken them.
	c := newCompiler(t)
	gvk := "software.deskos.org/v1alpha1"
	doc := (`{"version":"1.0","source":{"url":"https://example.org/t/1.0/t","sha256":""},"archive":"none","files":[{"destination":"/bin/t"}]}`)
	res := rawResource(gvk, "BinaryArtifact", "t", doc)
	prov, err := c.Registry.Lookup(res.GVK())
	if err != nil {
		t.Fatal(err)
	}
	err = prov.Decode(res)
	mustFail(t, err, "source.sha256 is required", "not allowed")
}

func TestFlatpakSetUnion(t *testing.T) {
	remote := `apiVersion: software.deskos.org/v1alpha1
kind: FlatpakRemote
metadata:
  name: flathub
spec:
  title: Flathub
  url: https://dl.flathub.org/repo/
  collectionID: org.flathub.Stable
  gpgKeyFile: key.gpg
`
	set := func(name string, apps ...string) string {
		var b strings.Builder
		fmt.Fprintf(&b, "apiVersion: software.deskos.org/v1alpha1\nkind: FlatpakSet\nmetadata:\n  name: %s\nspec:\n  remote: flathub\n  applications:\n", name)
		for _, a := range apps {
			fmt.Fprintf(&b, "    - id: %s\n", a)
		}
		return b.String()
	}
	files := base.with(fixture{
		"remote.yaml": remote,
		"key.gpg":     "\x99\x01binary-key-placeholder",
		"ws.yaml":     workstation("ws", "a", "b"),
		"a.yaml":      profile("a", "foundation", "FlatpakSet/one"),
		"b.yaml":      profile("b", "role", "FlatpakSet/two"),
		"one.yaml":    set("one", "org.example.Zeta", "org.example.Alpha"),
		"two.yaml":    set("two", "org.example.Alpha", "org.example.Mid"),
	})
	p := mustPlan(t, "ws", files.dir(t))
	var ids []string
	for _, a := range p.Provisioning.FlatpakApplications {
		ids = append(ids, a.ID)
		if a.CollectionID != "org.flathub.Stable" || a.Branch != "stable" {
			t.Errorf("app %s: %+v", a.ID, a)
		}
	}
	if got := strings.Join(ids, " "); got != "org.example.Alpha org.example.Mid org.example.Zeta" {
		t.Fatalf("applications = %q", got)
	}
	if p.Provisioning.FlatpakPreinstallUnit == "" {
		t.Error("preinstall unit missing")
	}

	conflict := files.with(fixture{"two.yaml": strings.Replace(set("two", "org.example.Alpha"), "id: org.example.Alpha", "id: org.example.Alpha\n      branch: beta", 1)})
	_, err := planOf(t, "ws", conflict.dir(t))
	mustFail(t, err, `composition conflict for Flatpak application "org.example.Alpha"`)
}

// An OCI remote is an index of Flatpak images in a container registry: flatpak
// reads it over the oci+https transport, the registry signs the images, and the
// authenticator that supplies credentials cannot live in the remote file, so a
// unit applies it once the remote exists.
func TestFlatpakOCIRemote(t *testing.T) {
	remote := `apiVersion: software.deskos.org/v1alpha1
kind: FlatpakRemote
metadata:
  name: katello
spec:
  title: Katello
  transport: oci
  url: https://satellite.example.com
  authenticator: org.flatpak.Authenticator.Oci
`
	set := `apiVersion: software.deskos.org/v1alpha1
kind: FlatpakSet
metadata:
  name: katello-apps
spec:
  remote: katello
  applications:
    - id: org.mozilla.firefox
`
	files := base.with(fixture{
		"remote.yaml": remote,
		"set.yaml":    set,
		"ws.yaml":     workstation("ws", "a"),
		"a.yaml":      profile("a", "foundation", "FlatpakSet/katello-apps"),
	})
	p := mustPlan(t, "ws", files.dir(t))

	if len(p.Provisioning.FlatpakRemotes) != 1 {
		t.Fatalf("remotes = %+v", p.Provisioning.FlatpakRemotes)
	}
	r := p.Provisioning.FlatpakRemotes[0]
	if r.Transport != plan.FlatpakTransportOCI || r.Authenticator != "org.flatpak.Authenticator.Oci" {
		t.Errorf("remote = %+v", r)
	}
	if r.GPGKey != "" || r.GPGKeySHA256 != "" {
		t.Errorf("an oci remote carries no signing key: %+v", r)
	}
	if got := r.RemoteURL(); got != "oci+https://satellite.example.com" {
		t.Errorf("remote URL = %q", got)
	}
	if p.Provisioning.FlatpakAuthenticatorUnit == "" {
		t.Error("authenticator unit missing")
	}
	if got := units(p); !strings.Contains(got, "deskos-flatpak-oci-authenticator.service") {
		t.Errorf("authenticator unit not enabled: %q", got)
	}

	rendered, err := containerfile.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	repo := string(fileData(t, rendered, "rootfs/usr/share/flatpak/remotes.d/katello.flatpakrepo"))
	for _, want := range []string{"Title=Katello\n", "Url=oci+https://satellite.example.com\n"} {
		if !strings.Contains(repo, want) {
			t.Errorf("remote file lacks %q:\n%s", want, repo)
		}
	}
	// The OCI transport has no OSTree summary key to verify against and no
	// collection to deploy sideloads into.
	for _, bad := range []string{"GPGKey=", "DeploySideloadCollectionID="} {
		if strings.Contains(repo, bad) {
			t.Errorf("remote file contains %q:\n%s", bad, repo)
		}
	}

	unit := string(fileData(t, rendered, "rootfs/usr/lib/systemd/system/deskos-flatpak-oci-authenticator.service"))
	for _, want := range []string{
		"ExecStart=/usr/bin/flatpak remote-add --system --if-not-exists --from katello /usr/share/flatpak/remotes.d/katello.flatpakrepo\n",
		"ExecStart=/usr/bin/flatpak remote-modify --system --authenticator-name=org.flatpak.Authenticator.Oci katello\n",
		"Before=deskos-flatpak-preinstall.service\n",
		"Type=oneshot\n",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("authenticator unit lacks %q:\n%s", want, unit)
		}
	}
	// It only runs local commands, so it adds no network wait to a boot target.
	for _, bad := range []string{"network-online", "Restart="} {
		if strings.Contains(unit, bad) {
			t.Errorf("authenticator unit contains %q:\n%s", bad, unit)
		}
	}

	// An OCI remote with no authenticator is a public registry index: it
	// compiles, and nothing has to be applied on the machine.
	public := files.with(fixture{"remote.yaml": strings.Replace(remote, "  authenticator: org.flatpak.Authenticator.Oci\n", "", 1)})
	pub := mustPlan(t, "ws", public.dir(t))
	if pub.Provisioning.FlatpakAuthenticatorUnit != "" {
		t.Error("a public oci remote needs no authenticator unit")
	}
	if got := units(pub); strings.Contains(got, "deskos-flatpak-oci-authenticator.service") {
		t.Errorf("authenticator unit enabled for a public remote: %q", got)
	}
}

func TestFlatpakRemoteTransportValidation(t *testing.T) {
	// The typed decoder enforces the same rules as the schema, so a future
	// schema regression cannot silently weaken them.
	c := newCompiler(t)
	gvk := "software.deskos.org/v1alpha1"
	for name, tc := range map[string]struct{ doc, want string }{
		"oci with a signing key":       {`{"title":"T","transport":"oci","url":"https://example.org/","gpgKeyFile":"key.gpg"}`, "gpgKeyFile is not allowed"},
		"oci with a collection":        {`{"title":"T","transport":"oci","url":"https://example.org/","collectionID":"org.example.Stable"}`, "collectionID is not allowed"},
		"ostree with an authenticator": {`{"title":"T","url":"https://example.org/","gpgKeyFile":"key.gpg","authenticator":"org.flatpak.Authenticator.Oci"}`, "authenticator is only allowed"},
		"ostree without a key":         {`{"title":"T","url":"https://example.org/"}`, "gpgKeyFile is required"},
		"unknown transport":            {`{"title":"T","transport":"docker","url":"https://example.org/","gpgKeyFile":"key.gpg"}`, "transport must be ostree or oci"},
		"authenticator not an app id":  {`{"title":"T","transport":"oci","url":"https://example.org/","authenticator":"not an id"}`, "invalid authenticator"},
	} {
		t.Run(name, func(t *testing.T) {
			res := rawResource(gvk, "FlatpakRemote", "r", tc.doc)
			prov, err := c.Registry.Lookup(res.GVK())
			if err != nil {
				t.Fatal(err)
			}
			mustFail(t, prov.Decode(res), tc.want)
		})
	}

	// The schema carries the same rules, so the document is rejected before the
	// typed decoder ever sees it.
	schema := base.with(fixture{
		"remote.yaml": `apiVersion: software.deskos.org/v1alpha1
kind: FlatpakRemote
metadata:
  name: katello
spec:
  title: Katello
  transport: oci
  url: https://satellite.example.com
  gpgKeyFile: key.gpg
`,
		"key.gpg": "\x99\x01binary-key-placeholder",
		"ws.yaml": workstation("ws", "a"),
		"a.yaml":  profile("a", "foundation", "FlatpakRemote/katello"),
	})
	_, err := planOf(t, "ws", schema.dir(t))
	mustFail(t, err, "schema validation failed")
}

func TestGnomeLocksAndDock(t *testing.T) {
	defaults := `    dock:
      enabled: true
      position: left
      behavior: fixed
    session:
      idle:
        blankAfter: 5m
  locks:
    - session.idle.blankAfter
`
	dir := base.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml":  gnome("g", defaults),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if v, _ := dconf(p, "/org/gnome/shell/enabled-extensions"); v != "['background-logo@fedorahosted.org', 'dash-to-dock@micxgx.gmail.com']" {
		t.Errorf("enabled-extensions = %s", v)
	}
	if v, _ := dconf(p, "/org/gnome/shell/extensions/dash-to-dock/dock-fixed"); v != "true" {
		t.Errorf("dock-fixed = %s", v)
	}
	if !strings.Contains(strings.Join(packages(p), " "), "gnome-shell-extension-dash-to-dock") {
		t.Error("dock extension package not installed")
	}
	if len(p.Artifact.Dconf.Locks) != 1 || p.Artifact.Dconf.Locks[0].Key != "/org/gnome/desktop/session/idle-delay" {
		t.Errorf("locks = %+v", p.Artifact.Dconf.Locks)
	}

	unset := base.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml":  gnome("g", idle5+"  locks:\n    - shell.favorites\n"),
	}).dir(t)
	_, err := planOf(t, "ws", unset)
	mustFail(t, err, `GNOME lock "shell.favorites" has no effective value`)
}

func TestAssetsStayInsideRoot(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.png")
	if err := os.WriteFile(secret, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	rel, err := filepath.Rel(dir, secret)
	if err != nil {
		t.Fatal(err)
	}
	doc := gnome("g", "    appearance:\n      wallpaper:\n        light: "+filepath.ToSlash(rel)+"\n")
	if err := os.WriteFile(filepath.Join(dir, "g.yaml"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = newCompiler(t).Load([]string{dir})
	mustFail(t, err, "outside the resource root")
}

func TestLoginLogo(t *testing.T) {
	gdmPlatform := strings.Replace(platformYAML, "    dconfDatabase: distro\n", "    dconfDatabase: distro\n    loginScreen: true\n", 1)
	logo := func(name, file string) string {
		return gnome(name, "    appearance:\n      loginLogo: "+file+"\n")
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`
	files := fixture{
		"platform.yaml": gdmPlatform,
		"core.svg":      svg,
		"org.svg":       strings.Replace(svg, "10", "20", 1),
		"core.yaml":     profile("core", "foundation", "GnomeProfile/core-logo"),
		"org.yaml":      profile("org", "organization", "GnomeProfile/org-logo"),
		"g-core.yaml":   logo("core-logo", "core.svg"),
		"g-org.yaml":    logo("org-logo", "org.svg"),
	}

	t.Run("higher layer overrides", func(t *testing.T) {
		p := mustPlan(t, "ws", files.with(fixture{"ws.yaml": workstation("ws", "core", "org")}).dir(t))
		if v, _ := dconf(p, "/org/gnome/login-screen/logo"); v != "'/usr/share/deskos/branding/org.svg'" {
			t.Errorf("login logo = %s", v)
		}
		var installed []string
		for _, f := range p.Artifact.Files {
			installed = append(installed, f.Path)
		}
		if got := strings.Join(installed, " "); got != "/usr/share/deskos/branding/org.svg" {
			t.Errorf("installed %q; only the effective logo belongs in the image", got)
		}
	})
	t.Run("same layer conflict", func(t *testing.T) {
		dir := files.with(fixture{
			"ws.yaml":   workstation("ws", "org", "org2"),
			"org2.yaml": profile("org2", "organization", "GnomeProfile/core-logo"),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, `composition conflict for GNOME setting "appearance.loginLogo"`, "GnomeProfile/core-logo", "GnomeProfile/org-logo")
	})
	t.Run("platform without GDM login screen", func(t *testing.T) {
		dir := files.with(fixture{"platform.yaml": platformYAML, "ws.yaml": workstation("ws", "org")}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, "Platform/test-platform does not provide a GDM login screen", "appearance.loginLogo", "GnomeProfile/org-logo")
	})
	t.Run("only png or svg", func(t *testing.T) {
		dir := files.with(fixture{"logo.jpg": "x", "g-org.yaml": logo("org-logo", "logo.jpg"), "ws.yaml": workstation("ws", "org")}).dir(t)
		_, err := newCompiler(t).Load([]string{dir})
		mustFail(t, err, "must be a .png or .svg file")
	})
}

func TestDockSemantics(t *testing.T) {
	dock := func(name, body string) string { return gnome(name, "    dock:\n"+body) }
	hasKey := func(p *plan.Plan, prefix string) bool {
		if p.Artifact.Dconf == nil {
			return false
		}
		for _, d := range p.Artifact.Dconf.Defaults {
			if strings.HasPrefix(d.Key, prefix) {
				return true
			}
		}
		return false
	}
	const ddKeys = "/org/gnome/shell/extensions/dash-to-dock/"
	build := func(extra fixture, profiles ...string) (*plan.Plan, error) {
		return planOf(t, "ws", base.with(extra).with(fixture{"ws.yaml": workstation("ws", profiles...)}).dir(t))
	}

	t.Run("enabled with showTrash false", func(t *testing.T) {
		p, err := build(fixture{
			"o.yaml": profile("org", "organization", "GnomeProfile/d"),
			"d.yaml": dock("d", "      enabled: true\n      showTrash: false\n"),
		}, "org")
		if err != nil {
			t.Fatal(err)
		}
		if v, _ := dconf(p, ddKeys+"show-trash"); v != "false" {
			t.Errorf("show-trash = %q", v)
		}
		if v, _ := dconf(p, "/org/gnome/shell/enabled-extensions"); !strings.Contains(v, "dash-to-dock@micxgx.gmail.com") {
			t.Errorf("extension not enabled: %s", v)
		}
	})
	t.Run("options without enabled fail", func(t *testing.T) {
		_, err := build(fixture{
			"o.yaml": profile("org", "organization", "GnomeProfile/d"),
			"d.yaml": dock("d", "      showTrash: false\n"),
		}, "org")
		mustFail(t, err, "dock.showTrash requires dock.enabled: true", "GnomeProfile/d")
	})
	t.Run("higher-layer disable masks lower-layer options", func(t *testing.T) {
		p, err := build(fixture{
			"c.yaml":  profile("core", "foundation", "GnomeProfile/lower"),
			"o.yaml":  profile("org", "organization", "GnomeProfile/off"),
			"l.yaml":  dock("lower", "      enabled: true\n      position: left\n      showTrash: true\n"),
			"of.yaml": dock("off", "      enabled: false\n"),
		}, "core", "org")
		if err != nil {
			t.Fatal(err)
		}
		if hasKey(p, ddKeys) {
			t.Error("masked dock options still produce extension keys")
		}
		if _, ok := dconf(p, "/org/gnome/shell/enabled-extensions"); ok {
			t.Error("a disabled dock must not write enabled-extensions")
		}
		if strings.Contains(strings.Join(packages(p), " "), "dash-to-dock") {
			t.Error("a disabled dock must not install the extension")
		}
		warnings := strings.Join(p.Warnings, "\n")
		for _, want := range []string{"dock.position from foundation layer is masked", "dock.showTrash from foundation layer is masked", "GnomeProfile/off"} {
			if !strings.Contains(warnings, want) {
				t.Errorf("warnings lack %q:\n%s", want, warnings)
			}
		}
	})
	t.Run("options at or above a disable fail", func(t *testing.T) {
		for name, files := range map[string]fixture{
			"same layer": {
				"o.yaml": profile("org", "organization", "GnomeProfile/d"),
				"d.yaml": dock("d", "      enabled: false\n      showTrash: false\n"),
			},
			"higher layer": {
				"o.yaml": profile("org", "organization", "GnomeProfile/off"),
				"r.yaml": profile("role", "role", "GnomeProfile/pos"),
				"f.yaml": dock("off", "      enabled: false\n"),
				"p.yaml": dock("pos", "      position: left\n"),
			},
		} {
			t.Run(name, func(t *testing.T) {
				profiles := []string{"org"}
				if name == "higher layer" {
					profiles = append(profiles, "role")
				}
				_, err := build(files, profiles...)
				mustFail(t, err, "but dock.enabled is false", "disabled by:")
			})
		}
	})
	t.Run("platform without the extension", func(t *testing.T) {
		noExt := strings.Replace(platformYAML, "    extensions:\n      - {name: dash-to-dock, package: gnome-shell-extension-dash-to-dock, uuid: dash-to-dock@micxgx.gmail.com}\n", "", 1)
		dir := fixture{
			"platform.yaml": noExt,
			"ws.yaml":       workstation("ws", "org"),
			"o.yaml":        profile("org", "organization", "GnomeProfile/d"),
			"d.yaml":        dock("d", "      enabled: true\n"),
		}.dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, `does not provide the GNOME extension "dash-to-dock"`)
	})
}

// Core sets no dock and no favorites: the shell layout is the platform's look and
// feel, or an organization's choice. An organization that wants Dash to Dock
// enables it from its own layer, and then its options apply.
func TestCoreSetsNoDockOrFavorites(t *testing.T) {
	dockKeys := func(p *plan.Plan) []string {
		var out []string
		for _, d := range p.Artifact.Dconf.Defaults {
			if strings.Contains(d.Key, "dash-to-dock") {
				out = append(out, d.Key)
			}
		}
		return out
	}
	const orgWorkstation = "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: org-ws\nspec:\n  platformRef: centos-stream-10\n  profiles: [deskos-core, deskos-reference-apps, org]\n"
	hasDockPackage := func(p *plan.Plan) bool {
		return strings.Contains(strings.Join(packages(p), " "), "gnome-shell-extension-dash-to-dock")
	}
	t.Run("core", func(t *testing.T) {
		p := mustPlan(t, "deskos-core-centos10", resourcesRoot)
		if v, ok := dconf(p, "/org/gnome/shell/favorite-apps"); ok {
			t.Errorf("Core writes favorite-apps = %s", v)
		}
		if v, ok := dconf(p, "/org/gnome/shell/enabled-extensions"); ok {
			t.Errorf("Core writes enabled-extensions = %s", v)
		}
		if hasDockPackage(p) {
			t.Error("Core installs Dash to Dock")
		}
		if k := dockKeys(p); len(k) != 0 {
			t.Errorf("Core sets dock options %q", k)
		}
	})
	t.Run("organization enables the dock itself", func(t *testing.T) {
		org := fixture{
			"org.yaml": profile("org", "organization", "GnomeProfile/org-desktop"),
			"g.yaml":   gnome("org-desktop", "    dock:\n      enabled: true\n      position: bottom\n"),
			"ws.yaml":  orgWorkstation,
		}.dir(t)
		p := mustPlan(t, "org-ws", resourcesRoot, org)
		if !hasDockPackage(p) {
			t.Error("the organization's dock is not installed")
		}
		if v, _ := dconf(p, "/org/gnome/shell/extensions/dash-to-dock/dock-position"); v != "'BOTTOM'" {
			t.Errorf("dock-position = %s", v)
		}
		if v, ok := dconf(p, "/org/gnome/shell/favorite-apps"); ok {
			t.Errorf("favorites are still written although Core sets none: %s", v)
		}
	})
	t.Run("dock options without enabling the dock are refused", func(t *testing.T) {
		org := fixture{
			"org.yaml": profile("org", "organization", "GnomeProfile/org-desktop"),
			"g.yaml":   gnome("org-desktop", "    dock:\n      position: bottom\n"),
			"ws.yaml":  orgWorkstation,
		}.dir(t)
		_, err := planOf(t, "org-ws", resourcesRoot, org)
		mustFail(t, err, "dock.position requires dock.enabled: true")
	})
}

// Core decides nothing about the look: no wallpaper, no login logo, no boot
// splash. They are the platform's look and feel, or an organization's choice, not
// the foundation's. A workstation that wants the DeskOS look adds the deskos-look
// profile, one that wants only a quiet graphical boot adds deskos-boot, and an
// organization that overrides a scalar still wins.
func TestCoreSetsNoLook(t *testing.T) {
	const ws = "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: %s\nspec:\n  platformRef: centos-stream-10\n  profiles: [%s]\n"
	const logoKey = "/org/gnome/login-screen/logo"
	wallpaper := func(p *plan.Plan) bool {
		for _, d := range p.Artifact.GSettings {
			if d.Setting == "appearance.wallpaper" {
				return true
			}
		}
		return false
	}
	themeFiles := func(p *plan.Plan) []string {
		var out []string
		for _, f := range p.Artifact.Files {
			if strings.Contains(f.Path, "/plymouth/themes/") {
				out = append(out, f.Path)
			}
		}
		return out
	}
	t.Run("core", func(t *testing.T) {
		f := fixture{"ws.yaml": fmt.Sprintf(ws, "no-look", "deskos-core, rhel-family")}
		p := mustPlan(t, "no-look", resourcesRoot, f.dir(t))
		if len(p.Artifact.KernelArguments) != 0 {
			t.Errorf("Core sets kernel arguments %+v", p.Artifact.KernelArguments)
		}
		if p.Artifact.Initramfs != nil {
			t.Errorf("Core regenerates the initramfs: %+v", p.Artifact.Initramfs)
		}
		if p.Artifact.PlymouthTheme != nil {
			t.Errorf("Core installs the DeskOS splash theme: %+v", p.Artifact.PlymouthTheme)
		}
		if files := themeFiles(p); len(files) != 0 {
			t.Errorf("Core writes Plymouth theme files %q", files)
		}
		if wallpaper(p) {
			t.Error("Core sets a wallpaper")
		}
		if v, ok := dconf(p, logoKey); ok {
			t.Errorf("Core sets the login logo to %s", v)
		}
	})
	t.Run("deskos-look opts in", func(t *testing.T) {
		f := fixture{"ws.yaml": fmt.Sprintf(ws, "with-look", "deskos-core, rhel-family, deskos-look")}
		p := mustPlan(t, "with-look", resourcesRoot, f.dir(t))
		if p.Artifact.PlymouthTheme == nil {
			t.Fatal("deskos-look does not install the DeskOS splash theme")
		}
		if len(p.Artifact.KernelArguments) == 0 || p.Artifact.Initramfs == nil {
			t.Errorf("deskos-look sets no boot: kargs %+v, initramfs %+v", p.Artifact.KernelArguments, p.Artifact.Initramfs)
		}
		if files := themeFiles(p); len(files) == 0 {
			t.Error("deskos-look writes no theme file")
		}
		if !wallpaper(p) {
			t.Error("deskos-look sets no wallpaper")
		}
		if _, ok := dconf(p, logoKey); !ok {
			t.Error("deskos-look sets no login logo")
		}
	})
	t.Run("deskos-boot opts in without the look", func(t *testing.T) {
		f := fixture{"ws.yaml": fmt.Sprintf(ws, "boot-only", "deskos-core, rhel-family, deskos-boot")}
		p := mustPlan(t, "boot-only", resourcesRoot, f.dir(t))
		if len(p.Artifact.KernelArguments) == 0 || p.Artifact.Initramfs == nil {
			t.Errorf("deskos-boot sets no boot: kargs %+v, initramfs %+v", p.Artifact.KernelArguments, p.Artifact.Initramfs)
		}
		if p.Artifact.PlymouthTheme != nil {
			t.Errorf("deskos-boot installs the DeskOS splash theme: %+v", p.Artifact.PlymouthTheme)
		}
		if wallpaper(p) {
			t.Error("deskos-boot sets a wallpaper")
		}
		if v, ok := dconf(p, logoKey); ok {
			t.Errorf("deskos-boot sets the login logo to %s", v)
		}
	})
}

func TestDockDisableOverridesPlatformDefault(t *testing.T) {
	const uuid = "dash-to-dock@micxgx.gmail.com"
	off := fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/off"),
		"d.yaml":  gnome("off", "    dock:\n      enabled: false\n"),
	}
	t.Run("platform enables the dock by default", func(t *testing.T) {
		pre := strings.Replace(platformYAML, "enabledExtensions: [background-logo@fedorahosted.org]", "enabledExtensions: [background-logo@fedorahosted.org, "+uuid+"]", 1)
		p := mustPlan(t, "ws", off.with(fixture{"platform.yaml": pre}).dir(t))
		if v, _ := dconf(p, "/org/gnome/shell/enabled-extensions"); v != "['background-logo@fedorahosted.org']" {
			t.Errorf("enabled-extensions = %q, want the platform list without the dock", v)
		}
		for _, d := range p.Artifact.Dconf.Defaults {
			if strings.Contains(d.Key, "dash-to-dock") {
				t.Errorf("disabled dock writes %s", d.Key)
			}
		}
		if strings.Contains(strings.Join(packages(p), " "), "dash-to-dock") {
			t.Error("disabling the dock installed it")
		}
	})
	t.Run("platform default without the dock writes nothing", func(t *testing.T) {
		p := mustPlan(t, "ws", base.with(off).dir(t))
		if _, ok := dconf(p, "/org/gnome/shell/enabled-extensions"); ok {
			t.Error("enabled-extensions written although the platform default lacks the dock")
		}
	})
	t.Run("locked disable pins the list", func(t *testing.T) {
		locked := off.with(fixture{"d.yaml": gnome("off", "    dock:\n      enabled: false\n  locks:\n    - dock.enabled\n")})
		p := mustPlan(t, "ws", base.with(locked).dir(t))
		if v, _ := dconf(p, "/org/gnome/shell/enabled-extensions"); v != "['background-logo@fedorahosted.org']" {
			t.Errorf("enabled-extensions = %q", v)
		}
		if len(p.Artifact.Dconf.Locks) != 1 || p.Artifact.Dconf.Locks[0].Key != "/org/gnome/shell/enabled-extensions" {
			t.Errorf("locks = %+v", p.Artifact.Dconf.Locks)
		}
	})
}

func TestBootProfile(t *testing.T) {
	bootPlatform := strings.Replace(platformYAML, "  flatpak: {package: flatpak, preinstall: true}\n",
		"  boot: {splashKernelArgument: rhgb, quietKernelArgument: quiet, plymouthPackages: [plymouth, plymouth-scripts], plymouthDracutModule: plymouth}\n  flatpak: {package: flatpak, preinstall: true}\n", 1)
	boot := func(name, body string) string {
		return "apiVersion: system.deskos.org/v1alpha1\nkind: BootProfile\nmetadata:\n  name: " + name + "\nspec:\n" + body
	}
	args := func(p *plan.Plan) string {
		var out []string
		for _, k := range p.Artifact.KernelArguments {
			out = append(out, k.Arg)
		}
		return strings.Join(out, " ")
	}
	files := fixture{
		"platform.yaml": bootPlatform,
		"core.yaml":     profile("core", "foundation", "BootProfile/core"),
		"b-core.yaml":   boot("core", "  splash: graphical\n  quiet: true\n"),
	}
	t.Run("graphical and quiet", func(t *testing.T) {
		p := mustPlan(t, "ws", files.with(fixture{"ws.yaml": workstation("ws", "core")}).dir(t))
		if got := args(p); got != "quiet rhgb" {
			t.Errorf("kernel arguments = %q", got)
		}
		if p.Artifact.Initramfs == nil || strings.Join(p.Artifact.Initramfs.DracutModules, " ") != "plymouth" {
			t.Errorf("initramfs = %+v", p.Artifact.Initramfs)
		}
		if !strings.Contains(strings.Join(packages(p), " "), "plymouth-scripts") {
			t.Error("Plymouth packages missing")
		}
	})
	t.Run("higher layer chooses text and verbose", func(t *testing.T) {
		p := mustPlan(t, "ws", files.with(fixture{
			"ws.yaml":    workstation("ws", "core", "org"),
			"org.yaml":   profile("org", "organization", "BootProfile/org"),
			"b-org.yaml": boot("org", "  splash: text\n  quiet: false\n"),
		}).dir(t))
		if len(p.Artifact.KernelArguments) != 0 || p.Artifact.Initramfs != nil {
			t.Errorf("text/verbose boot still emits %q, initramfs %+v", args(p), p.Artifact.Initramfs)
		}
	})
	t.Run("same layer conflict", func(t *testing.T) {
		_, err := planOf(t, "ws", files.with(fixture{
			"ws.yaml":      workstation("ws", "core", "core2"),
			"core2.yaml":   profile("core2", "foundation", "BootProfile/core2"),
			"b-core2.yaml": boot("core2", "  splash: text\n"),
		}).dir(t))
		mustFail(t, err, `composition conflict for boot setting "boot.splash"`, "BootProfile/core", "BootProfile/core2")
	})
	t.Run("platform without boot facts", func(t *testing.T) {
		_, err := planOf(t, "ws", files.with(fixture{"platform.yaml": platformYAML, "ws.yaml": workstation("ws", "core")}).dir(t))
		mustFail(t, err, "Platform/test-platform declares no boot facts", "BootProfile/core")
	})
	t.Run("empty spec rejected", func(t *testing.T) {
		_, err := newCompiler(t).Load([]string{fixture{"b.yaml": boot("empty", "  {}\n")}.dir(t)})
		mustFail(t, err, "BootProfile/empty")
	})
}

func pngOf(t *testing.T, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestBootWatermark(t *testing.T) {
	const facts = "  boot:\n    splashKernelArgument: rhgb\n    quietKernelArgument: quiet\n    plymouthPackages: [plymouth, plymouth-scripts]\n    plymouthDracutModule: plymouth\n"
	const frames = "    spinnerFrames: {package: plymouth-theme-spinner, imageDir: /usr/share/plymouth/themes/spinner}\n"
	withBoot := func(boot string) string {
		return strings.Replace(platformYAML, "  flatpak: {package: flatpak, preinstall: true}\n", boot+"  flatpak: {package: flatpak, preinstall: true}\n", 1)
	}
	boot := func(name, body string) string {
		return "apiVersion: system.deskos.org/v1alpha1\nkind: BootProfile\nmetadata:\n  name: " + name + "\nspec:\n" + body
	}
	const dest = "/usr/share/plymouth/themes/deskos/watermark.png"
	corePNG, orgPNG := pngOf(t, 4, 2), pngOf(t, 6, 2)
	files := fixture{
		"platform.yaml": withBoot(facts + frames),
		"core.png":      corePNG,
		"org.png":       orgPNG,
		"core.yaml":     profile("core", "foundation", "BootProfile/core"),
		"b-core.yaml":   boot("core", "  splash: graphical\n  watermark: core.png\n"),
		"org.yaml":      profile("org", "organization", "BootProfile/org"),
	}
	installed := func(p *plan.Plan) string {
		var out []string
		for _, f := range p.Artifact.Files {
			out = append(out, f.Path+"="+f.Asset)
		}
		return strings.Join(out, " ")
	}
	masked := func(p *plan.Plan) []string {
		var out []string
		for _, w := range p.Warnings {
			if strings.Contains(w, "masked") {
				out = append(out, w)
			}
		}
		return out
	}

	t.Run("graphical with watermark", func(t *testing.T) {
		p := mustPlan(t, "ws", files.with(fixture{"ws.yaml": workstation("ws", "core")}).dir(t))
		th := p.Artifact.PlymouthTheme
		if th == nil || th.Name != "deskos" || th.Dir != "/usr/share/plymouth/themes/deskos" || th.FramesFrom != "/usr/share/plymouth/themes/spinner" || th.Watermark != dest {
			t.Fatalf("theme = %+v", th)
		}
		if got := installed(p); got != dest+"=root/core.png" {
			t.Errorf("files = %q", got)
		}
		if !strings.Contains(strings.Join(packages(p), " "), "plymouth-theme-spinner") {
			t.Error("the frames package is not installed")
		}
		if p.Artifact.Initramfs == nil || len(p.Artifact.Initramfs.Provenance) == 0 {
			t.Errorf("initramfs = %+v", p.Artifact.Initramfs)
		}
		if m := masked(p); len(m) != 0 {
			t.Errorf("unexpected masking warnings %q", m)
		}
	})
	t.Run("higher layer replaces the watermark", func(t *testing.T) {
		p := mustPlan(t, "ws", files.with(fixture{
			"ws.yaml":    workstation("ws", "core", "org"),
			"b-org.yaml": boot("org", "  watermark: org.png\n"),
		}).dir(t))
		if got := installed(p); got != dest+"=root/org.png" {
			t.Errorf("files = %q; only the effective watermark belongs in the image", got)
		}
		if p.Artifact.PlymouthTheme == nil || p.Artifact.PlymouthTheme.Provenance[0].Resource != "BootProfile/org" {
			t.Errorf("theme = %+v", p.Artifact.PlymouthTheme)
		}
	})
	t.Run("same layer conflict", func(t *testing.T) {
		_, err := planOf(t, "ws", files.with(fixture{
			"ws.yaml":      workstation("ws", "core", "core2"),
			"core2.yaml":   profile("core2", "foundation", "BootProfile/core2"),
			"b-core2.yaml": boot("core2", "  watermark: org.png\n"),
		}).dir(t))
		mustFail(t, err, `composition conflict for boot setting "boot.watermark"`, "BootProfile/core", "BootProfile/core2")
	})
	t.Run("higher layer text masks a lower watermark", func(t *testing.T) {
		p := mustPlan(t, "ws", files.with(fixture{
			"ws.yaml":    workstation("ws", "core", "org"),
			"b-org.yaml": boot("org", "  splash: text\n"),
		}).dir(t))
		if p.Artifact.PlymouthTheme != nil || p.Artifact.Initramfs != nil || installed(p) != "" {
			t.Errorf("text splash still carries a theme %+v, initramfs %+v, files %q", p.Artifact.PlymouthTheme, p.Artifact.Initramfs, installed(p))
		}
		want := "boot.watermark from foundation layer is masked: boot.splash is text at organization layer (BootProfile/org)"
		if m := masked(p); len(m) != 1 || m[0] != want {
			t.Errorf("warnings = %q, want %q", m, want)
		}
	})
	t.Run("text and watermark at the same layer", func(t *testing.T) {
		_, err := planOf(t, "ws", files.with(fixture{
			"ws.yaml":    workstation("ws", "core", "org"),
			"b-org.yaml": boot("org", "  splash: text\n  watermark: org.png\n"),
		}).dir(t))
		mustFail(t, err, "boot.watermark is set at organization layer but boot.splash is text at organization layer", "BootProfile/org")
	})
	t.Run("watermark above a text splash", func(t *testing.T) {
		_, err := planOf(t, "ws", files.with(fixture{
			"ws.yaml":     workstation("ws", "core", "org"),
			"b-core.yaml": boot("core", "  splash: text\n"),
			"b-org.yaml":  boot("org", "  watermark: org.png\n"),
		}).dir(t))
		mustFail(t, err, "boot.watermark is set at organization layer but boot.splash is text at foundation layer", "text by:", "BootProfile/core")
	})
	t.Run("watermark without splash", func(t *testing.T) {
		_, err := planOf(t, "ws", files.with(fixture{
			"ws.yaml":     workstation("ws", "core"),
			"b-core.yaml": boot("core", "  quiet: true\n  watermark: core.png\n"),
		}).dir(t))
		mustFail(t, err, "boot.watermark requires splash: graphical, which no BootProfile sets", "BootProfile/core")
	})
	t.Run("platform without spinner frames", func(t *testing.T) {
		_, err := planOf(t, "ws", files.with(fixture{
			"platform.yaml": withBoot(facts),
			"ws.yaml":       workstation("ws", "core"),
		}).dir(t))
		mustFail(t, err, "Platform/test-platform declares no boot.spinnerFrames", "BootProfile/core")
	})
	t.Run("graphical without watermark keeps the platform theme", func(t *testing.T) {
		p := mustPlan(t, "ws", files.with(fixture{
			"ws.yaml":     workstation("ws", "core"),
			"b-core.yaml": boot("core", "  splash: graphical\n"),
		}).dir(t))
		if p.Artifact.PlymouthTheme != nil || installed(p) != "" {
			t.Errorf("theme %+v, files %q", p.Artifact.PlymouthTheme, installed(p))
		}
	})
	for name, tc := range map[string]struct {
		files fixture
		ref   string
		want  string
	}{
		"not a png":        {fixture{"fake.png": "not an image"}, "fake.png", "is not a PNG image"},
		"wrong extension":  {fixture{"mark.svg": "<svg/>"}, "mark.svg", "must be a .png file with a simple name"},
		"too large":        {fixture{"wide.png": pngOf(t, 1025, 1)}, "wide.png", "is 1025x1; at most 1024x1024 pixels"},
		"missing file":     {fixture{}, "absent.png", "absent.png"},
		"outside the root": {fixture{}, "../outside.png", "outside the resource root"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := tc.files.with(fixture{"b.yaml": boot("b", "  splash: graphical\n  watermark: "+tc.ref+"\n")}).dir(t)
			if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "outside.png"), []byte(corePNG), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := newCompiler(t).Load([]string{dir})
			mustFail(t, err, "BootProfile/b", "boot.watermark", tc.want)
		})
	}
}

func TestPackageGroupExcludePackages(t *testing.T) {
	platform := func(groups string) string {
		return strings.Replace(platformYAML, "  packageGroups:\n    - {name: workstation, rpmGroups: [workstation-product-environment], graphical: true}\n", groups, 1)
	}
	excluding := platform("  packageGroups:\n" +
		"    - {name: workstation, rpmGroups: [workstation-product-environment], graphical: true, excludePackages: [setroubleshoot-server, setroubleshoot]}\n" +
		"    - {name: office, rpmGroups: [office-suite], excludePackages: [setroubleshoot, libreoffice-help-en]}\n")
	fx := fixture{
		"platform.yaml": excluding,
		"ws.yaml":       workstation("ws", "core"),
		"core.yaml":     profile("core", "foundation", "PackageSet/a"),
		"a.yaml":        "apiVersion: software.deskos.org/v1alpha1\nkind: PackageSet\nmetadata:\n  name: a\nspec:\n  groups: [workstation, office]\n  packages: [git]\n",
	}
	p := mustPlan(t, "ws", fx.dir(t))
	got := map[string]string{}
	for _, g := range p.Artifact.PackageGroups {
		got[g.Name] = strings.Join(g.ExcludePackages, " ")
	}
	if got["workstation"] != "setroubleshoot setroubleshoot-server" || got["office"] != "libreoffice-help-en setroubleshoot" {
		t.Errorf("excludePackages in the plan = %v", got)
	}

	files, err := containerfile.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	cf := string(fileData(t, files, containerfile.ContainerfilePath))
	group, rest, _ := strings.Cut(cf[strings.Index(cf, "dnf -y group install"):], "\n\n")
	want := "'--exclude=libreoffice-help-en' \\\n        '--exclude=setroubleshoot' \\\n        '--exclude=setroubleshoot-server' \\\n        'office-suite' \\\n        'workstation-product-environment'"
	if !strings.Contains(group, want) {
		t.Errorf("group transaction does not exclude the union of excludePackages before the groups:\n%s", group)
	}
	if strings.Contains(rest, "--exclude") {
		t.Error("an exclusion leaks outside the group transaction")
	}

	// Explicit intent still installs an excluded group member.
	fx["a.yaml"] = "apiVersion: software.deskos.org/v1alpha1\nkind: PackageSet\nmetadata:\n  name: a\nspec:\n  groups: [workstation]\n  packages: [setroubleshoot]\n"
	if !slices.Contains(packages(mustPlan(t, "ws", fx.dir(t))), "setroubleshoot") {
		t.Error("an explicitly requested package was dropped")
	}

	bad := base.with(fixture{"platform.yaml": platform("  packageGroups:\n    - {name: workstation, rpmGroups: [workstation-product-environment], graphical: true, excludePackages: [setroubleshoot, setroubleshoot]}\n")}).dir(t)
	_, err = newCompiler(t).Load([]string{bad})
	mustFail(t, err, "excludePackages")
	bad = base.with(fixture{"platform.yaml": platform("  packageGroups:\n    - {name: workstation, rpmGroups: [workstation-product-environment], graphical: true, excludePackages: ['rm -rf /']}\n")}).dir(t)
	_, err = newCompiler(t).Load([]string{bad})
	mustFail(t, err, "excludePackages")
}

func TestPackageGroupExcludePackagesTypedValidationWithoutSchema(t *testing.T) {
	c := newCompiler(t)
	doc := `{"displayName":"T","family":"t","release":"1","architectures":["x86_64"],"bootc":{"image":"example.org/t:1"},
"distribution":{"redistributable":true,"requiresSubscription":false},"displayManager":"gdm.service",
"packageGroups":[{"name":"workstation","rpmGroups":["g"],"graphical":true,"excludePackages":["ok","ok","bad name"]}]}`
	res := rawResource("core.deskos.org/v1alpha1", "Platform", "t", doc)
	prov, err := c.Registry.Lookup(res.GVK())
	if err != nil {
		t.Fatal(err)
	}
	mustFail(t, prov.Decode(res), `package "ok" is excluded twice`, `invalid excluded package "bad name"`)
}

func fileData(t *testing.T, files []containerfile.File, path string) []byte {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return f.Data
		}
	}
	t.Fatalf("%s not rendered", path)
	return nil
}
