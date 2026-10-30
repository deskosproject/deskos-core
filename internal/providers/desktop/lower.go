package desktop

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
	"github.com/deskosproject/deskos-core/internal/theme"
)

// BackgroundsDir is the image-owned location of wallpaper assets.
const BackgroundsDir = "/usr/share/deskos/backgrounds"

// BrandingDir is the image-owned location of logo assets.
const BrandingDir = "/usr/share/deskos/branding"

// gtk4UserCSSPath is read by every GTK4/libadwaita app of the session user.
// A system image seeds it through the user skeleton (/etc/skel), so users
// created by the installer or GNOME Initial Setup inherit it.
const gtk4UserCSSPath = "/etc/skel/.config/gtk-4.0/gtk.css"

// gtk3UserCSSPath is read by every GTK3 app, alongside its theme.
const gtk3UserCSSPath = "/etc/skel/.config/gtk-3.0/gtk.css"

// ptyxisPalettesDir holds Ptyxis terminal palettes for the session user. A
// system image seeds it through the user skeleton.
const ptyxisPalettesDir = "/etc/skel/.local/share/org.gnome.Ptyxis/palettes"

// dconfDropInDir is one dconf database source directory; deskosctl writes the
// profile database there and the build runs dconf update.
const dconfDropInDir = "/etc/dconf/db/distro.d"

// mimeAppsPath is the system-wide default for document handlers. An entry in a
// user's ~/.config/mimeapps.list still wins; this is a default, not a lock.
const mimeAppsPath = "/etc/xdg/mimeapps.list"

// terminalListPath is the system-wide preference list read by
// xdg-terminal-exec, including the Ctrl+Alt+T binding's command.
const terminalListPath = "/etc/xdg/xdg-terminals.list"

// ptyxisProfileUUID is the fixed UUID of the profile DeskOS provisions so a
// terminal uses the Theme palette by default. It is a system default: a user
// who picks another palette or profile overrides it.
const ptyxisProfileUUID = "9a1f0f9a-6f2b-4a0e-8e0b-0d9f4a1c2b30"

// welcomeDialogShownVersion is gnome-shell's WELCOME_DIALOG_LAST_TOUR_CHANGE.
// At or after it the shell does not show its first-run welcome dialog.
const welcomeDialogShownVersion = "40.beta"

// dockSettingsAdapter names the adapter in messages: the settings DeskOS writes for
// the dock need the platform to declare the extension they belong to, because that
// entry is what names the UUID whose decision gates them (docs/adr/0017).
const dockSettingsAdapter = "the dock settings adapter"

// DockExtension is the platform extension name that implements the dock.
const DockExtension = "dash-to-dock"

// Lowerer translates composed GNOME settings into a dconf database.
type Lowerer struct{}

func (Lowerer) Name() string { return "desktop" }

type lowering struct {
	c      *compose.Composition
	p      *plan.Plan
	db     *plan.DconfDatabase
	bySett map[string][]string
	errs   model.ErrorList
}

