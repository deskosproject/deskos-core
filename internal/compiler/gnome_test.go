package compiler_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/plan"
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

func TestNoGnomeSettingCanBeDeclaredOnAPlatformThatKeepsItsVendorDesktop(t *testing.T) {
	// A workstation that wants DeskOS's GNOME settings simply does not select the
	// profile that carries them, so anything declared here is a requirement the
	// platform cannot meet: it fails composition at any layer instead of turning
	// into a green validate (AGENTS.md invariant 9; docs/adr/0016).
	platform := strings.Replace(platformYAML,
		"    dconfDatabase: distro\n", "    keepsVendorDefaults: true\n", 1)
	for _, layer := range []string{"foundation", "organization"} {
		t.Run(layer, func(t *testing.T) {
			dir := fixture{"platform.yaml": platform}.with(fixture{
				"ws.yaml": workstation("ws", "org"),
				"o.yaml":  profile("org", layer, "GnomeProfile/g"),
				"g.yaml":  gnome("g", shellDefaults("      hotCorners: false\n")),
			}).dir(t)
			_, err := planOf(t, "ws", dir)
			mustFail(t, err,
				"shell.hotCorners is set, but Platform/test-platform keeps its vendor's desktop defaults",
				layer+" GnomeProfile/g")
		})
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
		"    keepsVendorDefaults: true\n    loginScreen: true\n    loginScreenDatabase: gdm\n", 1)
	dir := fixture{"platform.yaml": platform}.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml":    gnome("core", shellDefaults("      hotCorners: false\n")),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, "gnome.loginScreen is declared although the platform keeps its vendor's desktop defaults")
}

// The greeter reads its own database, so a login-screen key declared in a profile
// must land there and not beside the session defaults; the backend splits by path.
func TestLoginScreenFieldsReachTheGreeterDatabase(t *testing.T) {
	const (
		disableUserList = "/org/gnome/login-screen/disable-user-list"
		bannerEnable    = "/org/gnome/login-screen/banner-message-enable"
		bannerText      = "/org/gnome/login-screen/banner-message-text"
	)
	platform := strings.Replace(platformYAML, "    dconfDatabase: distro\n",
		"    dconfDatabase: distro\n    loginScreen: true\n    loginScreenDatabase: gdm\n", 1)
	for _, tc := range []struct {
		name, defaults, wantDisable, wantEnable, wantText string
	}{
		{"the user list is hidden and a banner shown", "    login:\n      showUserList: false\n      banner: 'Lab machine'\n", "true", "true", "'Lab machine'"},
		{"an explicit empty banner turns the message off", "    login:\n      banner: ''\n", "", "false", ""},
		{"showUserList true writes GNOME's key, not only its absence", "    login:\n      showUserList: true\n", "false", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := fixture{"platform.yaml": platform}.with(fixture{
				"ws.yaml":   workstation("ws", "core"),
				"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
				"g.yaml":    gnome("core", tc.defaults),
			}).dir(t)
			p := mustPlan(t, "ws", dir)
			got, ok := dconf(p, disableUserList)
			switch {
			case tc.wantDisable == "" && ok:
				t.Fatalf("disable-user-list is present as %q, want the key untouched", got)
			case tc.wantDisable != "" && !ok:
				t.Fatalf("disable-user-list is missing, want %q", tc.wantDisable)
			case tc.wantDisable != "" && got != tc.wantDisable:
				t.Fatalf("disable-user-list = %q, want %q", got, tc.wantDisable)
			}
			if tc.wantEnable != "" {
				got, ok := dconf(p, bannerEnable)
				if !ok || got != tc.wantEnable {
					t.Fatalf("banner-message-enable = %q (present %v), want %q", got, ok, tc.wantEnable)
				}
			} else if _, ok := dconf(p, bannerEnable); ok {
				t.Fatalf("banner-message-enable must be untouched")
			}
			if tc.wantText == "" {
				if _, ok := dconf(p, bannerText); ok {
					t.Fatalf("banner-message-text must be absent when the banner is empty")
				}
				return
			}
			if got, _ := dconf(p, bannerText); got != tc.wantText {
				t.Fatalf("banner-message-text = %q, want %q", got, tc.wantText)
			}
		})
	}
}

