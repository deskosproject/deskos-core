package containerfile

import (
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
