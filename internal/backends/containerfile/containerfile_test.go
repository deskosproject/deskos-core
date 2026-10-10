package containerfile

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/plan"
)

func TestPlymouthThemeNeedsInitramfs(t *testing.T) {
	p := &plan.Plan{}
	p.Artifact.PlymouthTheme = &plan.PlymouthTheme{Name: "deskos", Dir: "/usr/share/plymouth/themes/deskos"}
	if _, err := Render(p); err == nil || !strings.Contains(err.Error(), "needs an initramfs regeneration") {
		t.Errorf("Render = %v", err)
	}
}

func TestPlymouthThemeFile(t *testing.T) {
	got := string(plymouthTheme(&plan.PlymouthTheme{Name: "deskos", Dir: "/usr/share/plymouth/themes/deskos"}))
	for _, want := range []string{"\nName=deskos\n", "\nModuleName=two-step\n", "\nImageDir=/usr/share/plymouth/themes/deskos\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("theme file lacks %q", want)
		}
	}
	if strings.Contains(got, "UseFirmwareBackground=true") {
		t.Error("a mode uses the firmware (BGRT) logo")
	}
	for _, mode := range []string{"boot-up", "shutdown", "reboot", "updates", "system-upgrade", "firmware-upgrade", "system-reset"} {
		_, section, ok := strings.Cut(got, "\n["+mode+"]\n")
		if section, _, _ = strings.Cut(section, "\n["); !ok || !strings.Contains(section, "UseFirmwareBackground=false\n") {
			t.Errorf("[%s] does not disable the firmware background", mode)
		}
	}
}

// A tar.gz member is installed only after the extraction proves it is a plain
// file: tar would extract a symlink or hardlink member as such, and install
// would then copy whatever it points at.
func TestBinaryArchiveMemberGuard(t *testing.T) {
	p := &plan.Plan{}
	p.Artifact.Binaries = []plan.VerifiedBinaryInstall{{
		Artifact: "tool", Version: "1", URL: "https://example.org/tool.tar.gz",
		SHA256: strings.Repeat("a", 64), Archive: "tar.gz", Member: "tool",
		Destination: "/usr/local/bin/tool", Mode: "0755",
	}}
	files, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	var cf string
	for _, f := range files {
		if f.Path == ContainerfilePath {
			cf = string(f.Data)
		}
	}
	for _, want := range []string{`tar -xzf "$tmp/download"`, `[ ! -L `, `stat -c %h`, "is not a regular file"} {
		if !strings.Contains(cf, want) {
			t.Errorf("the binary extraction lacks %q:\n%s", want, cf)
		}
	}
}

// The guard rejects a symlink member and accepts a regular one; without it,
// install would copy the target of the symlink into the image.
func TestBinaryArchiveGuardRejectsSymlinkMember(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte("TOPSECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(dir, "stage")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "good"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(stage, "evil")); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "a.tar.gz")
	if out, err := exec.Command("tar", "-czf", archive, "-C", stage, "good", "evil").CombinedOutput(); err != nil {
		t.Skipf("tar is unavailable: %v (%s)", err, out)
	}
	for _, m := range []struct {
		name   string
		reject bool
	}{{"evil", true}, {"good", false}} {
		dst := filepath.Join(dir, "out-"+m.name)
		// The guard exactly as the backend renders it.
		script := fmt.Sprintf(`set -eu; tmp="$(mktemp -d)"; tar -xzf %q -C "$tmp" --no-same-owner -- %q; f="$tmp"/%q; [ -f "$f" ] && [ ! -L "$f" ] && [ "$(stat -c %%h "$f")" = 1 ] || { echo 'not a regular file' >&2; exit 1; }; install -D -m 0755 "$f" %q`,
			archive, m.name, m.name, dst)
		if err := exec.Command("sh", "-c", script).Run(); (err != nil) != m.reject {
			t.Errorf("member %q: err=%v, want rejected=%v", m.name, err, m.reject)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "out-evil")); err == nil && bytes.Contains(data, []byte("TOPSECRET")) {
		t.Error("the symlink member leaked the file it points at")
	}
}
