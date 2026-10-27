package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func etcTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "usr/etc/modified"), "image\n")
	mustWrite(t, filepath.Join(root, "etc/modified"), "local\n")
	mustWrite(t, filepath.Join(root, "usr/etc/removed"), "image\n")
	mustWrite(t, filepath.Join(root, "etc/added"), "local\n")
	mustWrite(t, filepath.Join(root, "etc/NetworkManager/system-connections/wifi"), "secret\n")
	policy := `{"ignore":["custom/"]}`
	mustWrite(t, filepath.Join(root, "usr/etc/deskos/deskos.json"), policy)
	mustWrite(t, filepath.Join(root, "etc/deskos/deskos.json"), policy)
	mustWrite(t, filepath.Join(root, "etc/custom/state"), "runtime\n")
	return root
}

func TestEtcDiffNamesWhatChangedAndWhatIsIgnored(t *testing.T) {
	root := etcTree(t)
	code, out, errOut := runCLI(t, "etc", "--root", root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{
		"Modified (1):", "/etc/modified",
		"Added (1):", "/etc/added",
		"Removed (1):", "/etc/removed",
		"Ignored: 3 runtime path(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("diff lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/etc/NetworkManager") {
		t.Errorf("ignored paths must not be listed without --ignored:\n%s", out)
	}

	code, out, _ = runCLI(t, "etc", "--root", root, "--ignored")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"/etc/NetworkManager/system-connections", "/etc/custom", "/etc/deskos"} {
		if !strings.Contains(out, want) {
			t.Errorf("--ignored lacks %q:\n%s", want, out)
		}
	}
}

func TestEtcResetOnePathKeepsABackup(t *testing.T) {
	root := etcTree(t)
	code, out, errOut := runCLI(t, "etc", "reset", "--root", root, "/etc/modified")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	got, err := os.ReadFile(filepath.Join(root, "etc/modified"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "image\n" {
		t.Errorf("reset content = %q", got)
	}
	backups, err := filepath.Glob(filepath.Join(root, "var/lib/deskos/etc-backup/*/modified"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup not found: %v %v", backups, err)
	}
	if b, _ := os.ReadFile(backups[0]); string(b) != "local\n" {
		t.Errorf("backup content = %q", b)
	}
	if !strings.Contains(out, "reset /etc/modified") {
		t.Errorf("out = %q", out)
	}
}

func TestEtcResetAllRestoresTheImageAndLeavesIgnoredPaths(t *testing.T) {
	root := etcTree(t)
	code, _, errOut := runCLI(t, "etc", "reset", "--all", "--root", root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/added")); !os.IsNotExist(err) {
		t.Errorf("local addition still exists: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "etc/removed"))
	if string(got) != "image\n" {
		t.Errorf("removed file not restored: %q", got)
	}
	got, _ = os.ReadFile(filepath.Join(root, "etc/modified"))
	if string(got) != "image\n" {
		t.Errorf("modified file not reset: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/NetworkManager/system-connections/wifi")); err != nil {
		t.Errorf("ignored runtime file was touched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/deskos/deskos.json")); err != nil {
		t.Errorf("policy file was touched: %v", err)
	}
}

func TestEtcResetDryRunChangesNothing(t *testing.T) {
	root := etcTree(t)
	code, out, errOut := runCLI(t, "etc", "reset", "--dry-run", "--root", root, "/etc/added")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(root, "etc/added")); err != nil {
		t.Errorf("dry-run deleted the file: %v", err)
	}
	if !strings.Contains(out, "would delete /etc/added") {
		t.Errorf("out = %q", out)
	}
}

func TestEtcRefusesANonBootcRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "etc/foo"), "x\n")
	code, _, errOut := runCLI(t, "etc", "--root", root)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "not a bootc deployment") || !strings.Contains(errOut, "usr/etc") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestEtcResetRejectsTraversal(t *testing.T) {
	root := etcTree(t)
	code, _, errOut := runCLI(t, "etc", "reset", "--root", root, "/etc/../../passwd")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(errOut, "not a path under /etc") {
		t.Errorf("stderr = %q", errOut)
	}
}
