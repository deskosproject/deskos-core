package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
)

func TestAppendSystemFiles(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"usr/lib/tmpfiles.d/zz-deskos-relabel.conf",
		"usr/lib/systemd/system/fstrim.service.d/10-sysroot.conf",
	} {
		p := filepath.Join(root, "system_files", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(rel+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := appendSystemFiles(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	for _, f := range files {
		if !strings.HasPrefix(f.Path, "rootfs/usr/lib/") || f.Mode != 0o644 {
			t.Errorf("unexpected file %+v", f)
		}
	}
}

func TestAppendSystemFilesRejectsAnythingElse(t *testing.T) {
	for _, rel := range []string{
		"usr/lib/systemd/system/evil.service",
		"usr/bin/evil",
		"usr/lib/tmpfiles.d/nested/evil.conf",
	} {
		root := t.TempDir()
		p := filepath.Join(root, "system_files", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := appendSystemFiles(nil, []string{root}); err == nil {
			t.Errorf("%s was accepted", rel)
		}
	}
}

func TestAppendSystemFilesRejectsSymlinksAndDuplicates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "system_files", "usr/lib/tmpfiles.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.conf")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link.conf")); err != nil {
		t.Fatal(err)
	}
	if _, err := appendSystemFiles(nil, []string{root}); err == nil {
		t.Error("a symlink was accepted")
	}
	if err := os.Remove(filepath.Join(dir, "link.conf")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dup.conf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := appendSystemFiles([]containerfile.File{{Path: "rootfs/usr/lib/tmpfiles.d/dup.conf"}}, []string{root}); err == nil {
		t.Error("a duplicate destination was accepted")
	}
}
