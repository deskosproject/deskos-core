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