func (Lowerer) Lower(c *compose.Composition, p *plan.Plan) error {
	settings := c.Result.Scalars(DomainSettings)
	locks := c.Result.Set(DomainLocks)
	if len(settings) == 0 && len(locks) == 0 {
		return nil
	}
	pl := c.PlatformSpec
	if pl.Gnome == nil {
		return fmt.Errorf("%s declares no GNOME integration, but the composition contains GNOME settings", c.Platform.ID())
	}
	l := &lowering{c: c, p: p, db: &plan.DconfDatabase{
		Name:        pl.Gnome.DconfDatabase,
		LoginScreen: pl.Gnome.LoginScreenDatabase,
	}, bySett: map[string][]string{}}
	if pl.Gnome.KeepsVendorDefaults {
		// The platform's base ships no system dconf profile, so DeskOS writes no
		// default and no lock for it: its desktop is the vendor's (ADR 0016). A
		// workstation that wants DeskOS's GNOME settings simply does not select the
		// profile that carries them, so anything declared here is a requirement this
		// platform cannot meet and fails composition rather than being dropped. A
		// lock fails at any layer for the same reason, and Core never locks
		// (AGENTS.md invariant 8).
		for _, s := range settings {
			l.errs.Add(fmt.Errorf("%s is set, but %s keeps its vendor's desktop defaults and writes no dconf database\n  set by: %s",
				s.Key, c.Platform.ID(), describe(s.Provenance)))
		}
		for _, lk := range locks {
			l.errs.Add(fmt.Errorf("%s is locked, but %s keeps its vendor's desktop defaults and writes no dconf database\n  locked by: %s",
				lk.Name, c.Platform.ID(), describe(lk.Provenance)))
		}
		return l.errs.Err()
	}
	get := func(key string) (compose.Scalar, bool) { return c.Result.Scalar(DomainSettings, key) }

	// Window controls: two semantic keys produce one GNOME key. The plan writes
	// exactly the buttons the profile declares: DeskOS does not add the app menu
	// token GNOME ships by default, which no profile asked for and which decides
	// the shell's look on its own (docs/adr/0016). A flavor that wants it declares
	// it, once a field exists. GNOME's button-layout is "<left>:<right>": the colon
	// separates the window's left corner from its right corner, so the buttons go
	// after it unless the profile asks for the left side.
	if b, ok := get(KeyButtons); ok {
		buttons := strings.Join(b.Value.([]string), ",")
		layout := ":" + buttons
		prov := b.Provenance
		const key = "/org/gnome/desktop/wm/preferences/button-layout"
		if pos, ok := get(KeyButtonPlacement); ok {
			if pos.Value.(string) == "left" {
				layout = buttons + ":"
			}
			prov = append(append([]model.Provenance(nil), prov...), pos.Provenance...)
			l.bySett[KeyButtonPlacement] = []string{key}
		}
		l.set(KeyButtons, key, "s", gvString(layout), prov)
	} else if pos, ok := get(KeyButtonPlacement); ok {
		l.errs.Add(fmt.Errorf("%s is set but %s is not; window button placement needs the button list\n  set by: %s",
			KeyButtonPlacement, KeyButtons, describe(pos.Provenance)))
	}

	if f, ok := get(KeyFavorites); ok {
		l.set(KeyFavorites, "/org/gnome/shell/favorite-apps", "as", gvStrings(f.Value.([]string)), f.Provenance)
	}
	if v, ok := get(KeyDevTools); ok {
		l.set(KeyDevTools, "/org/gnome/shell/development-tools", "b",
			strconv.FormatBool(v.Value.(bool)), v.Provenance)
	}
	if v, ok := get(KeyBlankAfter); ok {
		l.set(KeyBlankAfter, "/org/gnome/desktop/session/idle-delay", "u", gvUint32(v.Value.(Seconds)), v.Provenance)
	}
	if v, ok := get(KeyIdleDim); ok {
		l.set(KeyIdleDim, "/org/gnome/settings-daemon/plugins/power/idle-dim", "b",
			strconv.FormatBool(v.Value.(bool)), v.Provenance)
	}
	if v, ok := get(KeyDimBrightness); ok {
		l.set(KeyDimBrightness, "/org/gnome/settings-daemon/plugins/power/idle-brightness", "i",
			strconv.Itoa(v.Value.(int)), v.Provenance)
	}
	if v, ok := get(KeyLockEnabled); ok {
		l.set(KeyLockEnabled, "/org/gnome/desktop/screensaver/lock-enabled", "b", strconv.FormatBool(v.Value.(bool)), v.Provenance)
	}
	if v, ok := get(KeyLockDelay); ok {
		l.set(KeyLockDelay, "/org/gnome/desktop/screensaver/lock-delay", "u", gvUint32(v.Value.(Seconds)), v.Provenance)
	}

	shield, hasShield := get(KeyLockShield)
	if wp, ok := get(KeyWallpaper); ok {
		v := wp.Value.(WallpaperValue)
		light := l.installAsset(v.Light, wp.Provenance)
		dark := light
		if v.Dark != nil {
			dark = l.installAsset(*v.Dark, wp.Provenance)
		}
		l.set(KeyWallpaper, "/org/gnome/desktop/background/picture-uri", "s", gvString(light), wp.Provenance)
		l.set(KeyWallpaper, "/org/gnome/desktop/background/picture-uri-dark", "s", gvString(dark), wp.Provenance)
		if !hasShield {
			// The shield follows the wallpaper unless a profile names its own.
			l.set(KeyWallpaper, "/org/gnome/desktop/screensaver/picture-uri", "s", gvString(light), wp.Provenance)
		}
		// GNOME Initial Setup's dconf profile skips the DeskOS database, so
		// the first-boot session gets the wallpaper as a schema default.
		for key, uri := range map[string]string{"picture-uri": light, "picture-uri-dark": dark} {
			p.Artifact.GSettings = append(p.Artifact.GSettings, plan.GSettingsVendorDefault{
				Schema: "org.gnome.desktop.background", Key: key, Type: "s", Value: gvString(uri),
				Setting: KeyWallpaper, Provenance: wp.Provenance,
			})
		}
	}

	if v, ok := get(KeyPalette); ok {
		pal := v.Value.(theme.Palette)
		accent := ""
		if a, ok := get(KeyAccentColor); ok {
			accent = a.Value.(string)
		}
		l.p.Artifact.GeneratedFiles = append(l.p.Artifact.GeneratedFiles,
			plan.GeneratedFile{Path: gtk4UserCSSPath, Mode: "0644", Content: pal.GTK4CSS(), Provenance: v.Provenance},
			plan.GeneratedFile{Path: gtk3UserCSSPath, Mode: "0644", Content: pal.GTK3CSS(accent), Provenance: v.Provenance},
		)
		if pal.TerminalReady() {
			l.p.Artifact.GeneratedFiles = append(l.p.Artifact.GeneratedFiles,
				plan.GeneratedFile{
					Path: path.Join(ptyxisPalettesDir, pal.Name+".palette"), Mode: "0644",
					Content: pal.PtyxisPalette(), Provenance: v.Provenance,
				},
				plan.GeneratedFile{
					Path: path.Join(dconfDropInDir, "60-deskos-ptyxis"), Mode: "0644",
					Content: pal.PtyxisProfile(ptyxisProfileUUID), Provenance: v.Provenance,
				})
		}
	}

	if v, ok := get(KeyWelcomeTour); ok && !v.Value.(bool) {
		// GNOME Shell shows its first-run welcome dialog (the tour prompt)
		// until welcome-dialog-last-shown-version is at least the version of
		// its last tour change; setting it suppresses the dialog.
		l.set(KeyWelcomeTour, "/org/gnome/shell/welcome-dialog-last-shown-version", "s", gvString(welcomeDialogShownVersion), v.Provenance)
	}

	if hasShield {
		l.set(KeyLockShield, "/org/gnome/desktop/screensaver/picture-uri", "s",
			gvString(l.installAsset(shield.Value.(Asset), shield.Provenance)), shield.Provenance)
	}

	if v, ok := get(KeyLoginLogo); ok {
		if !pl.Gnome.LoginScreen {
			l.errs.Add(fmt.Errorf("%s does not provide a GDM login screen that reads the DeskOS dconf database; %s cannot be applied\n  set by: %s",
				c.Platform.ID(), KeyLoginLogo, describe(v.Provenance)))
		} else {
			a := v.Value.(Asset)
			dest := path.Join(BrandingDir, path.Base(a.Path))
			p.Artifact.Files = append(p.Artifact.Files, plan.FileInstall{
				Path: dest, Mode: "0644", SHA256: a.SHA256, Asset: a.Path, AssetFile: a.File, Provenance: v.Provenance,
			})
			l.set(KeyLoginLogo, "/org/gnome/login-screen/logo", "s", gvString(dest), v.Provenance)
		}
	}

	if v, ok := get(KeyShowUserList); ok {
		if !pl.Gnome.LoginScreen {
			l.errs.Add(fmt.Errorf("%s does not provide a GDM login screen that reads the DeskOS dconf database; %s cannot be applied\n  set by: %s",
				c.Platform.ID(), KeyShowUserList, describe(v.Provenance)))
		} else {
			l.set(KeyShowUserList, "/org/gnome/login-screen/disable-user-list", "b",
				strconv.FormatBool(!v.Value.(bool)), v.Provenance)
		}
	}

	if v, ok := get(KeyBanner); ok {
		if !pl.Gnome.LoginScreen {
			l.errs.Add(fmt.Errorf("%s does not provide a GDM login screen that reads the DeskOS dconf database; %s cannot be applied\n  set by: %s",
				c.Platform.ID(), KeyBanner, describe(v.Provenance)))
		} else {
			text := v.Value.(string)
			l.set(KeyBanner, "/org/gnome/login-screen/banner-message-enable", "b",
				strconv.FormatBool(text != ""), v.Provenance)
			if text != "" {
				l.set(KeyBanner, "/org/gnome/login-screen/banner-message-text", "s", gvString(text), v.Provenance)
			}
		}
	}

	for key, gkey := range map[string]string{
		KeyFontInterface: "font-name",
		KeyFontDocument:  "document-font-name",
		KeyFontMonospace: "monospace-font-name",
	} {
		if v, ok := get(key); ok {
			l.set(key, "/org/gnome/desktop/interface/"+gkey, "s", gvString(fontName(v.Value.(Font))), v.Provenance)
		}
	}
	if v, ok := get(KeyFontAA); ok {
		l.set(KeyFontAA, "/org/gnome/settings-daemon/plugins/xsettings/antialiasing", "s",
			gvString(v.Value.(string)), v.Provenance)
	}
	if v, ok := get(KeyFontHinting); ok {
		l.set(KeyFontHinting, "/org/gnome/settings-daemon/plugins/xsettings/hinting", "s",
			gvString(v.Value.(string)), v.Provenance)
	}
	if v, ok := get(KeyFontRGBA); ok {
		l.set(KeyFontRGBA, "/org/gnome/settings-daemon/plugins/xsettings/rgba-order", "s",
			gvString(v.Value.(string)), v.Provenance)
	}
	// GNOME 47 reads the rendering policy from its own keys and ignores them
	// while font-rendering is automatic, which is its default: declaring any of
	// the three switches it to manual, and each declared value goes with it.
	// The Xsettings keys above stay for X11 applications.
	if _, hasAA := get(KeyFontAA); hasAA {
		l.rendering(get)
	} else if _, hasHint := get(KeyFontHinting); hasHint {
		l.rendering(get)
	} else if _, hasRGBA := get(KeyFontRGBA); hasRGBA {
		l.rendering(get)
	}
	l.fontconfig(get)

	if v, ok := get(KeyFontTitlebar); ok {
		// A title font only applies while GNOME is told not to use the interface
		// font, which is its default.
		l.set(KeyFontTitlebar, "/org/gnome/desktop/wm/preferences/titlebar-uses-system-font", "b", "false", v.Provenance)
		l.set(KeyFontTitlebar, "/org/gnome/desktop/wm/preferences/titlebar-font", "s",
			gvString(fontName(v.Value.(Font))), v.Provenance)
	}
	if v, ok := get(KeyFontTerminal); ok {
		pl := l.c.PlatformSpec
		if pl.Gnome == nil || pl.Gnome.Terminal == nil {
			l.errs.Add(fmt.Errorf("%s declares no terminal, which %s needs\n  set by: %s",
				l.c.Platform.ID(), KeyFontTerminal, describe(v.Provenance)))
		} else {
			t := pl.Gnome.Terminal
			l.p.AddPackage(t.Package, v.Provenance...)
			l.set(KeyFontTerminal, t.DconfPath+t.FontKey, "s", gvString(fontName(v.Value.(Font))), v.Provenance)
			l.set(KeyFontTerminal, t.DconfPath+t.UseSystemFontKey, "b", "false", v.Provenance)
		}
	}
	if v, ok := get(KeyTextScaling); ok {
		l.set(KeyTextScaling, "/org/gnome/desktop/interface/text-scaling-factor", "d",
			strconv.FormatFloat(v.Value.(float64), 'f', -1, 64), v.Provenance)
	}

	for key, gkey := range map[string]string{KeyIconTheme: "icon-theme", KeyCursorTheme: "cursor-theme"} {
		if v, ok := get(key); ok {
			l.set(key, "/org/gnome/desktop/interface/"+gkey, "s", gvString(v.Value.(string)), v.Provenance)
		}
	}

	for key, gkey := range map[string]string{KeyColorScheme: "color-scheme", KeyAccentColor: "accent-color"} {
		if v, ok := get(key); ok {
			l.set(key, "/org/gnome/desktop/interface/"+gkey, "s", gvString(v.Value.(string)), v.Provenance)
		}
	}
	if v, ok := get(KeySoftwareUpdates); ok {
		mode := v.Value.(string)
		l.set(KeySoftwareUpdates, "/org/gnome/software/allow-updates", "b", strconv.FormatBool(mode != "disabled"), v.Provenance)
		l.set(KeySoftwareUpdates, "/org/gnome/software/download-updates", "b", strconv.FormatBool(mode == "automatic"), v.Provenance)
	}
	if v, ok := get(KeyPowerButton); ok {
		l.set(KeyPowerButton, "/org/gnome/settings-daemon/plugins/power/power-button-action", "s", gvString(v.Value.(string)), v.Provenance)
	}
	for key, gkey := range map[string]string{
		KeyLockdownDisablePrinting:            "/org/gnome/desktop/lockdown/disable-printing",
		KeyLockdownDisablePrintSetup:          "/org/gnome/desktop/lockdown/disable-print-setup",
		KeyLockdownDisableSaveToDisk:          "/org/gnome/desktop/lockdown/disable-save-to-disk",
		KeyLockdownDisableLogOut:              "/org/gnome/desktop/lockdown/disable-log-out",
		KeyLockdownDisableUserSwitching:       "/org/gnome/desktop/lockdown/disable-user-switching",
		KeyLockdownDisableApplicationHandlers: "/org/gnome/desktop/lockdown/disable-application-handlers",
		KeyLockdownDisableCommandLine:         "/org/gnome/desktop/lockdown/disable-command-line",
		KeyLockdownDisableShowPassword:        "/org/gnome/desktop/lockdown/disable-show-password",
		KeyLockdownDisableLockScreen:          "/org/gnome/desktop/lockdown/disable-lock-screen",
		KeyLockdownRemovableStorageReadOnly:   "/org/gnome/desktop/lockdown/mount-removable-storage-devices-as-read-only",
		KeyLockdownDisableUserAdministration:  "/org/gnome/desktop/lockdown/user-administration-disabled",
	} {
		if v, ok := get(key); ok {
			l.set(key, gkey, "b", strconv.FormatBool(v.Value.(bool)), v.Provenance)
		}
	}

	l.lowerShell(get)
	l.lowerApplications(get)
	l.lowerExtensions(locks)
	l.lowerDock(get)

	for _, lk := range locks {
		keys, ok := l.bySett[lk.Name]
		if !ok {
			l.errs.Add(fmt.Errorf("GNOME lock %q has no effective value to lock\n  locked by: %s", lk.Name, describe(lk.Provenance)))
			continue
		}
		for _, k := range keys {
			l.db.Locks = append(l.db.Locks, plan.DconfLock{Key: k, Setting: lk.Name, Provenance: lk.Provenance})
		}
	}
	if err := l.errs.Err(); err != nil {
		return err
	}
	p.Artifact.Dconf = l.db
	return nil
}

