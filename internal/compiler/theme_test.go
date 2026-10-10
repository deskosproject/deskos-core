package compiler_test

import (
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
)

func theme(name, spec string) string {
	return "apiVersion: desktop.deskos.org/v1alpha1\nkind: Theme\nmetadata:\n  name: " + name + "\nspec:\n" + spec
}

// A Theme maps an palette-style palette (mode, a free accent, assets) onto
// the appearance surfaces GNOME itself defines.
func TestThemeMapsAppearanceToGNOME(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "Theme/t"),
		"t.yaml": theme("t", ""+
			"  mode: dark\n"+
			"  accent: \"#7aa2f7\"\n"+
			"  iconTheme: Yaru-blue\n"+
			"  cursorTheme: Adwaita\n"+
			"  fonts:\n"+
			"    monospace: { family: JetBrains Mono, size: 11 }\n"+
			"  wallpaper:\n"+
			"    light: ws.svg\n"+
			"    dark: wd.svg\n"),
		"ws.svg": "<svg/>",
		"wd.svg": "<svg/>",
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for key, want := range map[string]string{
		keyColorScheme: "'prefer-dark'",
		keyAccentColor: "'blue'",
		"/org/gnome/desktop/interface/icon-theme":          "'Yaru-blue'",
		"/org/gnome/desktop/interface/cursor-theme":        "'Adwaita'",
		"/org/gnome/desktop/interface/monospace-font-name": "'JetBrains Mono 11'",
		"/org/gnome/desktop/background/picture-uri":        "'file:///usr/share/deskos/backgrounds/ws.svg'",
		"/org/gnome/desktop/background/picture-uri-dark":   "'file:///usr/share/deskos/backgrounds/wd.svg'",
	} {
		if v, _ := dconf(p, key); v != want {
			t.Errorf("%s = %q, want %q", key, v, want)
		}
	}
}

// A Theme palette generates a GTK4/libadwaita named-color override, seeded
// through the user skeleton for every user the installer or GIS creates.
func TestThemePaletteGeneratesGTKCSS(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "Theme/t"),
		"t.yaml": theme("t", ""+
			"  mode: dark\n"+
			"  accent: \"#7aa2f7\"\n"+
			"  palette:\n"+
			"    background: \"#1a1b26\"\n"+
			"    foreground: \"#a9b1d6\"\n"),
	}).dir(t)
	out, err := containerfile.Render(mustPlan(t, "ws", dir))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range out {
		files[f.Path] = string(f.Data)
	}
	css := files["rootfs/etc/skel/.config/gtk-4.0/gtk.css"]
	for _, want := range []string{
		"--accent-bg-color: #7aa2f7;",
		"--accent-fg-color: rgb(0 0 0 / 80%);",
		"--window-bg-color: #1a1b26;",
		"--window-fg-color: #a9b1d6;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("gtk.css lacks %q:\n%s", want, css)
		}
	}
}

// A Theme and a GnomeProfile share the GNOME setting domain, so equal-layer
// conflicts are reported like any other GNOME setting.
func TestThemeConflictsWithGnomeProfileAtSameLayer(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml": workstation("ws", "a", "b"),
		"a.yaml":  profile("a", "organization", "Theme/t"),
		"b.yaml":  profile("b", "organization", "GnomeProfile/g"),
		"t.yaml":  theme("t", "  mode: dark\n"),
		"g.yaml":  gnome("g", appearance("prefer-light", "blue")),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, `composition conflict for GNOME setting "appearance.colorScheme"`)
}

func TestThemeValidation(t *testing.T) {
	load := func(spec string) error {
		dir := base.with(fixture{
			"ws.yaml": workstation("ws", "org"),
			"o.yaml":  profile("org", "organization", "Theme/t"),
			"t.yaml":  theme("t", spec),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		return err
	}
	if err := load("  mode: dark\n  accent: \"#7aa2f7\"\n"); err != nil {
		t.Fatalf("valid theme rejected: %v", err)
	}
	for name, spec := range map[string]string{
		"empty theme":      "  {}\n",
		"unknown mode":     "  mode: auto\n",
		"bad accent":       "  accent: azul\n",
		"bad palette":      "  palette:\n    background: nope\n",
		"empty palette":    "  palette: {}\n",
		"unknown field":    "  nope: x\n",
		"no sane settings": "\n",
	} {
		t.Run(name, func(t *testing.T) {
			mustFail(t, load(spec), "schema validation failed")
		})
	}
}
