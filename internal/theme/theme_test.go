package theme

import (
	"strings"
	"testing"
)

func TestPatchShellCSS(t *testing.T) {
	css := ".panel { background-color: #000000; color: #ffffff; }\n.x { border-color: #36363a; }"
	got := PatchShellCSS(css, Palette{Background: "#1a1b26", Foreground: "#a9b1d6"})
	for _, want := range []string{"background-color: #1a1b26", "color: #a9b1d6", "border-color: #1a1b26"} {
		if !strings.Contains(got, want) {
			t.Errorf("patched shell css lacks %q:\n%s", want, got)
		}
	}
}

func TestPtyxisProfile(t *testing.T) {
	got := Palette{Name: "gruvbox"}.PtyxisProfile("abc-123")
	for _, want := range []string{
		"[org/gnome/Ptyxis]",
		"default-profile-uuid='abc-123'",
		"[org/gnome/Ptyxis/Profiles/abc-123]",
		"palette='gruvbox'",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("ptyxis profile lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "[/") || strings.Contains(got, "//") {
		t.Errorf("ptyxis profile must not use bracketed relocatable sections:\n%s", got)
	}
}

func TestShellDconf(t *testing.T) {
	got := ShellDconf([]string{"dash-to-dock@micxgx.gmail.com"}, "deskos-a")
	for _, want := range []string{
		"enabled-extensions=['dash-to-dock@micxgx.gmail.com', 'user-theme@gnome-shell-extensions.gcampax.github.com']",
		"name='deskos-a'",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("shell dconf lacks %q:\n%s", want, got)
		}
	}
}

func TestGTK4CSSHierarchyAndAccent(t *testing.T) {
	p := Palette{Background: "#1a1b26", Foreground: "#a9b1d6"}
	css := p.GTK4CSS("blue", true)
	for _, want := range []string{
		"--accent-bg-color: var(--accent-blue);",
		"--window-bg-color: #1a1b26;",
		"--window-fg-color: #a9b1d6;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("gtk4 css lacks %q:\n%s", want, css)
		}
	}
	// The accent must come from the GNOME name, never a second hex.
	if strings.Contains(css, "--accent-color:") {
		t.Errorf("gtk4 css must not pin --accent-color:\n%s", css)
	}
	// Surfaces step away from the window color.
	if strings.Contains(css, "--card-bg-color: #1a1b26;") {
		t.Errorf("card is not shaded from the window color:\n%s", css)
	}
}

func TestAccentHex(t *testing.T) {
	if h, ok := AccentHex("blue"); !ok || h != "#3584e4" {
		t.Errorf(`AccentHex("blue") = %q, %v; want #3584e4`, h, ok)
	}
	if _, ok := AccentHex("nope"); ok {
		t.Error("AccentHex accepted an unknown accent")
	}
}

func TestNearestAccent(t *testing.T) {
	for color, want := range map[string]string{
		"#3584e4": "blue",
		"#3a944a": "green",
		"#e62d42": "red",
		"#ed5b00": "orange",
		"#7aa2f7": "blue",
		"#9ece6a": "green",
		"#f7768e": "red",
		"#b8bb26": "yellow",
		"#000000": "slate",
	} {
		if got, ok := NearestAccent(color); !ok || got != want {
			t.Errorf("NearestAccent(%q) = %q, %v; want %q", color, got, ok, want)
		}
	}
	for _, bad := range []string{"nope", "#12345", "#gggggg"} {
		if _, ok := NearestAccent(bad); ok {
			t.Errorf("NearestAccent(%q) accepted an invalid color", bad)
		}
	}
}
