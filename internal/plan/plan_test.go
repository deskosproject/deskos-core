package plan

import (
	"strings"
	"testing"
)

func TestNormalizeDesktopEntries(t *testing.T) {
	const exec, icon = "/usr/local/bin/app", "/usr/share/icons/hicolor/48x48/apps/org.example.App.png"
	entry := func() *Plan {
		p := &Plan{}
		p.Artifact.DesktopEntries = []DesktopEntry{{ID: "org.example.App", Exec: exec, IconPath: icon}}
		return p
	}
	p := entry()
	p.Artifact.Files = []FileInstall{{Path: icon}}
	if err := p.Normalize(); err == nil || !strings.Contains(err.Error(), "is not a binary destination of the plan") {
		t.Errorf("entry without its binary: %v", err)
	}
	p = entry()
	p.Artifact.Binaries = []VerifiedBinaryInstall{{Destination: exec}}
	if err := p.Normalize(); err == nil || !strings.Contains(err.Error(), "is not an image file of the plan") {
		t.Errorf("entry without its icon: %v", err)
	}
	p = entry()
	p.Artifact.Binaries = []VerifiedBinaryInstall{{Destination: exec}}
	p.Artifact.Files = []FileInstall{{Path: icon}}
	p.Artifact.DesktopEntries = append(p.Artifact.DesktopEntries, p.Artifact.DesktopEntries[0])
	if err := p.Normalize(); err == nil || !strings.Contains(err.Error(), "defined twice") {
		t.Errorf("duplicate entry: %v", err)
	}
}

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

func TestNormalizeTrustAnchors(t *testing.T) {
	anchor := func() *Plan {
		p := &Plan{}
		p.Artifact.TrustAnchors = []TrustAnchorInstall{{Name: "root", Path: "/etc/pki/ca-trust/source/anchors/root.crt"}}
		p.Artifact.TrustStore = &TrustStoreUpdate{Command: "update-ca-trust"}
		return p
	}
	// An anchor needs its trust store update.
	p := anchor()
	p.Artifact.TrustStore = nil
	if err := p.Normalize(); err == nil || !strings.Contains(err.Error(), "need a trust store update") {
		t.Errorf("anchor without store: %v", err)
	}
	// A trust store update without anchors is an error.
	p = &Plan{}
	p.Artifact.TrustStore = &TrustStoreUpdate{Command: "update-ca-trust"}
	if err := p.Normalize(); err == nil || !strings.Contains(err.Error(), "needs at least one trust anchor") {
		t.Errorf("store without anchor: %v", err)
	}
	// An anchor may not share a destination with an image file.
	p = anchor()
	p.Artifact.Files = []FileInstall{{Path: "/etc/pki/ca-trust/source/anchors/root.crt"}}
	if err := p.Normalize(); err == nil || !strings.Contains(err.Error(), "share the destination") {
		t.Errorf("anchor colliding with an image file: %v", err)
	}
	// Two anchors under one name are an error.
	p = anchor()
	p.Artifact.TrustAnchors = append(p.Artifact.TrustAnchors, p.Artifact.TrustAnchors[0])
	if err := p.Normalize(); err == nil || !strings.Contains(err.Error(), "defined twice") {
		t.Errorf("duplicate anchor: %v", err)
	}
	// A complete plan normalizes.
	if err := anchor().Normalize(); err != nil {
		t.Errorf("complete plan: %v", err)
	}
}
