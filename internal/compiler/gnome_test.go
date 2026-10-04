package compiler_test

import (
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
)

const (
	keyColorScheme     = "/org/gnome/desktop/interface/color-scheme"
	keyAccentColor     = "/org/gnome/desktop/interface/accent-color"
	keyAllowUpdates    = "/org/gnome/software/allow-updates"
	keyDownloadUpdates = "/org/gnome/software/download-updates"
)

func appearance(colorScheme, accentColor string) string {
	return "    appearance:\n      colorScheme: " + colorScheme + "\n      accentColor: " + accentColor + "\n"
}

func softwareUpdates(mode string) string {
	return "    software:\n      updates: " + mode + "\n"
}

func TestGnomeAppearanceAndSoftwareValidation(t *testing.T) {
	load := func(defaults string) error {
		_, err := newCompiler(t).Load([]string{fixture{"g.yaml": gnome("g", defaults)}.dir(t)})
		return err
	}
	if err := load(appearance("prefer-dark", "slate") + softwareUpdates("manual")); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	for name, defaults := range map[string]string{
		"unknown color scheme": appearance("dark", "blue"),
		"unknown accent color": appearance("default", "magenta"),
		"unknown update mode":  softwareUpdates("weekly"),
		"boolean update mode":  softwareUpdates("false"),
		"unknown software key": "    software:\n      updates: manual\n      command: gnome-software --quit\n",
	} {
		t.Run(name, func(t *testing.T) {
			mustFail(t, load(defaults), "schema validation failed")
		})
	}
}

// The typed decoder enforces the same rules as the schema on its own.
func TestGnomeAppearanceAndSoftwareTypedValidationWithoutSchema(t *testing.T) {
	c := newCompiler(t)
	for name, tc := range map[string]struct{ spec, want string }{
		"color scheme":     {`{"defaults":{"appearance":{"colorScheme":"dark"}}}`, `appearance.colorScheme: expected default, prefer-dark, prefer-light, got "dark"`},
		"accent color":     {`{"defaults":{"appearance":{"accentColor":"magenta"}}}`, `appearance.accentColor: expected blue, teal, green, yellow, orange, red, pink, purple, slate, got "magenta"`},
		"update mode":      {`{"defaults":{"software":{"updates":"weekly"}}}`, `software.updates: expected automatic, manual, disabled, got "weekly"`},
		"unknown software": {`{"defaults":{"software":{"updates":"manual","dconf":{}}}}`, "unknown field"},
	} {
		t.Run(name, func(t *testing.T) {
			res := rawResource("desktop.deskos.org/v1alpha1", "GnomeProfile", "x", tc.spec)
			p, err := c.Registry.Lookup(res.GVK())
			if err != nil {
				t.Fatal(err)
			}
			mustFail(t, p.Decode(res), tc.want)
		})
	}
}

func TestSoftwareUpdatesLowering(t *testing.T) {
	for mode, want := range map[string][2]string{
		"automatic": {"true", "true"},
		"manual":    {"true", "false"},
		"disabled":  {"false", "false"},
	} {
		t.Run(mode, func(t *testing.T) {
			dir := base.with(fixture{
				"ws.yaml": workstation("ws", "org"),
				"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
				"g.yaml":  gnome("g", softwareUpdates(mode)),
			}).dir(t)
			p := mustPlan(t, "ws", dir)
			allow, _ := dconf(p, keyAllowUpdates)
			download, _ := dconf(p, keyDownloadUpdates)
			if allow != want[0] || download != want[1] {
				t.Errorf("allow-updates=%s download-updates=%s, want %s %s", allow, download, want[0], want[1])
			}
		})
	}
}

func TestAppearanceAndSoftwareComposition(t *testing.T) {
	override := base.with(fixture{
		"ws.yaml":   workstation("ws", "core", "org"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"org.yaml":  profile("org", "organization", "GnomeProfile/org"),
		"g1.yaml":   gnome("core", appearance("default", "blue")+softwareUpdates("manual")),
		"g2.yaml":   gnome("org", appearance("prefer-dark", "teal")+softwareUpdates("disabled")),
	}).dir(t)
	p := mustPlan(t, "ws", override)
	for key, want := range map[string]string{
		keyColorScheme:     "'prefer-dark'",
		keyAccentColor:     "'teal'",
		keyAllowUpdates:    "false",
		keyDownloadUpdates: "false",
	} {
		if v, _ := dconf(p, key); v != want {
			t.Errorf("%s = %q, want %q", key, v, want)
		}
	}

	conflict := base.with(fixture{
		"ws.yaml": workstation("ws", "a", "b"),
		"a.yaml":  profile("a", "organization", "GnomeProfile/one"),
		"b.yaml":  profile("b", "organization", "GnomeProfile/two"),
		"g1.yaml": gnome("one", softwareUpdates("manual")),
		"g2.yaml": gnome("two", softwareUpdates("automatic")),
	}).dir(t)
	_, err := planOf(t, "ws", conflict)
	mustFail(t, err, `composition conflict for GNOME setting "software.updates"`,
		"organization GnomeProfile/one", "value: manual",
		"organization GnomeProfile/two", "value: automatic")
}

func TestAppearanceAndSoftwareRenderAndLocks(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml": gnome("g", appearance("prefer-light", "purple")+softwareUpdates("manual")+
			"  locks:\n    - software.updates\n    - appearance.accentColor\n"),
	}).dir(t)
	out, err := containerfile.Render(mustPlan(t, "ws", dir))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range out {
		files[f.Path] = string(f.Data)
	}
	keyfile := files["rootfs/etc/dconf/db/distro.d/50-deskos"]
	for _, want := range []string{
		"[org/gnome/desktop/interface]\naccent-color='purple'\ncolor-scheme='prefer-light'\n",
		"[org/gnome/software]\nallow-updates=true\ndownload-updates=false\n",
	} {
		if !strings.Contains(keyfile, want) {
			t.Errorf("dconf keyfile lacks %q:\n%s", want, keyfile)
		}
	}
	const locks = "# Generated by deskosctl. Do not edit; change the DeskOS resources instead.\n" +
		keyAccentColor + "\n" + keyAllowUpdates + "\n" + keyDownloadUpdates + "\n"
	if got := files["rootfs/etc/dconf/db/distro.d/locks/50-deskos"]; got != locks {
		t.Errorf("dconf locks =\n%s\nwant\n%s", got, locks)
	}
}
