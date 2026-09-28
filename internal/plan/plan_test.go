package plan

import (
	"strings"
	"testing"
)

func TestNormalizePlymouthTheme(t *testing.T) {
	theme := func() *Plan {
		p := &Plan{}
		p.Artifact.PlymouthTheme = &PlymouthTheme{Name: "deskos", Dir: "/usr/share/plymouth/themes/deskos", Watermark: "/usr/share/plymouth/themes/deskos/watermark.png"}
		return p
	}
	p := theme()
	p.Artifact.Files = []FileInstall{{Path: "/usr/share/plymouth/themes/deskos/watermark.png"}}
	if err := p.Normalize(); err == nil || !strings.Contains(err.Error(), "needs an initramfs regeneration") {
		t.Errorf("theme without initramfs: %v", err)
	}
	p = theme()
	p.Artifact.Initramfs = &InitramfsRegeneration{DracutModules: []string{"plymouth"}}
	if err := p.Normalize(); err == nil || !strings.Contains(err.Error(), "is not an image file of the plan") {
		t.Errorf("theme without watermark file: %v", err)
	}
	p.Artifact.Files = []FileInstall{{Path: "/usr/share/plymouth/themes/deskos/watermark.png"}}
	if err := p.Normalize(); err != nil {
		t.Errorf("complete theme: %v", err)
	}
}
