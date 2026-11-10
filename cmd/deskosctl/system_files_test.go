package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSystemFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, "system_files", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSystemFiles(t *testing.T) {
	root := t.TempDir()
	writeSystemFile(t, root, "usr/lib/tmpfiles.d/zz-deskos-relabel.conf", "d /var/home 0755 root root -\n")
	writeSystemFile(t, root, "usr/lib/systemd/system/fstrim.service.d/10-sysroot.conf",
		"[Service]\nExecStart=\nExecStart=/usr/sbin/fstrim --listed-in /proc/self/mountinfo\n")
	files, err := systemFiles(root, nil)
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

func TestSystemFilesRejectsOrganizationRoots(t *testing.T) {
	core := t.TempDir()
	org := t.TempDir()
	writeSystemFile(t, org, "usr/lib/systemd/system/gdm.service.d/50-org.conf",
		"[Service]\nExecStartPre=/bin/sh -c true\n")
	if _, err := systemFiles(core, []string{org}); err == nil {
		t.Fatal("an organization system_files tree was accepted")
	}
}

func TestSystemFilesRejectsAnythingElse(t *testing.T) {
	for _, rel := range []string{
		"usr/lib/systemd/system/evil.service",
		"usr/bin/evil",
		"usr/lib/tmpfiles.d/nested/evil.conf",
	} {
		root := t.TempDir()
		writeSystemFile(t, root, rel, "x")
		if _, err := systemFiles(root, nil); err == nil {
			t.Errorf("%s was accepted", rel)
		}
	}
}

func TestSystemFilesRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "system_files", "usr/lib/tmpfiles.d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.conf")
	if err := os.WriteFile(target, []byte("d /tmp 0755 root root -\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link.conf")); err != nil {
		t.Fatal(err)
	}
	if _, err := systemFiles(root, nil); err == nil {
		t.Error("a symlink was accepted")
	}
}

func TestSystemFilesRejectsUnsafeTmpfilesTypes(t *testing.T) {
	for _, line := range []string{
		"f /etc/motd 0644 root root - hello\n",
		"w /etc/motd - - - - hello\n",
		"L /etc/motd - - - - /tmp/x\n",
		"C /etc/motd 0644 root root - /tmp/x\n",
		"c /dev/x 0600 root root - 1 3\n",
	} {
		root := t.TempDir()
		writeSystemFile(t, root, "usr/lib/tmpfiles.d/evil.conf", line)
		if _, err := systemFiles(root, nil); err == nil {
			t.Errorf("tmpfiles line %q was accepted", line)
		}
	}
}

func TestSystemFilesRejectsUnsafeDropinKeys(t *testing.T) {
	for _, line := range []string{
		"ExecStartPre=/bin/sh -c true",
		"ExecStopPost=/bin/sh -c true",
		"Environment=LD_PRELOAD=/tmp/x.so",
		"EnvironmentFile=/tmp/env",
		"StandardInput=file:/tmp/x",
		"User=root",
		"LoadCredential=x:/tmp/x",
	} {
		root := t.TempDir()
		writeSystemFile(t, root, "usr/lib/systemd/system/gdm.service.d/50-org.conf", "[Service]\n"+line+"\n")
		if _, err := systemFiles(root, nil); err == nil {
			t.Errorf("drop-in key %q was accepted", line)
		}
	}
}

func TestSystemFilesAllowExecStartReplacement(t *testing.T) {
	root := t.TempDir()
	writeSystemFile(t, root, "usr/lib/systemd/system/fstrim.service.d/10-sysroot.conf",
		"[Service]\nExecStart=\nExecStart=/usr/sbin/fstrim --listed-in /proc/self/mountinfo\n")
	if _, err := systemFiles(root, nil); err != nil {
		t.Fatal(err)
	}
}
