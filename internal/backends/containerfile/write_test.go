package containerfile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/plan"
)

func sample() []File {
	return []File{
		{Path: "Containerfile", Mode: 0o644, Data: []byte("FROM scratch\n")},
		{Path: "rootfs/usr/share/x", Mode: 0o644, Data: []byte("x\n")},
		{Path: "rootfs/usr/bin/tool", Mode: 0o755, Data: []byte("#!/bin/sh\n")},
	}
}

// render writes a valid previous render (with its manifest) into dir.
func render(t *testing.T, dir string, files []File) {
	t.Helper()
	if err := WriteDir(dir, withManifest(t, files)); err != nil {
		t.Fatal(err)
	}
}

func withManifest(t *testing.T, files []File) []File {
	t.Helper()
	var entries []manifestEntry
	for _, f := range files {
		sum := sha256.Sum256(f.Data)
		entries = append(entries, manifestEntry{Path: f.Path, Mode: fmt.Sprintf("%04o", f.Mode.Perm()), SHA256: hex.EncodeToString(sum[:])})
	}
	return append(append([]File(nil), files...), File{Path: ManifestPath, Mode: 0o644, Data: manifestJSON(t, entries)})
}

func manifestJSON(t *testing.T, entries []manifestEntry) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"format": manifestFormat, "backend": Name, "workstation": "w", "files": entries})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func write(t *testing.T, p, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// snapshot records every path under dir with its type, mode and content
// (or link target), without following symlinks.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		v := info.Mode().String()
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(p)
			v += " -> " + target
		case info.Mode().IsRegular():
			data, _ := os.ReadFile(p)
			v += " " + string(data)
		}
		out[p] = v
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameSnapshot(t *testing.T, before, after map[string]string) {
	t.Helper()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s changed: %q -> %q", k, v, after[k])
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s was created", k)
		}
	}
}

func TestWriteDirRefusesUnsafeOutput(t *testing.T) {
	entry := func(p string) manifestEntry {
		return manifestEntry{Path: p, Mode: "0644", SHA256: strings.Repeat("a", 64)}
	}
	cases := map[string]struct {
		setup func(t *testing.T, base, out string)
		want  string
	}{
		"manifest traversal to sibling": {func(t *testing.T, base, out string) {
			write(t, filepath.Join(out, ManifestPath), `{"files":[{"path":"../sentinel.txt"}]}`)
		}, "invalid"},
		"manifest traversal with valid header": {func(t *testing.T, base, out string) {
			write(t, filepath.Join(out, ManifestPath), string(manifestJSON(t, []manifestEntry{entry("../sentinel.txt")})))
		}, "not a canonical relative path"},
		"manifest absolute path": {func(t *testing.T, base, out string) {
			write(t, filepath.Join(out, ManifestPath), string(manifestJSON(t, []manifestEntry{entry(filepath.Join(base, "sentinel.txt"))})))
		}, "not a canonical relative path"},
		"manifest non-canonical spellings": {func(t *testing.T, base, out string) {
			write(t, filepath.Join(out, ManifestPath), string(manifestJSON(t, []manifestEntry{entry("a/../b"), entry("./c"), entry("d//e")})))
		}, "not a canonical relative path"},
		"manifest duplicate": {func(t *testing.T, base, out string) {
			write(t, filepath.Join(out, ManifestPath), string(manifestJSON(t, []manifestEntry{entry("x"), entry("x")})))
		}, "listed twice"},
		"manifest malformed": {func(t *testing.T, base, out string) {
			write(t, filepath.Join(out, ManifestPath), `{"format": `)
		}, "invalid"},
		"manifest unknown field": {func(t *testing.T, base, out string) {
			write(t, filepath.Join(out, ManifestPath), `{"format":"`+manifestFormat+`","backend":"containerfile","files":[],"delete":["../x"]}`)
		}, "unknown field"},
		"manifest bad hash": {func(t *testing.T, base, out string) {
			write(t, filepath.Join(out, ManifestPath), string(manifestJSON(t, []manifestEntry{{Path: "x", Mode: "0644", SHA256: "nope"}})))
		}, "invalid sha256"},
		"foreign file without manifest": {func(t *testing.T, base, out string) {
			write(t, filepath.Join(out, "notes.txt"), "mine")
		}, "not a previous render"},
		"foreign file beside a valid render": {func(t *testing.T, base, out string) {
			render(t, out, sample())
			write(t, filepath.Join(out, "notes.txt"), "mine")
		}, "not part of the previous render"},
		"foreign empty directory": {func(t *testing.T, base, out string) {
			render(t, out, sample())
			os.Mkdir(filepath.Join(out, "mine"), 0o755)
		}, "directory mine is not part"},
		"edited generated file": {func(t *testing.T, base, out string) {
			render(t, out, sample())
			write(t, filepath.Join(out, "Containerfile"), "FROM edited\n")
		}, "was edited"},
		"changed mode": {func(t *testing.T, base, out string) {
			render(t, out, sample())
			os.Chmod(filepath.Join(out, "Containerfile"), 0o600)
		}, "changed mode"},
		"output directory is a symlink": {func(t *testing.T, base, out string) {
			os.Remove(out)
			real := filepath.Join(base, "real")
			render(t, real, sample())
			os.Symlink(real, out)
		}, "is a symbolic link"},
		"output path is a file": {func(t *testing.T, base, out string) {
			os.Remove(out)
			write(t, out, "file")
		}, "not a directory"},
		"symlink entry listed in manifest": {func(t *testing.T, base, out string) {
			render(t, out, sample())
			os.Remove(filepath.Join(out, "Containerfile"))
			os.Symlink(filepath.Join(base, "sentinel.txt"), filepath.Join(out, "Containerfile"))
		}, "symbolic link"},
		"symlinked parent directory": {func(t *testing.T, base, out string) {
			render(t, out, sample())
			outside := filepath.Join(base, "outside")
			write(t, filepath.Join(outside, "x"), "outside")
			os.RemoveAll(filepath.Join(out, "rootfs/usr/share"))
			os.Symlink(outside, filepath.Join(out, "rootfs/usr/share"))
		}, "symbolic link"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			write(t, filepath.Join(base, "sentinel.txt"), "keep")
			out := filepath.Join(base, "out")
			if err := os.Mkdir(out, 0o755); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, base, out)
			before := snapshot(t, base)
			err := WriteDir(out, withManifest(t, sample()))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
			sameSnapshot(t, before, snapshot(t, base))
		})
	}
}

