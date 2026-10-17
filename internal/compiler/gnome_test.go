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

func TestAPlatformThatKeepsVendorDefaultsWritesNoDconf(t *testing.T) {
	// A platform whose base ships no system dconf profile keeps its vendor's
	// desktop defaults: DeskOS writes none, and a default a profile asked for is
	// recorded as not applied instead of promised to a user who never gets it
	// (AGENTS.md invariant 9; docs/adr/0016).
	platform := strings.Replace(platformYAML,
		"    dconfDatabase: distro\n", "    keepsVendorDefaults: true\n", 1)
	dir := fixture{"platform.yaml": platform}.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml":    gnome("core", shellDefaults("      hotCorners: false\n")),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if p.Artifact.Dconf != nil {
		t.Fatalf("dconf written for a platform that keeps its vendor's defaults: %+v", p.Artifact.Dconf)
	}
	want := "shell.hotCorners is not applied: Platform/test-platform declares gnome.keepsVendorDefaults"
	if !strings.Contains(strings.Join(p.Warnings, "\n"), want) {
		t.Errorf("warnings %q lack %q", p.Warnings, want)
	}
}

func TestKeepsVendorDefaultsAndADconfDatabaseConflict(t *testing.T) {
	// The two facts contradict each other: one says DeskOS writes a database, the
	// other says it writes none.
	platform := strings.Replace(platformYAML, "    dconfDatabase: distro\n",
		"    dconfDatabase: distro\n    keepsVendorDefaults: true\n", 1)
	dir := fixture{"platform.yaml": platform}.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml":    gnome("core", shellDefaults("      hotCorners: false\n")),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err,
		`gnome.dconfDatabase "distro" is declared although the platform keeps its vendor's desktop defaults`)
}

func TestGNOMEDefaultsNeedADconfDatabase(t *testing.T) {
	// The database name is optional in the schema only for a platform that keeps
	// its vendor's defaults; every other platform must name where it writes.
	platform := strings.Replace(platformYAML, "    dconfDatabase: distro\n", "", 1)
	dir := fixture{"platform.yaml": platform}.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml":    gnome("core", shellDefaults("      hotCorners: false\n")),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	if err == nil || !strings.Contains(err.Error(), "dconfDatabase") {
		t.Fatalf("a platform with neither dconfDatabase nor keepsVendorDefaults must be rejected, got %v", err)
	}
}

func TestWindowButtonsGoOnTheSideTheProfileAsksFor(t *testing.T) {
	// GNOME's button-layout is "<left>:<right>": the colon separates the window's
	// left corner from its right corner. A golden that only compares the produced
	// string does not catch a swap of the two sides, so the side is asserted here.
	const key = "/org/gnome/desktop/wm/preferences/button-layout"
	for _, tc := range []struct{ name, defaults, want string }{
		{"no placement declared puts the buttons on the right", "    windows:\n      buttons: [close]\n", "':close'"},
		{"left when the profile asks for it", "    windows:\n      buttons: [close]\n      buttonPlacement: left\n", "'close:'"},
		{"right when the profile asks for it", "    windows:\n      buttons: [close, minimize]\n      buttonPlacement: right\n", "':close,minimize'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := base.with(fixture{
				"ws.yaml":   workstation("ws", "core"),
				"core.yaml": profile("core", "foundation", "GnomeProfile/g"),
				"g.yaml":    gnome("g", tc.defaults),
			}).dir(t)
			p := mustPlan(t, "ws", dir)
			if v, ok := dconf(p, key); !ok || v != tc.want {
				t.Errorf("button-layout = %s, want %s", v, tc.want)
			}
		})
	}
}

func TestAnOrganizationCannotSetGnomeOnAPlatformThatKeepsItsVendorDesktop(t *testing.T) {
	// The foundation's own GNOME opinion is recorded as not applied on such a
	// platform, but a higher layer asking for a value or a lock is a requirement
	// the platform cannot meet, and that fails composition instead of turning into
	// a green validate (AGENTS.md invariant 9; docs/adr/0016).
	platform := strings.Replace(platformYAML,
		"    dconfDatabase: distro\n", "    keepsVendorDefaults: true\n", 1)
	dir := fixture{"platform.yaml": platform}.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml":  gnome("g", shellDefaults("      hotCorners: false\n")),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err,
		"shell.hotCorners is set at organization layer, but Platform/test-platform keeps its vendor's desktop defaults",
		"organization GnomeProfile/g")
}

func TestNoLockCanBeDeclaredOnAPlatformThatKeepsItsVendorDesktop(t *testing.T) {
	// A lock is enforcement, and Core never locks (AGENTS.md invariant 8), so a lock
	// here is always an owner's requirement that this platform cannot enforce: it
	// fails composition at any layer, rather than recording a guarantee that does not
	// exist (docs/adr/0016).
	platform := strings.Replace(platformYAML,
		"    dconfDatabase: distro\n", "    keepsVendorDefaults: true\n", 1)
	for _, layer := range []string{"foundation", "organization"} {
		t.Run(layer, func(t *testing.T) {
			gnomeWithLock := strings.Replace(gnome("g", shellDefaults("      hotCorners: false\n")),
				"spec:\n", "spec:\n  locks: [shell.hotCorners]\n", 1)
			dir := fixture{"platform.yaml": platform}.with(fixture{
				"ws.yaml": workstation("ws", "org"),
				"o.yaml":  profile("org", layer, "GnomeProfile/g"),
				"g.yaml":  gnomeWithLock,
			}).dir(t)
			_, err := planOf(t, "ws", dir)
			mustFail(t, err, "shell.hotCorners is locked, but Platform/test-platform keeps its vendor's desktop defaults")
		})
	}
}

func TestKeepsVendorDefaultsAndALoginScreenConflict(t *testing.T) {
	platform := strings.Replace(platformYAML, "    dconfDatabase: distro\n",
		"    keepsVendorDefaults: true\n    loginScreen: true\n", 1)
	dir := fixture{"platform.yaml": platform}.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml":    gnome("core", shellDefaults("      hotCorners: false\n")),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, "gnome.loginScreen is declared although the platform keeps its vendor's desktop defaults")
}
