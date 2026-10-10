package compiler_test

import (
	"testing"
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
		"unknown field":    "  palette: {}\n",
		"no sane settings": "\n",
	} {
		t.Run(name, func(t *testing.T) {
			mustFail(t, load(spec), "schema validation failed")
		})
	}
}