// A symlinked output directory is rejected however it is spelled.
func TestWriteDirRejectsSymlinkOutputSpellings(t *testing.T) {
	for _, suffix := range []string{"", "/", "/.", "//", "/./"} {
		t.Run("out"+suffix, func(t *testing.T) {
			base := t.TempDir()
			real := filepath.Join(base, "real")
			render(t, real, sample())
			if err := os.Symlink(real, filepath.Join(base, "out")); err != nil {
				t.Fatal(err)
			}
			before := snapshot(t, base)
			err := WriteDir(filepath.Join(base, "out")+suffix, withManifest(t, sample()))
			if err == nil || !strings.Contains(err.Error(), "is a symbolic link") {
				t.Fatalf("want symlink refusal, got %v", err)
			}
			sameSnapshot(t, before, snapshot(t, base))
		})
	}
}

func TestWriteDirRejectsEscapingNewFiles(t *testing.T) {
	for name, files := range map[string][]File{
		"traversal":    {{Path: "../escape", Mode: 0o644}},
		"absolute":     {{Path: "/tmp/escape", Mode: 0o644}},
		"dot segment":  {{Path: "a/./b", Mode: 0o644}},
		"empty":        {{Path: "", Mode: 0o644}},
		"duplicate":    {{Path: "a", Mode: 0o644}, {Path: "a", Mode: 0o644}},
		"file and dir": {{Path: "a", Mode: 0o644}, {Path: "a/b", Mode: 0o644}},
		"setuid mode":  {{Path: "a", Mode: 0o4755}},
		"backslash":    {{Path: `a\..\b`, Mode: 0o644}},
	} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			out := filepath.Join(base, "out")
			render(t, out, sample())
			before := snapshot(t, base)
			if err := WriteDir(out, files); err == nil {
				t.Fatal("expected an error")
			}
			sameSnapshot(t, before, snapshot(t, base))
		})
	}
}

func TestWriteDirValidRenders(t *testing.T) {
	base := t.TempDir()
	// Symlinked ancestors are legitimate (for example /home -> /var/home).
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(base, "link", "ctx")
	render(t, out, sample())
	first := snapshot(t, real)

	// Re-rendering over an unmodified previous render gives the same tree.
	render(t, out, sample())
	sameSnapshot(t, first, snapshot(t, real))

	// A listed file that was deleted is not an obstacle.
	if err := os.Remove(filepath.Join(out, "rootfs/usr/share/x")); err != nil {
		t.Fatal(err)
	}
	render(t, out, sample())
	sameSnapshot(t, first, snapshot(t, real))

	// A smaller render removes files and directories it no longer produces.
	render(t, out, sample()[:1])
	if _, err := os.Stat(filepath.Join(out, "rootfs")); !os.IsNotExist(err) {
		t.Errorf("stale directory kept: %v", err)
	}
	if info, err := os.Stat(filepath.Join(real, "ctx", "Containerfile")); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("Containerfile: %v %v", info, err)
	}
}

func TestRenderRejectsRepositoryNamedLikeRHSM(t *testing.T) {
	p := &plan.Plan{Format: plan.Format}
	p.Artifact.BaseImage.Ref = "example.org/base:1"
	p.Artifact.RpmRepositories = []plan.RpmRepository{{ID: "redhat", DisplayName: "x", BaseURL: "https://example.org/x", Enabled: true}}
	p.Artifact.RpmPackages = []plan.RpmInstall{{Name: "x"}}
	if err := p.Normalize(); err != nil {
		t.Fatal(err)
	}
	if _, err := Render(p); err == nil || !strings.Contains(err.Error(), "reserve for subscription-manager") {
		t.Fatalf("want reserved-name error, got %v", err)
	}
}
