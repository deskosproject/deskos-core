package compiler_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/plan"
)

const (
	keyHotCorners    = "/org/gnome/desktop/interface/enable-hot-corners"
	keyClockWeekday  = "/org/gnome/desktop/interface/clock-show-weekday"
	keyClockFormat   = "/org/gnome/desktop/interface/clock-format"
	keyNumLock       = "/org/gnome/desktop/peripherals/keyboard/numlock-state"
	keyDynamicWS     = "/org/gnome/mutter/dynamic-workspaces"
	keyNumWS         = "/org/gnome/desktop/wm/preferences/num-workspaces"
	keyFolders       = "/org/gnome/desktop/app-folders/folder-children"
	keyCustomKeys    = "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings"
	terminalKeysPath = "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/deskos-terminal/"
)

var launcherPlatform = fixture{"platform.yaml": strings.Replace(platformYAML,
	"  flatpak: {package: flatpak, preinstall: true}\n",
	"    appLauncher: {package: gtk3, command: /usr/bin/gtk-launch}\n  flatpak: {package: flatpak, preinstall: true}\n", 1)}

const devFolder = "      appFolders:\n" +
	"        - id: Development\n          name: Development\n          apps: [code.desktop]\n          categories: [Development, IDE]\n" +
	"        - id: Utilities\n          name: Utilities\n          apps: [com.github.tchx84.Flatseal.desktop]\n"

func shellDefaults(body string) string { return "    shell:\n" + body }

func TestGnomeShellClockKeyboardValidation(t *testing.T) {
	load := func(defaults string) error {
		_, err := newCompiler(t).Load([]string{fixture{"g.yaml": gnome("g", defaults)}.dir(t)})
		return err
	}
	valid := shellDefaults("      hotCorners: false\n      workspaces: 4\n"+devFolder) +
		"    clock:\n      showWeekday: true\n      format: 12h\n" +
		"    keyboard:\n      numLock: true\n      terminal: org.gnome.Ptyxis.desktop\n"
	if err := load(valid); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	if err := load(shellDefaults("      workspaces: dynamic\n")); err != nil {
		t.Fatalf("dynamic workspaces rejected: %v", err)
	}
	for name, defaults := range map[string]string{
		"zero workspaces":       shellDefaults("      workspaces: 0\n"),
		"too many workspaces":   shellDefaults("      workspaces: 37\n"),
		"workspace word":        shellDefaults("      workspaces: fixed\n"),
		"folder without apps":   shellDefaults("      appFolders:\n        - {id: Empty, name: Empty}\n"),
		"folder id with slash":  shellDefaults("      appFolders:\n        - {id: a/b, name: A, apps: [x.desktop]}\n"),
		"folder app not an id":  shellDefaults("      appFolders:\n        - {id: A, name: A, apps: [code]}\n"),
		"no folders":            shellDefaults("      appFolders: []\n"),
		"clock format":          "    clock:\n      format: 24-hour\n",
		"terminal not an id":    "    keyboard:\n      terminal: ghostty\n",
		"terminal with command": "    keyboard:\n      terminal: x.desktop\n      command: xterm\n",
	} {
		t.Run(name, func(t *testing.T) {
			mustFail(t, load(defaults), "schema validation failed")
		})
	}
}

// The typed decoder enforces the same rules as the schema on its own.
func TestGnomeShellTypedValidationWithoutSchema(t *testing.T) {
	c := newCompiler(t)
	for name, tc := range map[string]struct{ spec, want string }{
		"workspaces zero":   {`{"defaults":{"shell":{"workspaces":0}}}`, "shell.workspaces: expected dynamic or a number from 1 to 36"},
		"workspaces float":  {`{"defaults":{"shell":{"workspaces":2.5}}}`, "shell.workspaces: expected dynamic or a number from 1 to 36"},
		"workspaces word":   {`{"defaults":{"shell":{"workspaces":"fixed"}}}`, `shell.workspaces: expected dynamic or a number from 1 to 36, got "fixed"`},
		"folder twice":      {`{"defaults":{"shell":{"appFolders":[{"id":"A","name":"A","apps":["a.desktop"]},{"id":"A","name":"B","apps":["b.desktop"]}]}}}`, `shell.appFolders: folder "A" is listed twice`},
		"folder empty":      {`{"defaults":{"shell":{"appFolders":[{"id":"A","name":"A"}]}}}`, `shell.appFolders: folder "A" needs apps or categories`},
		"folder name":       {`{"defaults":{"shell":{"appFolders":[{"id":"A","name":"","apps":["a.desktop"]}]}}}`, `shell.appFolders: folder "A" needs a single-line name`},
		"folder no folders": {`{"defaults":{"shell":{"appFolders":[]}}}`, "shell.appFolders: at least one folder is required"},
		"clock format":      {`{"defaults":{"clock":{"format":"am/pm"}}}`, `clock.format: expected 24h or 12h, got "am/pm"`},
		"terminal":          {`{"defaults":{"keyboard":{"terminal":"ghostty"}}}`, `keyboard.terminal: "ghostty" is not a desktop file id`},
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

func TestGnomeShellLowering(t *testing.T) {
	dir := launcherPlatform.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml": gnome("g", shellDefaults("      hotCorners: false\n      workspaces: 4\n"+devFolder)+
			"    clock:\n      showWeekday: true\n      format: 24h\n"+
			"    keyboard:\n      numLock: true\n      terminal: com.mitchellh.ghostty.desktop\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	for key, want := range map[string]string{
		keyHotCorners:   "false",
		keyClockWeekday: "true",
		keyClockFormat:  "'24h'",
		keyNumLock:      "true",
		keyDynamicWS:    "false",
		keyNumWS:        "4",
		keyFolders:      "['Development', 'Utilities']",
		"/org/gnome/desktop/app-folders/folders/Development/name":       "'Development'",
		"/org/gnome/desktop/app-folders/folders/Development/translate":  "false",
		"/org/gnome/desktop/app-folders/folders/Development/apps":       "['code.desktop']",
		"/org/gnome/desktop/app-folders/folders/Development/categories": "['Development', 'IDE']",
		"/org/gnome/desktop/app-folders/folders/Utilities/categories":   "@as []",
		keyCustomKeys:                "['" + terminalKeysPath + "']",
		terminalKeysPath + "command": "'/usr/bin/gtk-launch com.mitchellh.ghostty'",
		terminalKeysPath + "binding": "'<Control><Alt>t'",
		terminalKeysPath + "name":    "'Terminal'",
	} {
		if v, ok := dconf(p, key); !ok || v != want {
			t.Errorf("%s = %q (set %v), want %q", key, v, ok, want)
		}
	}
	if !slices.ContainsFunc(p.Artifact.RpmPackages, func(r plan.RpmInstall) bool { return r.Name == "gtk3" }) {
		t.Error("keyboard.terminal does not install the platform's application launcher package gtk3")
	}

	dynamic := launcherPlatform.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml":  gnome("g", shellDefaults("      workspaces: dynamic\n")),
	}).dir(t)
	p = mustPlan(t, "ws", dynamic)
	if v, _ := dconf(p, keyDynamicWS); v != "true" {
		t.Errorf("dynamic-workspaces = %q, want true", v)
	}
	if _, ok := dconf(p, keyNumWS); ok {
		t.Error("dynamic workspaces must not set num-workspaces")
	}
}