// TerminalBinding is the accelerator of keyboard.terminal.
const TerminalBinding = "<Control><Alt>t"

// applicationRoles maps the semantic document roles to the MIME types they own.
// MIME types cannot be globbed, so the table lives in Core and a profile names
// only the desktop entry: a profile never authors a MIME list per platform.
var applicationRoles = []struct {
	setting string
	mimes   []string
}{
	{KeyAppBrowser, []string{"application/xhtml+xml", "text/html", "x-scheme-handler/http", "x-scheme-handler/https"}},
	{KeyAppFileManager, []string{"inode/directory"}},
	{KeyAppTextEditor, []string{"text/plain"}},
	{KeyAppImageViewer, []string{"image/gif", "image/jpeg", "image/png", "image/webp"}},
	{KeyAppVideoPlayer, []string{"video/mp4", "video/webm", "video/x-matroska"}},
	{KeyAppPDFViewer, []string{"application/pdf"}},
}

const (
	foldersPath  = "/org/gnome/desktop/app-folders/"
	mediaKeys    = "/org/gnome/settings-daemon/plugins/media-keys/"
	terminalPath = mediaKeys + "custom-keybindings/deskos-terminal/"
)

// lowerShell lowers hot corners, workspaces, app folders, clock and keyboard.
func (l *lowering) lowerShell(get func(string) (compose.Scalar, bool)) {
	for key, gkey := range map[string]string{
		KeyHotCorners:   "/org/gnome/desktop/interface/enable-hot-corners",
		KeyClockWeekday: "/org/gnome/desktop/interface/clock-show-weekday",
		KeyNumLock:      "/org/gnome/desktop/peripherals/keyboard/numlock-state",
	} {
		if v, ok := get(key); ok {
			l.set(key, gkey, "b", strconv.FormatBool(v.Value.(bool)), v.Provenance)
		}
	}
	if v, ok := get(KeyClockFormat); ok {
		l.set(KeyClockFormat, "/org/gnome/desktop/interface/clock-format", "s", gvString(v.Value.(string)), v.Provenance)
	}
	if v, ok := get(KeyKeyboardLayouts); ok {
		pairs := make([]string, 0, len(v.Value.([]KeyboardLayout)))
		for _, l := range v.Value.([]KeyboardLayout) {
			id := l.Layout
			if l.Variant != "" {
				id += "+" + l.Variant
			}
			pairs = append(pairs, "('xkb', "+gvString(id)+")")
		}
		l.set(KeyKeyboardLayouts, "/org/gnome/desktop/input-sources/sources", "a(ss)",
			"["+strings.Join(pairs, ", ")+"]", v.Provenance)
	}
	if v, ok := get(KeyXKBOptions); ok {
		l.set(KeyXKBOptions, "/org/gnome/desktop/input-sources/xkb-options", "as", gvStrings(v.Value.([]string)), v.Provenance)
	}
	if v, ok := get(KeyWorkspaces); ok {
		n := v.Value.(Workspaces).Count
		l.set(KeyWorkspaces, "/org/gnome/mutter/dynamic-workspaces", "b", strconv.FormatBool(n == 0), v.Provenance)
		if n > 0 {
			l.set(KeyWorkspaces, "/org/gnome/desktop/wm/preferences/num-workspaces", "i", strconv.Itoa(n), v.Provenance)
		}
	}
	if v, ok := get(KeyAppFolders); ok && !l.c.PlatformSpec.Gnome.AppFolders {
		l.errs.Add(fmt.Errorf("%s does not keep app folders from the DeskOS dconf database; %s cannot be applied\n  set by: %s",
			l.c.Platform.ID(), KeyAppFolders, describe(v.Provenance)))
	} else if ok {
		folders := v.Value.([]AppFolder)
		var ids []string
		for _, f := range folders {
			ids = append(ids, f.ID)
		}
		l.set(KeyAppFolders, foldersPath+"folder-children", "as", gvStrings(ids), v.Provenance)
		for _, f := range folders {
			base := foldersPath + "folders/" + f.ID + "/"
			l.set(KeyAppFolders, base+"name", "s", gvString(f.Name), v.Provenance)
			l.set(KeyAppFolders, base+"translate", "b", "false", v.Provenance)
			l.set(KeyAppFolders, base+"apps", "as", gvStrings(f.Apps), v.Provenance)
			l.set(KeyAppFolders, base+"categories", "as", gvStrings(f.Categories), v.Provenance)
		}
	}
	if v, ok := get(KeyTerminal); ok {
		pl := l.c.PlatformSpec
		if pl.Gnome.AppLauncher == nil {
			l.errs.Add(fmt.Errorf("%s declares no GNOME application launcher, which %s needs\n  set by: %s",
				l.c.Platform.ID(), KeyTerminal, describe(v.Provenance)))
			return
		}
		l.p.AddPackage(pl.Gnome.AppLauncher.Package, v.Provenance...)
		id := strings.TrimSuffix(v.Value.(string), ".desktop")
		l.set(KeyTerminal, mediaKeys+"custom-keybindings", "as", gvStrings([]string{terminalPath}), v.Provenance)
		l.set(KeyTerminal, terminalPath+"name", "s", gvString("Terminal"), v.Provenance)
		l.set(KeyTerminal, terminalPath+"command", "s", gvString(pl.Gnome.AppLauncher.Command+" "+id), v.Provenance)
		l.set(KeyTerminal, terminalPath+"binding", "s", gvString(TerminalBinding), v.Provenance)
		l.p.Artifact.GeneratedFiles = append(l.p.Artifact.GeneratedFiles, plan.GeneratedFile{
			Path: terminalListPath, Mode: "0644",
			Content:    "# Generated by deskosctl. Do not edit; change the DeskOS resources instead.\n" + v.Value.(string) + "\n",
			Provenance: v.Provenance,
		})
	}
}

