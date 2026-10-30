package compiler_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
)

// The platform's manifest is the set its groups must resolve to; the build fails
// when the installed set differs (docs/adr/0028).
func TestPlatformPackageManifestReachesThePlan(t *testing.T) {
	platform := strings.Replace(platformYAML, "  packageGroups:",
		"  packageManifest: manifest.txt\n  packageGroups:", 1)
	if platform == platformYAML {
		t.Fatal("the fixture no longer declares packageGroups")
	}
	dir := fixture{"platform.yaml": platform}.with(fixture{
		"manifest.txt": "# one name per line\n\ncoreutils\nbash\n",
		"ws.yaml":      workstation("ws", "core"),
		"core.yaml":    profile("core", "foundation"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if !slices.Equal(p.Artifact.PackageManifest, []string{"bash", "coreutils"}) {
		t.Errorf("manifest = %v, want sorted and without comments or blanks", p.Artifact.PackageManifest)
	}
}

func TestPlatformPackageManifestIsChecked(t *testing.T) {
	load := func(manifest string) error {
		platform := strings.Replace(platformYAML, "  packageGroups:",
			"  packageManifest: manifest.txt\n  packageGroups:", 1)
		dir := fixture{"platform.yaml": platform}.with(fixture{
			"manifest.txt": manifest,
			"ws.yaml":      workstation("ws", "core"),
			"core.yaml":    profile("core", "foundation"),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		return err
	}
	if err := load("bash\n"); err != nil {
		t.Fatalf("a one-name manifest is valid: %v", err)
	}
	mustFail(t, load(""), "empty")
	mustFail(t, load("bash\nbash\n"), "listed twice")
	mustFail(t, load("bash\nnot a package\n"), "not a package name")
}

func TestTheBuildChecksTheManifest(t *testing.T) {
	render := func(manifest []string) string {
		p := &plan.Plan{Platform: plan.Platform{Name: "test", PackageManager: model.PackageManagerRpm}}
		p.Artifact.PackageManifest = manifest
		files, err := containerfile.Render(p)
		if err != nil {
			t.Fatal(err)
		}
		var cf string
		for _, f := range files {
			if f.Path == containerfile.ContainerfilePath {
				cf = string(f.Data)
			}
		}
		return cf
	}
	cf := render([]string{"bash", "coreutils"})
	for _, want := range []string{
		"COPY manifest/ /tmp/deskos-manifest/",
		"diff -u /tmp/deskos-manifest/packages.txt",
		"the installed packages differ from the platform manifest",
	} {
		if !strings.Contains(cf, want) {
			t.Errorf("the build lacks %q:\n%s", want, cf)
		}
	}
	if got := render(nil); strings.Contains(got, "deskos-manifest") {
		t.Errorf("a plan without a manifest renders a check:\n%s", got)
	}
}
