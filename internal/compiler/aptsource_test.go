package compiler_test

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// aptVSCodeSpec is a valid APT source; a case rewrites the line it varies.
const aptVSCodeSpec = `  id: vscode
  displayName: Visual Studio Code - Stable
  baseURL: https://packages.microsoft.com/repos/code
  suite: stable
  components: [main]
  architectures: [amd64]
  gpgKeyFiles: [assets/apt-key.asc]
`

func aptRepo(name, spec string) string {
	return fmt.Sprintf("apiVersion: software.deskos.org/v1alpha1\nkind: AptRepository\nmetadata:\n  name: %s\nspec:\n%s", name, spec)
}

// debPlatform is the test platform with the package manager of ADR 0015: the same
// fixture, with the group members a deb platform declares instead of RPM groups.
func debPlatform(t *testing.T) string {
	t.Helper()
	doc := strings.Replace(platformYAML, "  packageManager: rpm\n", "  packageManager: deb\n", 1)
	doc = strings.Replace(doc, "rpmGroups: [workstation-product-environment]", "members: [test-desktop]", 1)
	if doc == platformYAML {
		t.Fatal("the platform fixture no longer carries the RPM facts this rewrites, so the test would prove nothing")
	}
	return doc
}

func aptKeySHA() string {
	sum := sha256.Sum256([]byte(testArmoredKey()))
	return fmt.Sprintf("%x", sum)
}

// A PackageSet brings in the sources it needs, and the reference is typed: the
// set names an AptRepository, so the wrong platform reports the gap instead of
// including it where nothing reads it (invariant 9).
func TestPackageSetIncludesItsAptRepository(t *testing.T) {
	set := packageSet("vendor-tools", "code") + "  aptRepositories: [vscode]\n"
	files := fixture{
		"p.yaml":             profile("core", "foundation", "PackageSet/vendor-tools"),
		"set.yaml":           set,
		"apt.yaml":           aptRepo("vscode", aptVSCodeSpec),
		"assets/apt-key.asc": testArmoredKey(),
		"ws.yaml":            workstation("ws", "core"),
	}
	deb := fixture{"platform.yaml": debPlatform(t)}.with(files)
	p := mustPlan(t, "ws", deb.dir(t))
	if n := len(p.Artifact.AptRepositories); n != 1 {
		t.Fatalf("the source the set names was not included: %d sources", n)
	}
	_, err := planOf(t, "ws", base.with(files).dir(t))
	mustFail(t, err, "an APT source requires an APT platform", "AptRepository/vscode", "PackageSet/vendor-tools")
}

// The mirror of TestRpmRepositoryOnADebPlatformIsReported. A repository belongs
// to one package manager's world from either side, and asking the other platform
// for it is a gap to report, not a file to write where nothing reads it
// (invariant 9).
func TestAptSourceOnAnRpmPlatformIsReported(t *testing.T) {
	dir := base.with(fixture{
		"p.yaml":             profile("core", "foundation", "AptRepository/vscode"),
		"apt.yaml":           aptRepo("vscode", aptVSCodeSpec),
		"assets/apt-key.asc": testArmoredKey(),
		"ws.yaml":            workstation("ws", "core"),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, "an APT source requires an APT platform", "Platform/test-platform is rpm", "AptRepository/vscode")
}

// On the platform whose manager reads it, the source lands in the plan whole:
// the fields apt acts on, the suite and components as one decision, and the key
// with the sha256 of the very bytes the image will carry.
func TestAptSourceLandsInThePlanOnADebPlatform(t *testing.T) {
	dir := fixture{
		"platform.yaml":      debPlatform(t),
		"p.yaml":             profile("core", "foundation", "AptRepository/vscode"),
		"apt.yaml":           aptRepo("vscode", aptVSCodeSpec),
		"assets/apt-key.asc": testArmoredKey(),
		"ws.yaml":            workstation("ws", "core"),
	}.dir(t)
	p := mustPlan(t, "ws", dir)
	if n := len(p.Artifact.AptRepositories); n != 1 {
		t.Fatalf("got %d APT sources, want 1", n)
	}
	src := p.Artifact.AptRepositories[0]
	if src.ID != "vscode" || src.Suite != "stable" || src.BaseURL != "https://packages.microsoft.com/repos/code" {
		t.Errorf("source = %+v", src)
	}
	if got := strings.Join(src.Components, " "); got != "main" {
		t.Errorf("components = %q", got)
	}
	if got := strings.Join(src.Architectures, " "); got != "amd64" {
		t.Errorf("architectures = %q", got)
	}
	if n := len(src.GPGKeys); n != 1 {
		t.Fatalf("got %d keys, want 1", n)
	}
	if src.GPGKeys[0].SHA256 != aptKeySHA() || src.GPGKeys[0].Content != testArmoredKey() {
		t.Error("the key in the plan is not the bytes of the declared asset")
	}
	if len(src.Provenance) == 0 {
		t.Error("the source carries no provenance")
	}
	// An RPM platform's plan is unaffected: the field exists and is empty, so a
	// consumer never has to guess between absent and empty.
	rpm := mustPlan(t, "deskos-core-centos10", resourcesRoot)
	if rpm.Artifact.AptRepositories == nil {
		t.Error("aptRepositories is nil on an RPM plan, want []")
	}
}

// apt decides what a source means by the trailing slash, so the two halves are
// validated as one decision rather than two independent fields. Every case here
// passes the schema, which cannot express this; the typed decoder is the layer
// that does.
func TestAptSourceRefusesWhatAptWouldRefuse(t *testing.T) {
	without := func(line string) string {
		return strings.Replace(aptVSCodeSpec, line+"\n", "", 1)
	}
	for name, tc := range map[string]struct {
		spec string
		want string
	}{
		"named suite without components": {
			spec: without("  components: [main]"),
			want: "at least one component is required",
		},
		"exact path with components": {
			// The Kubernetes archive is the real case: its suite is a path.
			spec: strings.Replace(aptVSCodeSpec, "  suite: stable\n", "  suite: /stable/v1.37/\n", 1),
			want: "is a path, so components must be omitted",
		},
		"an internal slash is not a path": {
			spec: strings.Replace(aptVSCodeSpec, "  suite: stable\n", "  suite: stable/v1.37\n", 1),
			want: "neither a suite name nor a path ending in a slash",
		},
		"a suite with a space": {
			spec: strings.Replace(aptVSCodeSpec, "  suite: stable\n", "  suite: \"stable main\"\n", 1),
			want: "suite is required and must be a single value",
		},
		"a key that is not armored": {
			spec: strings.Replace(aptVSCodeSpec, "assets/apt-key.asc", "assets/not-a-key.asc", 1),
			want: "must be an ASCII-armored OpenPGP public key",
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := fixture{
				"platform.yaml":        debPlatform(t),
				"p.yaml":               profile("core", "foundation", "AptRepository/x"),
				"apt.yaml":             aptRepo("x", tc.spec),
				"assets/apt-key.asc":   testArmoredKey(),
				"assets/not-a-key.asc": "not a key at all\n",
				"ws.yaml":              workstation("ws", "core"),
			}.dir(t)
			_, err := planOf(t, "ws", dir)
			mustFail(t, err, tc.want)
		})
	}
}