// lowerApplications writes the default document handlers. A profile names one
// desktop entry per role; Core owns the MIME table and sorts the output, so file
// and profile order cannot change the generated file.
func (l *lowering) lowerApplications(get func(string) (compose.Scalar, bool)) {
	seen := map[string]bool{}
	var entries []string
	var prov []model.Provenance
	for _, role := range applicationRoles {
		v, ok := get(role.setting)
		if !ok {
			continue
		}
		id := v.Value.(string)
		for _, mime := range role.mimes {
			if seen[mime] {
				l.errs.Add(fmt.Errorf("%s and another application role both claim MIME type %q", role.setting, mime))
				continue
			}
			seen[mime] = true
			entries = append(entries, mime+"="+id+";")
		}
		prov = append(prov, v.Provenance...)
	}
	if len(entries) == 0 {
		return
	}
	sort.Strings(entries)
	l.p.Artifact.GeneratedFiles = append(l.p.Artifact.GeneratedFiles, plan.GeneratedFile{
		Path: mimeAppsPath, Mode: "0644",
		Content:    "# Generated by deskosctl. Do not edit; change the DeskOS resources instead.\n[Default Applications]\n" + strings.Join(entries, "\n") + "\n",
		Provenance: prov,
	})
}

// lowerExtensions writes the effective `enabled-extensions` list. It is the only
// writer **of the plan**: the platform's own list, every profile decision and the dock
// all pass through here, so two writers cannot disagree and neither the order of the
// profiles nor of the files can change the result (docs/adr/0017).
//
// One other writer exists outside the plan: the experimental GNOME Shell stylesheet
// path writes its own drop-in, `61-deskos-shell`, which repeats the whole list because
// dconf does not merge arrays and sorts after `50-deskos`. Whoever opts into it has to
// pass the effective list, or it overrides this one (docs/roadmap.md).
//
// The list is the platform's own, in its order, with the decisions applied. Any
// UUID may be decided: the platform's catalog installs the package when it declares
// the extension, and an extension the composition brings itself is installed by the
// packages the composition declares and verified in the session (docs/adr/0017).
func (l *lowering) lowerExtensions(locks []compose.Member) {
	const key = "/org/gnome/shell/enabled-extensions"
	pl := l.c.PlatformSpec
	list := append([]string(nil), pl.Gnome.EnabledExtensions...)

	var decisions []compose.Scalar
	for _, s := range l.c.Result.Scalars(DomainSettings) {
		if strings.HasPrefix(s.Key, extensionKeyPrefix) {
			decisions = append(decisions, s)
		}
	}
	// Sorted by the UUID they decide about, so the appended order does not depend on
	// how the composition happened to enumerate them.
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].Key < decisions[j].Key })

	// The whole list is one dconf key, so it is written once, under one name a lock
	// can attach to. It is written even when it equals the platform's own list: the
	// artifact then states the extensions it enables, which is what the session check
	// verifies, instead of leaning on whichever default the vendor shipped.
	var prov []model.Provenance
	for _, s := range decisions {
		uuid := strings.TrimPrefix(s.Key, extensionKeyPrefix)
		if s.Value.(bool) {
			if !contains(list, uuid) {
				list = append(list, uuid)
			}
			// What the composition turned on is what the session check asks GNOME
			// about: a UUID that does not really load must be visible.
			l.p.Artifact.EnabledExtensions = append(l.p.Artifact.EnabledExtensions, uuid)
			if ext, ok := pl.GnomeCatalogExtension(uuid); ok {
				if ext.Source != nil {
					// The platform's repositories do not carry it, so the image
					// delivers the pinned artifact itself (docs/adr/0021). The
					// provenance is both halves of the decision: the catalog entry
					// that names the delivery, and the profile that enabled it.
					prov := append([]model.Provenance{{
						Resource: l.c.Platform.ID(),
						Source:   l.c.Platform.Source.String(),
					}}, s.Provenance...)
					l.p.Artifact.Extensions = append(l.p.Artifact.Extensions, plan.ExtensionInstall{
						UUID: ext.UUID, Version: ext.Source.Version,
						URL: ext.Source.URL, SHA256: ext.Source.SHA256,
						Provenance: prov,
					})
				} else {
					l.p.AddPackage(ext.Package, s.Provenance...)
				}
			}
		} else {
			list = without(list, uuid)
		}
		prov = append(prov, s.Provenance...)
	}
	if len(prov) == 0 {
		// Nothing decided: the list is the platform's own, and the platform is where it
		// comes from.
		prov = []model.Provenance{{Resource: l.c.Platform.ID(), Source: l.c.Platform.Source.String()}}
	}
	l.set(KeyExtensions, key, "as", gvStrings(list), prov)
}