func TestLoginScreenFieldsNeedAGreeter(t *testing.T) {
	// No loginScreen: the greeter's database is unknown, so writing the key would
	// be an intention nobody reads. The platform gap is named (invariant 9).
	for _, tc := range []struct{ name, defaults, setting string }{
		{"banner", "    login:\n      banner: 'Lab machine'\n", "login.banner"},
		{"user list", "    login:\n      showUserList: false\n", "login.showUserList"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := base.with(fixture{
				"ws.yaml":   workstation("ws", "core"),
				"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
				"g.yaml":    gnome("core", tc.defaults),
			}).dir(t)
			_, err := planOf(t, "ws", dir)
			mustFail(t, err, "does not provide a GDM login screen that reads the DeskOS dconf database", tc.setting)
		})
	}
}

func TestIdleDimReachesThePowerPlugin(t *testing.T) {
	// The guide's "dim the screen when idle" is two keys beside the idle delay, and
	// they live in gsd's power plugin, not in the session schema.
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml": gnome("core", "    session:\n      idle:\n        blankAfter: 5m\n"+
			"        dim: true\n        dimBrightness: 30\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for key, want := range map[string]string{
		"/org/gnome/desktop/session/idle-delay":                    "uint32 300",
		"/org/gnome/settings-daemon/plugins/power/idle-dim":        "true",
		"/org/gnome/settings-daemon/plugins/power/idle-brightness": "30",
	} {
		if got, ok := dconf(p, key); !ok || got != want {
			t.Errorf("%s = %q (present %v), want %q", key, got, ok, want)
		}
	}
}

func TestDimBrightnessIsAPercentage(t *testing.T) {
	for _, value := range []string{"0", "101"} {
		dir := base.with(fixture{
			"ws.yaml":   workstation("ws", "core"),
			"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
			"g.yaml":    gnome("core", "    session:\n      idle:\n        dimBrightness: "+value+"\n"),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, "schema validation failed", "dimBrightness")
	}
}

func TestDevelopmentToolsReachTheShell(t *testing.T) {
	// The guide's "lock down enabled extensions" also disables Looking Glass; the
	// key is org.gnome.shell development-tools, false meaning no inspector.
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml":    gnome("core", "    shell:\n      developmentTools: false\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if got, ok := dconf(p, "/org/gnome/shell/development-tools"); !ok || got != "false" {
		t.Fatalf("development-tools = %q (present %v), want false", got, ok)
	}
}

func TestLockShieldCanDifferFromTheWallpaper(t *testing.T) {
	// The guide changes the lock screen shield independently of the wallpaper. The
	// wallpaper also writes the shield key, so a profile that names its own must win
	// and the key must be written once.
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml": gnome("core", "    appearance:\n      wallpaper:\n        light: bg.svg\n"+
			"      lockShield: shield.svg\n"),
		"bg.svg":     "<svg/>",
		"shield.svg": "<svg/>",
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if got, _ := dconf(p, "/org/gnome/desktop/background/picture-uri"); got != "'file:///usr/share/deskos/backgrounds/bg.svg'" {
		t.Errorf("wallpaper = %q", got)
	}
	if got, _ := dconf(p, "/org/gnome/desktop/screensaver/picture-uri"); got != "'file:///usr/share/deskos/backgrounds/shield.svg'" {
		t.Errorf("shield = %q, want the lockShield asset", got)
	}
	if n := len(p.Artifact.Dconf.Defaults); n == 0 {
		t.Fatal("no dconf defaults")
	}
	var seen int
	for _, d := range p.Artifact.Dconf.Defaults {
		if d.Key == "/org/gnome/desktop/screensaver/picture-uri" {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("screensaver picture-uri written %d times, want 1", seen)
	}
}

func TestFontRenderingWritesFontconfigAndXsettings(t *testing.T) {
	// GTK on Wayland reads fontconfig, X11 applications read Xsettings: both get
	// the same decision, and fontconfig gets a generated file rather than a shell.
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml": gnome("core", "    appearance:\n      fonts:\n        antialiasing: rgba\n"+
			"        rgbaOrder: bgr\n        hinting: slight\n        lcdfilter: light\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for key, want := range map[string]string{
		// GNOME 47 reads these, and ignores them unless font-rendering is manual.
		"/org/gnome/desktop/interface/font-rendering":    "'manual'",
		"/org/gnome/desktop/interface/font-antialiasing": "'rgba'",
		"/org/gnome/desktop/interface/font-hinting":      "'slight'",
		"/org/gnome/desktop/interface/font-rgba-order":   "'bgr'",
		// X11 applications read these through the settings daemon.
		"/org/gnome/settings-daemon/plugins/xsettings/antialiasing": "'rgba'",
		"/org/gnome/settings-daemon/plugins/xsettings/hinting":      "'slight'",
		"/org/gnome/settings-daemon/plugins/xsettings/rgba-order":   "'bgr'",
	} {
		if got, ok := dconf(p, key); !ok || got != want {
			t.Errorf("%s = %q (present %v), want %q", key, got, ok, want)
		}
	}
	var content string
	for _, f := range p.Artifact.GeneratedFiles {
		if f.Path == "/etc/fonts/conf.d/60-deskos-fonts.conf" {
			content = f.Content
		}
	}
	if content == "" {
		t.Fatal("no fontconfig file in the plan")
	}
	for _, want := range []string{
		`<edit name="antialias" mode="assign"><bool>true</bool></edit>`,
		`<edit name="rgba" mode="assign"><const>bgr</const></edit>`,
		`<edit name="hintstyle" mode="assign"><const>hintslight</const></edit>`,
		`<edit name="lcdfilter" mode="assign"><const>lcdlight</const></edit>`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("fontconfig file lacks %s:\n%s", want, content)
		}
	}
}