func TestTerminalShortcutNeedsPlatformLauncher(t *testing.T) {
	dir := base.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml":  gnome("g", "    keyboard:\n      terminal: org.gnome.Ptyxis.desktop\n"),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, "Platform/test-platform declares no GNOME application launcher, which keyboard.terminal needs",
		"organization GnomeProfile/g")
}

// A higher layer replaces the whole folder list; folders do not merge.
func TestAppFoldersAndWorkspacesComposition(t *testing.T) {
	dir := launcherPlatform.with(fixture{
		"ws.yaml":   workstation("ws", "core", "org"),
		"core.yaml": profile("core", "foundation", "GnomeProfile/core"),
		"org.yaml":  profile("org", "organization", "GnomeProfile/org"),
		"g1.yaml":   gnome("core", shellDefaults("      workspaces: dynamic\n"+devFolder)),
		"g2.yaml": gnome("org", shellDefaults("      workspaces: 2\n      appFolders:\n"+
			"        - {id: Office, name: Office, categories: [Office]}\n")),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if v, _ := dconf(p, keyFolders); v != "['Office']" {
		t.Errorf("folder-children = %s, want ['Office']", v)
	}
	if _, ok := dconf(p, "/org/gnome/desktop/app-folders/folders/Development/name"); ok {
		t.Error("a folder from a lower layer survived the override")
	}
	if v, _ := dconf(p, keyNumWS); v != "2" {
		t.Errorf("num-workspaces = %s, want 2", v)
	}

	conflict := launcherPlatform.with(fixture{
		"ws.yaml": workstation("ws", "a", "b"),
		"a.yaml":  profile("a", "organization", "GnomeProfile/one"),
		"b.yaml":  profile("b", "organization", "GnomeProfile/two"),
		"g1.yaml": gnome("one", "    keyboard:\n      terminal: org.gnome.Ptyxis.desktop\n"),
		"g2.yaml": gnome("two", "    keyboard:\n      terminal: com.mitchellh.ghostty.desktop\n"),
	}).dir(t)
	_, err := planOf(t, "ws", conflict)
	mustFail(t, err, `composition conflict for GNOME setting "keyboard.terminal"`,
		"organization GnomeProfile/one", "organization GnomeProfile/two")
}

func TestGnomeShellRenderAndLocks(t *testing.T) {
	dir := launcherPlatform.with(fixture{
		"ws.yaml": workstation("ws", "org"),
		"o.yaml":  profile("org", "organization", "GnomeProfile/g"),
		"g.yaml": gnome("g", shellDefaults("      hotCorners: false\n")+
			"    keyboard:\n      terminal: org.gnome.Ptyxis.desktop\n"+
			"  locks:\n    - shell.hotCorners\n    - keyboard.terminal\n"),
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
		"[org/gnome/desktop/interface]\nenable-hot-corners=false\n",
		"[org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/deskos-terminal]\nbinding='<Control><Alt>t'\ncommand='/usr/bin/gtk-launch org.gnome.Ptyxis'\nname='Terminal'\n",
	} {
		if !strings.Contains(keyfile, want) {
			t.Errorf("dconf keyfile lacks %q:\n%s", want, keyfile)
		}
	}
	locks := files["rootfs/etc/dconf/db/distro.d/locks/50-deskos"]
	for _, want := range []string{keyHotCorners, keyCustomKeys, terminalKeysPath + "binding", terminalKeysPath + "command"} {
		if !strings.Contains(locks, want+"\n") {
			t.Errorf("dconf locks lack %s:\n%s", want, locks)
		}
	}
}