// locksNamed reports whether a lock names one of the settings given.
func locksNamed(locks []compose.Member, settings ...string) bool {
	for _, lk := range locks {
		for _, s := range settings {
			if lk.Name == s {
				return true
			}
		}
	}
	return false
}

func without(list []string, s string) []string {
	out := make([]string, 0, len(list))
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// dockOptions are settings that only exist while the dock extension runs.
var dockOptions = []string{KeyDockPosition, KeyDockBehavior, KeyDockIconSize, KeyDockShowTrash}

// lowerDock applies the dock settings DeskOS models: they need an effective
// decision enabling the dock's extension; a decision of false at a layer masks
// settings from lower layers (reported as a warning) and rejects settings from its
// own or higher layers; no decision at all with settings present is an error.
// Whether the dock runs is not decided here (docs/adr/0017).
func (l *lowering) lowerDock(get func(string) (compose.Scalar, bool)) {
	const schema = "/org/gnome/shell/extensions/dash-to-dock/"
	pl := l.c.PlatformSpec
	ext, dockInCatalog := pl.GnomeExtension(DockExtension)
	// The dock's options are the settings DeskOS models for Dash to Dock, and whether
	// the dock runs is the decision about its extension, taken with every other
	// extension (docs/adr/0017). The platform's catalog entry is what names the
	// extension whose decision gates them.
	en, hasEn := get(ExtensionSetting(ext.UUID))
	if !dockInCatalog {
		en, hasEn = compose.Scalar{}, false
	}
	enabled := hasEn && en.Value.(bool)
	var active []compose.Scalar
	for _, key := range dockOptions {
		v, ok := get(key)
		switch {
		case !ok:
		case !dockInCatalog:
			l.errs.Add(fmt.Errorf("%s needs the GNOME extension DeskOS models as %q (%s) so the settings have an extension to gate them, and %s does not declare it\n  set by: %s",
				key, DockExtension, dockSettingsAdapter, l.c.Platform.ID(), describe(v.Provenance)))
		case !hasEn:
			l.errs.Add(fmt.Errorf("%s requires enabling %q, which no profile decides\n  set by: %s",
				key, ext.UUID, describe(v.Provenance)))
		case enabled:
			active = append(active, v)
		case v.Layer < en.Layer:
			l.p.Warnings = append(l.p.Warnings, fmt.Sprintf("%s from %s layer is masked: %s is false at %s layer (%s)",
				key, v.Layer, ext.UUID, en.Layer, describeShort(en.Provenance)))
		default:
			l.errs.Add(fmt.Errorf("%s is set at %s layer but %s is disabled at %s layer; a disabled dock cannot apply it\n  set by: %s\n  disabled by: %s",
				key, v.Layer, ext.UUID, en.Layer, describe(v.Provenance), describe(en.Provenance)))
		}
	}
	if !enabled {
		// The list itself is lowerExtensions' job.
		return
	}
	for _, v := range active {
		switch v.Key {
		case KeyDockPosition:
			l.set(v.Key, schema+"dock-position", "s", gvString(strings.ToUpper(v.Value.(string))), v.Provenance)
		case KeyDockBehavior:
			mode := v.Value.(string)
			l.set(v.Key, schema+"dock-fixed", "b", strconv.FormatBool(mode == "fixed"), v.Provenance)
			l.set(v.Key, schema+"autohide", "b", strconv.FormatBool(mode != "fixed"), v.Provenance)
			l.set(v.Key, schema+"intellihide", "b", strconv.FormatBool(mode == "intellihide"), v.Provenance)
		case KeyDockIconSize:
			l.set(v.Key, schema+"dash-max-icon-size", "i", strconv.Itoa(v.Value.(int)), v.Provenance)
		case KeyDockShowTrash:
			l.set(v.Key, schema+"show-trash", "b", strconv.FormatBool(v.Value.(bool)), v.Provenance)
		}
	}
}

func describeShort(ps []model.Provenance) string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Resource)
	}
	return strings.Join(out, ", ")
}