func TestFontRenderingWritesOnlyWhatIsDeclared(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml":    gnome("core", "    appearance:\n      fonts:\n        hinting: full\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for _, key := range []string{
		"/org/gnome/settings-daemon/plugins/xsettings/antialiasing",
		"/org/gnome/settings-daemon/plugins/xsettings/rgba-order",
	} {
		if _, ok := dconf(p, key); ok {
			t.Errorf("%s was written although only hinting was declared", key)
		}
	}
	for _, f := range p.Artifact.GeneratedFiles {
		if f.Path != "/etc/fonts/conf.d/60-deskos-fonts.conf" {
			continue
		}
		if strings.Contains(f.Content, "antialias") || strings.Contains(f.Content, "rgba") ||
			strings.Contains(f.Content, "lcdfilter") {
			t.Errorf("fontconfig file writes undeclared settings:\n%s", f.Content)
		}
		if !strings.Contains(f.Content, "hintfull") {
			t.Errorf("fontconfig file lacks the hinting it was given:\n%s", f.Content)
		}
	}
}

func TestSearchAndNotificationsLower(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml": gnome("core", "    search:\n      external: false\n"+
			"      disabled: [org.gnome.Nautilus.desktop, org.gnome.clocks.desktop]\n"+
			"    notifications:\n      banners: true\n      onLockScreen: false\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for key, want := range map[string]string{
		"/org/gnome/desktop/search-providers/disable-external": "false",
		"/org/gnome/desktop/search-providers/disabled":         "['org.gnome.Nautilus.desktop', 'org.gnome.clocks.desktop']",
		"/org/gnome/desktop/notifications/show-banners":        "true",
		"/org/gnome/desktop/notifications/show-in-lock-screen": "false",
	} {
		if got, ok := dconf(p, key); !ok || got != want {
			t.Errorf("%s = %q (present %v), want %q", key, got, ok, want)
		}
	}
}