// The shapes the schema already types: the public path refuses them before the
// decoder sees them, and the typed decoder holds the same line when it is
// reached without the schema (the armor frame and the source id, which apt
// treats as a file name).
func TestAptSourceTypedRulesHoldWithoutTheSchema(t *testing.T) {
	const group = "software.deskos.org/v1alpha1"
	c := newCompiler(t)
	decode := func(spec string) error {
		t.Helper()
		res := rawResource(group, "AptRepository", "x", spec)
		prov, err := c.Registry.Lookup(res.GVK())
		if err != nil {
			t.Fatal(err)
		}
		return prov.Decode(res)
	}
	base := `"displayName":"x","baseURL":"https://example.org/deb/","suite":"stable","components":["main"],"gpgKeyFiles":["k.asc"]`
	for name, tc := range map[string]struct {
		spec string
		want string
	}{
		"an id apt would ignore": {
			spec: `{"id":"code:stable",` + base + `}`,
			want: "apt ignores a file in sources.list.d",
		},
		"no key at all": {
			spec: `{"id":"x","displayName":"x","baseURL":"https://example.org/deb/","suite":"stable","components":["main"]}`,
			want: "gpgKeyFiles are required",
		},
		"a component that is a path": {
			spec: `{"id":"x",` + strings.Replace(base, `"main"`, `"main/stable"`, 1) + `}`,
			want: "invalid component",
		},
		"an architecture with a dot": {
			spec: `{"id":"x",` + base + `,"architectures":["amd64.v2"]}`,
			want: "invalid architecture",
		},
		"an http archive": {
			spec: `{"id":"x",` + strings.Replace(base, "https://", "http://", 1) + `}`,
			want: "must be an https URL",
		},
	} {
		t.Run(name, func(t *testing.T) {
			mustFail(t, decode(tc.spec), tc.want)
		})
	}

	// The schema refuses the same documents on the public path: a name apt would
	// ignore, a component that is a path, an architecture with a dot, a plain
	// http archive.
	for name, doc := range map[string]string{
		"an id apt would ignore":     aptRepo("x", strings.Replace(aptVSCodeSpec, "  id: vscode", "  id: code:stable", 1)),
		"a component that is a path": aptRepo("x", strings.Replace(aptVSCodeSpec, "  components: [main]", "  components: [main/stable]", 1)),
		"an architecture with a dot": aptRepo("x", strings.Replace(aptVSCodeSpec, "  architectures: [amd64]", "  architectures: [amd64.v2]", 1)),
		"an http archive":            aptRepo("x", strings.Replace(aptVSCodeSpec, "https://packages.microsoft.com/repos/code", "http://packages.microsoft.com/repos/code", 1)),
		"no key at all":              aptRepo("x", strings.Replace(aptVSCodeSpec, "  gpgKeyFiles: [assets/apt-key.asc]\n", "", 1)),
	} {
		t.Run("schema/"+name, func(t *testing.T) {
			_, err := c.Load([]string{fixture{"x.yaml": doc}.dir(t)})
			mustFail(t, err, "schema validation failed")
		})
	}
}