func (l *lowering) set(setting, key, typ, value string, prov []model.Provenance) {
	l.db.Defaults = append(l.db.Defaults, plan.DconfDefault{Key: key, Type: typ, Value: value, Setting: setting, Provenance: prov})
	l.bySett[setting] = append(l.bySett[setting], key)
}

// rendering mirrors the low-level rendering decision into the keys GNOME 47
// added. It is one function because GNOME gates all three on font-rendering:
// setting one of them without manual is a value nothing reads.
func (l *lowering) rendering(get func(string) (compose.Scalar, bool)) {
	first := true
	for key, gkey := range map[string]string{
		KeyFontAA:      "/org/gnome/desktop/interface/font-antialiasing",
		KeyFontHinting: "/org/gnome/desktop/interface/font-hinting",
		KeyFontRGBA:    "/org/gnome/desktop/interface/font-rgba-order",
	} {
		v, ok := get(key)
		if !ok {
			continue
		}
		if first {
			l.set(key, "/org/gnome/desktop/interface/font-rendering", "s", gvString("manual"), v.Provenance)
			first = false
		}
		l.set(key, gkey, "s", gvString(v.Value.(string)), v.Provenance)
	}
}

// FontconfigPath is where the rendering policy lands: fontconfig reads every
// .conf in /etc/fonts/conf.d, and GTK on Wayland asks fontconfig directly, so this
// file is what changes the rendering for every application. The Xsettings keys
// cover X11 applications.
const FontconfigPath = "/etc/fonts/conf.d/60-deskos-fonts.conf"