func TestSearchProviderIdsAreChecked(t *testing.T) {
	load := func(disabled string) error {
		dir := base.with(fixture{
			"ws.yaml":   workstation("ws", "core"),
			"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
			"g.yaml":    gnome("core", "    search:\n      disabled: "+disabled+"\n"),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		return err
	}
	if err := load("[org.gnome.Nautilus.desktop]"); err != nil {
		t.Fatalf("a provider id is valid: %v", err)
	}
	// The schema rejects the shape first; the typed decoder repeats the rules so a
	// schema regression cannot weaken them.
	mustFail(t, load("[Nautilus]"), "does not match pattern")
	mustFail(t, load("[org.gnome.Nautilus.desktop, org.gnome.Nautilus.desktop]"), "are equal")
}

func TestIdleActionLowersToThePowerPlugin(t *testing.T) {
	// A timeout and its action are one policy; GNOME keeps them in two keys, the
	// type as a string and the timeout as plain integer seconds.
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml": gnome("core", "    session:\n      idle:\n        action: suspend\n"+
			"        after: 30m\n        batteryAction: logout\n        batteryAfter: 15m\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for key, want := range map[string]string{
		"/org/gnome/settings-daemon/plugins/power/sleep-inactive-ac-type":         "'suspend'",
		"/org/gnome/settings-daemon/plugins/power/sleep-inactive-ac-timeout":      "1800",
		"/org/gnome/settings-daemon/plugins/power/sleep-inactive-battery-type":    "'logout'",
		"/org/gnome/settings-daemon/plugins/power/sleep-inactive-battery-timeout": "900",
	} {
		if got, ok := dconf(p, key); !ok || got != want {
			t.Errorf("%s = %q (present %v), want %q", key, got, ok, want)
		}
	}
}

func TestIdleActionIsChecked(t *testing.T) {
	load := func(idle string) error {
		dir := base.with(fixture{
			"ws.yaml":   workstation("ws", "core"),
			"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
			"g.yaml":    gnome("core", "    session:\n      idle:\n"+idle),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		return err
	}
	if err := load("        action: suspend\n        after: 30m\n"); err != nil {
		t.Fatalf("a pair is valid: %v", err)
	}
	if err := load("        action: nothing\n        after: never\n"); err != nil {
		t.Fatalf("nothing forever is a valid pair: %v", err)
	}
	mustFail(t, load("        action: suspend\n"), "declared together")
	mustFail(t, load("        after: 30m\n"), "declared together")
	mustFail(t, load("        action: explode\n        after: 30m\n"), "must be one of")
	mustFail(t, load("        action: suspend\n        after: never\n"), "needs a timeout")
}

func TestProxyLowersToTheSystemProxy(t *testing.T) {
	// A corporate proxy is one decision with several keys. Declaring a server for
	// a protocol other than http has to turn use-same-proxy off, because GNOME
	// otherwise ignores that server entirely.
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml": gnome("core", "    network:\n      proxy:\n        mode: manual\n"+
			"        ignoreHosts: [localhost, '*.example.com']\n"+
			"        http: {host: proxy.example.com, port: 3128}\n"+
			"        https: {host: proxy.example.com, port: 3129}\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for key, want := range map[string]string{
		"/org/gnome/system/proxy/mode":           "'manual'",
		"/org/gnome/system/proxy/ignore-hosts":   "['localhost', '*.example.com']",
		"/org/gnome/system/proxy/http/host":      "'proxy.example.com'",
		"/org/gnome/system/proxy/http/port":      "3128",
		"/org/gnome/system/proxy/https/host":     "'proxy.example.com'",
		"/org/gnome/system/proxy/https/port":     "3129",
		"/org/gnome/system/proxy/use-same-proxy": "false",
	} {
		if got, ok := dconf(p, key); !ok || got != want {
			t.Errorf("%s = %q (present %v), want %q", key, got, ok, want)
		}
	}
}

func TestProxyIsChecked(t *testing.T) {
	load := func(proxy string) error {
		dir := base.with(fixture{
			"ws.yaml":   workstation("ws", "core"),
			"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
			"g.yaml":    gnome("core", "    network:\n      proxy:\n"+proxy),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		return err
	}
	if err := load("        mode: none\n"); err != nil {
		t.Fatalf("mode none is valid: %v", err)
	}
	if err := load("        mode: auto\n        autoconfigUrl: http://wpad.example.com/proxy.pac\n"); err != nil {
		t.Fatalf("mode auto with a PAC file is valid: %v", err)
	}
	mustFail(t, load("        mode: auto\n"), "needs autoconfigUrl")
	mustFail(t, load("        mode: none\n        autoconfigUrl: http://wpad/proxy.pac\n"), "only read when mode is auto")
	mustFail(t, load("        mode: manual\n"), "at least one proxy server")
	mustFail(t, load("        mode: none\n        http: {host: proxy, port: 3128}\n"), "only read when mode is manual")
	// The schema rejects the shape first; the typed decoder repeats the rules so a
	// schema regression cannot weaken them.
	mustFail(t, load("        mode: manual\n        http: {host: proxy, port: 0}\n"), "minimum")
	mustFail(t, load("        mode: manual\n        http: {host: 'http://proxy', port: 3128}\n"), "does not match pattern")
	mustFail(t, load("        mode: manual\n        ignoreHosts: ['two words']\n        http: {host: proxy, port: 3128}\n"), "does not match pattern")
}

func TestPrivacyLowersToThePrivacySchema(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml": gnome("core", "    privacy:\n      rememberRecentFiles: false\n"+
			"      recentFilesMaxAge: 7\n      removeOldTempFiles: true\n"+
			"      oldFilesAge: 14\n      reportTechnicalProblems: false\n"+
			"      disableMicrophone: true\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for key, want := range map[string]string{
		"/org/gnome/desktop/privacy/remember-recent-files":     "false",
		"/org/gnome/desktop/privacy/recent-files-max-age":      "7",
		"/org/gnome/desktop/privacy/remove-old-temp-files":     "true",
		"/org/gnome/desktop/privacy/old-files-age":             "uint32 14",
		"/org/gnome/desktop/privacy/report-technical-problems": "false",
		"/org/gnome/desktop/privacy/disable-microphone":        "true",
	} {
		if got, ok := dconf(p, key); !ok || got != want {
			t.Errorf("%s = %q (present %v), want %q", key, got, ok, want)
		}
	}
}

func TestKeyboardLayoutsLowerToInputSources(t *testing.T) {
	// GNOME stores one (type, id) pair per layout, in switch order, and the first
	// is the one a session starts in. The variant is part of the id, not a
	// separate key, so `es` with `nodeadkeys` is one entry.
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml": gnome("core", "    keyboard:\n      layouts:\n"+
			"        - {layout: us}\n        - {layout: es, variant: nodeadkeys}\n"+
			"      xkbOptions: [grp:alt_shift_toggle]\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	got, ok := dconf(p, "/org/gnome/desktop/input-sources/sources")
	if !ok || got != "[('xkb', 'us'), ('xkb', 'es+nodeadkeys')]" {
		t.Errorf("sources = %q (present %v), want the two layouts in order", got, ok)
	}
	for _, d := range p.Artifact.Dconf.Defaults {
		if d.Key == "/org/gnome/desktop/input-sources/sources" && d.Type != "a(ss)" {
			t.Errorf("sources has type %q, want a(ss)", d.Type)
		}
	}
	if opts, _ := dconf(p, "/org/gnome/desktop/input-sources/xkb-options"); opts != "['grp:alt_shift_toggle']" {
		t.Errorf("xkb-options = %q, want the option the profile declared", opts)
	}
}

func TestKeyboardLayoutsAreChecked(t *testing.T) {
	load := func(layouts string) error {
		dir := base.with(fixture{
			"ws.yaml":   workstation("ws", "core"),
			"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
			"g.yaml":    gnome("core", "    keyboard:\n      layouts: "+layouts+"\n"),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		return err
	}
	if err := load("[{layout: us}]"); err != nil {
		t.Fatalf("a single layout is valid: %v", err)
	}
	// The schema rejects the shape first; the typed decoder repeats the rules so a
	// schema regression cannot weaken them, and it is the one that sees a duplicate.
	mustFail(t, load("[]"), "minItems")
	mustFail(t, load("[{layout: US}]"), "does not match pattern")
	mustFail(t, load("[{layout: us, variant: Nodeadkeys}]"), "does not match pattern")
	mustFail(t, load("[{layout: us}, {layout: us}]"), "listed twice")
}

func TestEveryFontSurfaceGNOMEdraws(t *testing.T) {
	// GNOME has one font key per surface, plus a multiplier over all of them, and
	// the terminal keeps its own because it is the platform's packaged app. A
	// workstation that names them gets each key, not just the interface one.
	dir := base.with(fixture{
		"ws.yaml":   workstation("ws", "core"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml": gnome("core", "    appearance:\n      fonts:\n"+
			"        interface: {family: Noto Sans, size: 11}\n"+
			"        document: {family: Noto Serif, size: 12}\n"+
			"        monospace: {family: Noto Sans Mono, size: 10}\n"+
			"        titlebar: {family: Noto Sans, size: 11}\n"+
			"        terminal: {family: JetBrains Mono, size: 11}\n"+
			"        textScaling: 1.25\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for key, want := range map[string]string{
		"/org/gnome/desktop/interface/font-name":                      `'Noto Sans 11'`,
		"/org/gnome/desktop/interface/document-font-name":             `'Noto Serif 12'`,
		"/org/gnome/desktop/interface/monospace-font-name":            `'Noto Sans Mono 10'`,
		"/org/gnome/desktop/wm/preferences/titlebar-font":             `'Noto Sans 11'`,
		"/org/gnome/desktop/wm/preferences/titlebar-uses-system-font": "false",
		"/org/gnome/Ptyxis/font-name":                                 `'JetBrains Mono 11'`,
		"/org/gnome/Ptyxis/use-system-font":                           "false",
		"/org/gnome/desktop/interface/text-scaling-factor":            "1.25",
	} {
		if got, ok := dconf(p, key); !ok || got != want {
			t.Errorf("%s = %q (present %v), want %q", key, got, ok, want)
		}
	}
	if !slices.Contains(packages(p), "ptyxis") {
		t.Error("the terminal the font is set on is not installed")
	}
}

func TestATerminalFontNeedsAPlatformThatShipsOne(t *testing.T) {
	// The font lives on the platform's terminal, so a platform that declares none
	// cannot carry it: composition says which gap, rather than writing a key that
	// nothing reads (docs/adr/0025).
	platform := strings.Replace(platformYAML,
		"    terminal: {package: ptyxis, dconfPath: /org/gnome/Ptyxis/, fontKey: font-name, useSystemFontKey: use-system-font}\n", "", 1)
	if platform == platformYAML {
		t.Fatal("the fixture no longer declares a terminal, so this test proves nothing")
	}
	dir := base.with(fixture{
		"platform.yaml": platform,
		"ws.yaml":       workstation("ws", "core"),
		"core.yaml":     profile("core", "foundation", "GnomeProfile/core"),
		"g.yaml":        gnome("core", "    appearance:\n      fonts:\n        terminal: {family: JetBrains Mono, size: 11}\n"),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, "declares no terminal", "appearance.fonts.terminal")
}

func TestEveryDeclaredSettingCanBeLocked(t *testing.T) {
	// The lock list is a schema enum: a setting that lowers but is missing from it
	// cannot be locked, which is how the two greeter fields shipped without one.
	// Every key the composed plan carries must therefore be lockable.
	for _, tc := range []struct{ setting, yaml string }{
		{"session.idle.dim", "    session:\n      idle:\n        dim: true\n"},
		{"login.banner", "    login:\n      banner: 'Lab machine'\n"},
		{"login.showUserList", "    login:\n      showUserList: false\n"},
		{"shell.developmentTools", "    shell:\n      developmentTools: false\n"},
		{"appearance.lockShield", "    appearance:\n      lockShield: shield.svg\n"},
		{"appearance.fonts.hinting", "    appearance:\n      fonts:\n        hinting: slight\n"},
		{"appearance.fonts.titlebar", "    appearance:\n      fonts:\n        titlebar: {family: Noto Sans, size: 11}\n"},
		{"appearance.fonts.terminal", "    appearance:\n      fonts:\n        terminal: {family: JetBrains Mono, size: 11}\n"},
		{"appearance.fonts.textScaling", "    appearance:\n      fonts:\n        textScaling: 1.25\n"},
		{"keyboard.layouts", "    keyboard:\n      layouts: [{layout: us}]\n"},
		{"network.proxy.mode", "    network:\n      proxy:\n        mode: none\n"},
		{"session.idle.action", "    session:\n      idle:\n        action: suspend\n        after: 30m\n"},
		{"search.external", "    search:\n      external: false\n"},
		{"notifications.onLockScreen", "    notifications:\n      onLockScreen: false\n"},
		{"privacy.rememberRecentFiles", "    privacy:\n      rememberRecentFiles: false\n"},
	} {
		t.Run(tc.setting, func(t *testing.T) {
			platform := strings.Replace(platformYAML, "    dconfDatabase: distro\n",
				"    dconfDatabase: distro\n    loginScreen: true\n    loginScreenDatabase: gdm\n", 1)
			profileYAML := strings.Replace(gnome("core", tc.yaml), "spec:\n",
				"spec:\n  locks: ["+tc.setting+"]\n", 1)
			dir := fixture{"platform.yaml": platform}.with(fixture{
				"ws.yaml":    workstation("ws", "core"),
				"core.yaml":  profile("core", "foundation", "GnomeProfile/core"),
				"g.yaml":     profileYAML,
				"shield.svg": "<svg/>",
			}).dir(t)
			p := mustPlan(t, "ws", dir)
			if p.Artifact.Dconf == nil || len(p.Artifact.Dconf.Locks) == 0 {
				t.Fatalf("%s is declared and lockable but produced no lock", tc.setting)
			}
		})
	}
}

const (
	enabledExtensionsKey = "/org/gnome/shell/enabled-extensions"
	appIndicatorUUID     = "appindicatorsupport@rgcjonas.gmail.com"
	dashToDockUUID       = "dash-to-dock@micxgx.gmail.com"
	backgroundLogoUUID   = "background-logo@fedorahosted.org"
)

// platformWithAppIndicator adds a second extension to the test platform's catalog.
func platformWithAppIndicator() string {
	return strings.Replace(platformYAML,
		"      - {name: dash-to-dock, package: gnome-shell-extension-dash-to-dock, uuid: dash-to-dock@micxgx.gmail.com}",
		"      - {name: dash-to-dock, package: gnome-shell-extension-dash-to-dock, uuid: dash-to-dock@micxgx.gmail.com}\n"+
			"      - {name: appindicator, package: gnome-shell-extension-appindicator, uuid: "+appIndicatorUUID+"}",
		1)
}

func extensionDecision(uuid string, enabled bool) string {
	state := "true"
	if !enabled {
		state = "false"
	}
	return "      extensions:\n        - {uuid: " + uuid + ", enabled: " + state + "}\n"
}

func extensionPlan(t *testing.T, layer, defaults string) *plan.Plan {
	t.Helper()
	dir := fixture{"platform.yaml": platformWithAppIndicator()}.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", layer, "GnomeProfile/g"),
		"g.yaml":  gnome("g", shellDefaults(defaults)),
	}).dir(t)
	return mustPlan(t, "ws", dir)
}

func TestAProfileEnablesAnExtensionThePlatformDeclares(t *testing.T) {
	// The platform offers it, a profile turns it on, and its package comes from the
	// platform's catalog (docs/adr/0017).
	p := extensionPlan(t, "organization", extensionDecision(appIndicatorUUID, true))
	want := "['" + backgroundLogoUUID + "', '" + appIndicatorUUID + "']"
	if v, _ := dconf(p, enabledExtensionsKey); v != want {
		t.Errorf("enabled-extensions = %s, want %s", v, want)
	}
	if !strings.Contains(strings.Join(packages(p), " "), "gnome-shell-extension-appindicator") {
		t.Errorf("packages lack the extension the profile enabled: %v", packages(p))
	}
}

func TestAProfileDisablesAnExtensionThePlatformEnables(t *testing.T) {
	// A UUID the platform enables by default needs no catalog entry to be turned
	// off: there is no package to install, and the list is rewritten without it.
	p := extensionPlan(t, "organization", extensionDecision(backgroundLogoUUID, false))
	// dconf prints an empty array with its type annotation, which the session check
	// knows how to compare with the plan's `[]`.
	if v, _ := dconf(p, enabledExtensionsKey); v != "@as []" {
		t.Errorf("enabled-extensions = %s, want @as []", v)
	}
}

func TestAnExtensionDecidedTheSameWayTwiceInOneLayerFails(t *testing.T) {
	// One UUID, two decisions, no order to break the tie (docs/adr/0017).
	dir := fixture{"platform.yaml": platformWithAppIndicator()}.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml": gnome("g", shellDefaults("      extensions:\n"+
			"        - {uuid: "+appIndicatorUUID+", enabled: true}\n"+
			"        - {uuid: "+appIndicatorUUID+", enabled: false}\n")),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, "shell.extensions: \""+appIndicatorUUID+"\" is decided twice")
}

func TestAnyExtensionCanBeEnabledEvenIfThePlatformDoesNotDeclareIt(t *testing.T) {
	// Enabling is open: the platform's catalog installs the package when it knows the
	// extension, and an extension the composition brings itself is installed by the
	// packages the composition declares and verified in the session (docs/adr/0017).
	p := extensionPlan(t, "organization", extensionDecision("elsewhere@example.com", true))
	if v, _ := dconf(p, enabledExtensionsKey); !strings.Contains(v, "elsewhere@example.com") {
		t.Errorf("enabled-extensions = %s, want the extension the profile brought", v)
	}
	if strings.Contains(strings.Join(packages(p), " "), "elsewhere") {
		t.Errorf("no package may be invented for an unknown extension: %v", packages(p))
	}
}

func TestThePlatformsOwnListIsAlwaysStated(t *testing.T) {
	// The artifact says which extensions it enables, rather than leaning on whichever
	// default the vendor shipped: the session check verifies that list, so a decision
	// that changes nothing still lands in the plan.
	p := extensionPlan(t, "organization", extensionDecision(backgroundLogoUUID, true))
	if v, ok := dconf(p, enabledExtensionsKey); !ok || v != "['"+backgroundLogoUUID+"']" {
		t.Errorf("enabled-extensions = %q, want the platform's own list", v)
	}
}

func TestThePowerButtonIsAScalarLikeEveryOtherOption(t *testing.T) {
	dir := fixture{"platform.yaml": platformYAML}.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml":  gnome("g", "    power:\n      buttonAction: hibernate\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if v, _ := dconf(p, "/org/gnome/settings-daemon/plugins/power/power-button-action"); v != "'hibernate'" {
		t.Errorf("power-button-action = %s", v)
	}
}

func TestAnUnknownPowerButtonActionIsRejected(t *testing.T) {
	dir := fixture{"platform.yaml": platformYAML}.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml":  gnome("g", "    power:\n      buttonAction: reboot\n"),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, "at /spec/defaults/power/buttonAction: value must be one of 'nothing', 'suspend', 'hibernate', 'interactive'")
}
