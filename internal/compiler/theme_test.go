package compiler_test

import (
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/plan"
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
	gtk3 := files["rootfs/etc/skel/.config/gtk-3.0/gtk.css"]
	for _, want := range []string{
		"@define-color accent_bg_color #7aa2f7;",
		"@define-color theme_bg_color #1a1b26;",
		"@define-color theme_fg_color #a9b1d6;",
	} {
		if !strings.Contains(gtk3, want) {
			t.Errorf("gtk-3.0/gtk.css lacks %q:\n%s", want, gtk3)
		}
	}
	// Without the normal ANSI colors there is no terminal palette.
	if _, ok := files["rootfs/etc/skel/.local/share/org.gnome.Ptyxis/palettes/t.palette"]; ok {
		t.Error("a palette without ANSI colors must not generate a terminal palette")
	}
}

// A palette that carries the normal ANSI colors also seeds a Ptyxis terminal
// palette in the user skeleton.
func TestThemePaletteGeneratesTerminalPalette(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "Theme/t"),
		"t.yaml": theme("t", ""+
			"  mode: dark\n"+
			"  accent: \"#b8bb26\"\n"+
			"  palette:\n"+
			"    background: \"#282828\"\n"+
			"    foreground: \"#ebdbb2\"\n"+
			"    red: \"#fb4934\"\n"+
			"    green: \"#b8bb26\"\n"+
			"    yellow: \"#fabd2f\"\n"+
			"    blue: \"#83a598\"\n"+
			"    magenta: \"#d3869b\"\n"+
			"    cyan: \"#8ec07c\"\n"),
	}).dir(t)
	out, err := containerfile.Render(mustPlan(t, "ws", dir))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range out {
		files[f.Path] = string(f.Data)
	}
	pal := files["rootfs/etc/skel/.local/share/org.gnome.Ptyxis/palettes/t.palette"]
	for _, want := range []string{
		"Name=t", "Primary=true",
		"Foreground=#ebdbb2", "Background=#282828",
		"Color1=#fb4934", "Color15=#ebdbb2",
	} {
		if !strings.Contains(pal, want) {
			t.Errorf("terminal palette lacks %q:\n%s", want, pal)
		}
	}
	prof := files["rootfs/etc/dconf/db/distro.d/60-deskos-ptyxis"]
	for _, want := range []string{
		"default-profile-uuid='9a1f0f9a-6f2b-4a0e-8e0b-0d9f4a1c2b30'",
		"[org/gnome/Ptyxis/Profiles/9a1f0f9a-6f2b-4a0e-8e0b-0d9f4a1c2b30]",
		"palette='t'",
	} {
		if !strings.Contains(prof, want) {
			t.Errorf("ptyxis profile lacks %q:\n%s", want, prof)
		}
	}
}

// The first-run welcome dialog (the tour prompt) is optional and on by
// default; turning it off writes the shell's last-shown version.
func TestWelcomeTourOptional(t *testing.T) {
	const key = "/org/gnome/shell/welcome-dialog-last-shown-version"
	profile := func(tour string) *plan.Plan {
		dir := base.with(fixture{
			"ws.yaml": workstation("ws", "org"),
			"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
			"g.yaml":  gnome("g", "    shell:\n      welcomeTour: "+tour+"\n"),
		}).dir(t)
		return mustPlan(t, "ws", dir)
	}
	if v, _ := dconf(profile("false"), key); v != "'40.beta'" {
		t.Errorf("%s = %q, want '40.beta'", key, v)
	}
	if _, ok := dconf(profile("true"), key); ok {
		t.Errorf("welcomeTour: true must not write %s", key)
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