var hintStyles = map[string]string{
	"none": "hintnone", "slight": "hintslight", "medium": "hintmedium", "full": "hintfull",
}

var lcdFilters = map[string]string{
	"none": "lcdnone", "default": "lcddefault", "light": "lcdlight", "legacy": "lcdlegacy",
}

// fontconfig writes the rendering policy when a profile declares any of its
// settings. Only what is declared is written: an image that says nothing keeps the
// distribution's choices.
func (l *lowering) fontconfig(get func(string) (compose.Scalar, bool)) {
	aa, hasAA := get(KeyFontAA)
	hint, hasHint := get(KeyFontHinting)
	rgba, hasRGBA := get(KeyFontRGBA)
	lcd, hasLCD := get(KeyFontLCD)
	if !hasAA && !hasHint && !hasRGBA && !hasLCD {
		return
	}
	var prov []model.Provenance
	var edits []string
	if hasAA {
		prov = append(prov, aa.Provenance...)
		mode := aa.Value.(string)
		edits = append(edits, fmt.Sprintf(`    <edit name="antialias" mode="assign"><bool>%t</bool></edit>`, mode != "none"))
		switch mode {
		case "grayscale":
			edits = append(edits, `    <edit name="rgba" mode="assign"><const>none</const></edit>`)
		case "rgba":
			order := "rgb"
			if hasRGBA {
				order = rgba.Value.(string)
				prov = append(prov, rgba.Provenance...)
			}
			edits = append(edits, fmt.Sprintf(`    <edit name="rgba" mode="assign"><const>%s</const></edit>`, order))
		}
	} else if hasRGBA {
		prov = append(prov, rgba.Provenance...)
		edits = append(edits, fmt.Sprintf(`    <edit name="rgba" mode="assign"><const>%s</const></edit>`, rgba.Value.(string)))
	}
	if hasHint {
		prov = append(prov, hint.Provenance...)
		edits = append(edits, fmt.Sprintf(`    <edit name="hintstyle" mode="assign"><const>%s</const></edit>`,
			hintStyles[hint.Value.(string)]))
	}
	if hasLCD {
		prov = append(prov, lcd.Provenance...)
		edits = append(edits, fmt.Sprintf(`    <edit name="lcdfilter" mode="assign"><const>%s</const></edit>`,
			lcdFilters[lcd.Value.(string)]))
	}
	content := "<?xml version=\"1.0\"?>\n" +
		"<!DOCTYPE fontconfig SYSTEM \"urn:fontconfig:fonts.dtd\">\n" +
		"<!-- Generated by deskosctl. Do not edit; change the DeskOS resources instead. -->\n" +
		"<fontconfig>\n  <match target=\"font\">\n" +
		strings.Join(edits, "\n") + "\n  </match>\n</fontconfig>\n"
	l.p.Artifact.GeneratedFiles = append(l.p.Artifact.GeneratedFiles, plan.GeneratedFile{
		Path: FontconfigPath, Mode: "0644", Content: content, Provenance: prov,
	})
}

func (l *lowering) installAsset(a Asset, prov []model.Provenance) string {
	dest := path.Join(BackgroundsDir, path.Base(a.Path))
	l.p.Artifact.Files = append(l.p.Artifact.Files, plan.FileInstall{
		Path: dest, Mode: "0644", SHA256: a.SHA256, Asset: a.Path, AssetFile: a.File, Provenance: prov,
	})
	return "file://" + dest
}

// gvString renders a GVariant text-format string.
func gvString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

func gvStrings(list []string) string {
	if len(list) == 0 {
		return "@as []"
	}
	parts := make([]string, len(list))
	for i, s := range list {
		parts[i] = gvString(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func gvUint32(s Seconds) string { return "uint32 " + strconv.FormatUint(uint64(s), 10) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func describe(ps []model.Provenance) string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Describe()+" ("+p.Source+")")
	}
	return strings.Join(out, "; ")
}
