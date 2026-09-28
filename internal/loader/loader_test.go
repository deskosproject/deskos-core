package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/model"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const pkgSet = `apiVersion: software.deskos.org/v1alpha1
kind: PackageSet
metadata:
  name: %s
spec:
  packages: [git]
`

func TestLoadRepositoryResources(t *testing.T) {
	res, err := Load([]string{"../../resources"})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, r := range res {
		kinds[r.Kind]++
		if r.Source.Root != "resources" || strings.HasPrefix(r.Source.Path, "/") {
			t.Errorf("%s: source %+v is not root-relative", r.ID(), r.Source)
		}
	}
	for _, k := range []string{"Platform", "Profile", "Workstation", "PackageSet", "GnomeProfile", "FlatpakRemote", "FlatpakSet"} {
		if kinds[k] == 0 {
			t.Errorf("no %s loaded from resources/", k)
		}
	}
}

func TestLoadMultipleRootsIsOrderIndependent(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	write(t, a, "a.yaml", strings.ReplaceAll(pkgSet, "%s", "one"))
	write(t, b, "b.yaml", strings.ReplaceAll(pkgSet, "%s", "two"))
	r1, err := Load([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Load([]string{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if len(r1) != 2 || len(r2) != 2 {
		t.Fatalf("got %d and %d resources", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i].Source != r2[i].Source {
			t.Errorf("order depends on root order: %v vs %v", r1[i].Source, r2[i].Source)
		}
	}
}

func TestMultiDocument(t *testing.T) {
	data := "---\n" + strings.ReplaceAll(pkgSet, "%s", "one") + "---\n# empty\n---\n" + strings.ReplaceAll(pkgSet, "%s", "two")
	res, err := Parse([]byte(data), model.Source{Root: "r", Path: "multi.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Metadata.Name != "one" || res[1].Metadata.Name != "two" {
		t.Fatalf("unexpected resources: %v", res)
	}
	if res[1].Source.Line <= res[0].Source.Line {
		t.Errorf("document lines not tracked: %d, %d", res[0].Source.Line, res[1].Source.Line)
	}
}

func TestMalformed(t *testing.T) {
	cases := map[string]struct{ doc, want string }{
		"yaml syntax":    {"apiVersion: [unclosed\n", "malformed YAML"},
		"not a mapping":  {"- a\n- b\n", "must be a mapping"},
		"no apiVersion":  {"kind: PackageSet\nmetadata: {name: x}\nspec: {}\n", "missing or non-string apiVersion"},
		"bad apiVersion": {"apiVersion: v1\nkind: X\nmetadata: {name: x}\nspec: {}\n", "invalid apiVersion"},
		"no kind":        {"apiVersion: a.b/v1\nmetadata: {name: x}\nspec: {}\n", "missing or non-string kind"},
		"bad name":       {"apiVersion: a.b/v1\nkind: X\nmetadata: {name: Not_Valid}\nspec: {}\n", "metadata.name"},
		"no metadata":    {"apiVersion: a.b/v1\nkind: X\nspec: {}\n", "missing metadata"},
		"anchor":         {"apiVersion: a.b/v1\nkind: X\nmetadata: {name: x}\nspec: &s {a: 1}\n", "anchors are not supported"},
		"alias":          {"apiVersion: a.b/v1\nkind: X\nmetadata: {name: x}\nspec:\n  a: &v 1\n  b: *v\n", "not supported"},
		"merge key":      {"apiVersion: a.b/v1\nkind: X\nmetadata: {name: x}\nspec:\n  <<: {a: 1}\n", "merge keys"},
		"custom tag":     {"apiVersion: a.b/v1\nkind: X\nmetadata: {name: x}\nspec: !shell {a: 1}\n", "unsupported YAML tag"},
		"duplicate key":  {"apiVersion: a.b/v1\nkind: X\nkind: Y\nmetadata: {name: x}\n", "duplicate key"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.doc), model.Source{Root: "r", Path: "bad.yaml"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestSkipsHiddenAndNonYAML(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "ok.yaml", strings.ReplaceAll(pkgSet, "%s", "one"))
	write(t, dir, ".hidden/x.yaml", "not: [valid")
	write(t, dir, "notes.txt", "ignored")
	res, err := Load([]string{dir})
	if err != nil || len(res) != 1 {
		t.Fatalf("got %d resources, err %v", len(res), err)
	}
}

func TestSymlinkedResourceFiles(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	write(t, base, "outside/x.yaml", strings.ReplaceAll(pkgSet, "%s", "outside"))
	write(t, root, "real/y.yaml", strings.ReplaceAll(pkgSet, "%s", "inside"))
	if err := os.Symlink("../outside/x.yaml", filepath.Join(root, "escape.yaml")); err != nil {
		t.Fatal(err)
	}
	_, err := Load([]string{root})
	if err == nil || !strings.Contains(err.Error(), "escape.yaml") || !strings.Contains(err.Error(), "outside the resource root") {
		t.Fatalf("want escape refusal, got %v", err)
	}

	if err := os.Remove(filepath.Join(root, "escape.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real/y.yaml", filepath.Join(root, "alias.yaml")); err != nil {
		t.Fatal(err)
	}
	res, err := Load([]string{root})
	if err != nil || len(res) != 2 {
		t.Fatalf("symlink inside the root: %d resources, %v", len(res), err)
	}
	// A file named explicitly as a root is trusted as given.
	if res, err := Load([]string{filepath.Join(base, "outside", "x.yaml")}); err != nil || len(res) != 1 {
		t.Fatalf("explicit file root: %v", err)
	}
}
