// Package desktop provides desktop.deskos.org/GnomeProfile.
//
// GnomeProfile exposes administrator concepts (window controls, favorites,
// idle and lock behavior, wallpaper, fonts, color scheme, dock, GNOME
// Software updates, hot corners, workspaces, app folders, clock, keyboard). The provider validates
// them as semantic settings; the lowerer translates the composed settings to
// dconf using only facts declared by the Platform.
package desktop

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/providers/assets"
	"github.com/deskosproject/deskos-core/internal/registry"
)

var GnomeProfileGVK = model.GVK{Group: model.GroupDesktop, Version: model.V1Alpha1, Kind: "GnomeProfile"}

// Composition domains owned by this package.
var (
	DomainSettings = compose.Domain{ID: "gnome-setting", Label: "GNOME setting"}
	DomainLocks    = compose.Domain{ID: "gnome-lock", Label: "GNOME lock"}
)

// Semantic setting keys. They are the public vocabulary used in locks and
// in composition messages.
const (
	KeyButtons         = "windows.buttons"
	KeyButtonPlacement = "windows.buttonPlacement"
	KeyWallpaper       = "appearance.wallpaper"
	KeyLoginLogo       = "appearance.loginLogo"
	KeyFontInterface   = "appearance.fonts.interface"
	KeyFontDocument    = "appearance.fonts.document"
	KeyFontMonospace   = "appearance.fonts.monospace"
	KeyIconTheme       = "appearance.iconTheme"
	KeyCursorTheme     = "appearance.cursorTheme"
	KeyColorScheme     = "appearance.colorScheme"
	KeyAccentColor     = "appearance.accentColor"
	KeyFavorites       = "shell.favorites"
	KeyBlankAfter      = "session.idle.blankAfter"
	KeyLockEnabled     = "session.lock.enabled"
	KeyLockDelay       = "session.lock.delay"
	KeyDockEnabled     = "dock.enabled"
	KeyDockPosition    = "dock.position"
	KeyDockBehavior    = "dock.behavior"
	KeyDockIconSize    = "dock.iconSize"
	KeyDockShowTrash   = "dock.showTrash"
	KeySoftwareUpdates = "software.updates"
	KeyHotCorners      = "shell.hotCorners"
	KeyWorkspaces      = "shell.workspaces"
	KeyAppFolders      = "shell.appFolders"
	KeyClockWeekday    = "clock.showWeekday"
	KeyClockFormat     = "clock.format"
	KeyNumLock         = "keyboard.numLock"
	KeyTerminal        = "keyboard.terminal"
)

var knownKeys = map[string]bool{
	KeyButtons: true, KeyButtonPlacement: true, KeyWallpaper: true, KeyLoginLogo: true,
	KeyFontInterface: true, KeyFontDocument: true, KeyFontMonospace: true, KeyIconTheme: true,
	KeyCursorTheme: true, KeyFavorites: true, KeyBlankAfter: true, KeyLockEnabled: true,
	KeyLockDelay: true, KeyDockEnabled: true, KeyDockPosition: true, KeyDockBehavior: true,
	KeyDockIconSize: true, KeyDockShowTrash: true, KeyColorScheme: true, KeyAccentColor: true,
	KeySoftwareUpdates: true, KeyHotCorners: true, KeyWorkspaces: true, KeyAppFolders: true,
	KeyClockWeekday: true, KeyClockFormat: true, KeyNumLock: true, KeyTerminal: true,
}

// Enumerations of gsettings-desktop-schemas 47.1 (GDesktopColorScheme, GDesktopAccentColor).
var (
	colorSchemes = []string{"default", "prefer-dark", "prefer-light"}
	accentColors = []string{"blue", "teal", "green", "yellow", "orange", "red", "pink", "purple", "slate"}
	updateModes  = []string{"automatic", "manual", "disabled"}
	clockFormats = []string{"24h", "12h"}
)

// MaxWorkspaces is mutter's limit for num-workspaces.
const MaxWorkspaces = 36

// GnomeProfileSpec is the public GnomeProfile spec.
type GnomeProfileSpec struct {
	Defaults Settings `json:"defaults"`
	Locks    []string `json:"locks,omitempty"`
}

type Settings struct {
	Windows    *Windows    `json:"windows,omitempty"`
	Appearance *Appearance `json:"appearance,omitempty"`
	Shell      *Shell      `json:"shell,omitempty"`
	Session    *Session    `json:"session,omitempty"`
	Dock       *Dock       `json:"dock,omitempty"`
	Software   *Software   `json:"software,omitempty"`
	Clock      *Clock      `json:"clock,omitempty"`
	Keyboard   *Keyboard   `json:"keyboard,omitempty"`
}

type Windows struct {
	Buttons         []string `json:"buttons,omitempty"`
	ButtonPlacement string   `json:"buttonPlacement,omitempty"`
}

type Appearance struct {
	Wallpaper   *Wallpaper `json:"wallpaper,omitempty"`
	LoginLogo   string     `json:"loginLogo,omitempty"`
	Fonts       *Fonts     `json:"fonts,omitempty"`
	IconTheme   string     `json:"iconTheme,omitempty"`
	CursorTheme string     `json:"cursorTheme,omitempty"`
	ColorScheme string     `json:"colorScheme,omitempty"`
	AccentColor string     `json:"accentColor,omitempty"`
}

type Wallpaper struct {
	Light string `json:"light,omitempty"`
	Dark  string `json:"dark,omitempty"`
}

type Fonts struct {
	Interface *Font `json:"interface,omitempty"`
	Document  *Font `json:"document,omitempty"`
	Monospace *Font `json:"monospace,omitempty"`
}

type Font struct {
	Family string  `json:"family"`
	Size   float64 `json:"size"`
}

type Shell struct {
	Favorites  []string    `json:"favorites,omitempty"`
	HotCorners *bool       `json:"hotCorners,omitempty"`
	Workspaces any         `json:"workspaces,omitempty"`
	AppFolders []AppFolder `json:"appFolders,omitempty"`
}

// AppFolder is an app grid folder; declared folders replace GNOME's built-in ones.
type AppFolder struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Apps       []string `json:"apps,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

type Clock struct {
	ShowWeekday *bool  `json:"showWeekday,omitempty"`
	Format      string `json:"format,omitempty"`
}

type Keyboard struct {
	NumLock  *bool  `json:"numLock,omitempty"`
	Terminal string `json:"terminal,omitempty"`
}

// Workspaces is the composed workspace mode; Count is 0 for dynamic workspaces.
type Workspaces struct {
	Count int `json:"count"`
}

type Session struct {
	Idle *Idle `json:"idle,omitempty"`
	Lock *Lock `json:"lock,omitempty"`
}

type Idle struct {
	BlankAfter string `json:"blankAfter,omitempty"`
}

type Lock struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Delay   string `json:"delay,omitempty"`
}

type Dock struct {
	Enabled   *bool  `json:"enabled,omitempty"`
	Position  string `json:"position,omitempty"`
	Behavior  string `json:"behavior,omitempty"`
	IconSize  *int   `json:"iconSize,omitempty"`
	ShowTrash *bool  `json:"showTrash,omitempty"`
}

// Software is GNOME Software behavior.
type Software struct {
	Updates string `json:"updates,omitempty"`
}

// Asset is a composed image asset value.
type Asset struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	File   string `json:"-"`
}

// WallpaperValue is the composed wallpaper; Dark is optional.
type WallpaperValue struct {
	Light Asset  `json:"light"`
	Dark  *Asset `json:"dark,omitempty"`
}

// Seconds is a duration setting in whole seconds.
type Seconds uint32

type setting struct {
	key     string
	value   any
	display string
}

type decoded struct {
	settings []setting
	locks    []string
}

var (
	fontFamilyRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _-]*$`)
	themeRE      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._+-]*$`)
	desktopIDRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.desktop$`)
	assetNameRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.(png|jpg|jpeg|svg)$`)
	logoNameRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.(png|svg)$`)
	folderIDRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	categoryRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	folderNameRE = regexp.MustCompile(`^[^\x00-\x1f\x7f]+$`)
)

// Providers returns the desktop.deskos.org providers.
func Providers() []registry.Provider { return []registry.Provider{gnomeProfile{}, theme{}} }

type gnomeProfile struct{}

func (gnomeProfile) GVK() model.GVK { return GnomeProfileGVK }
func (gnomeProfile) Schema() string { return "desktop.deskos.org/v1alpha1/gnomeprofile.json" }

func (gnomeProfile) Decode(res *model.Resource) error {
	var spec GnomeProfileSpec
	if err := model.DecodeSpec(res, &spec); err != nil {
		return err
	}
	d := &decoded{}
	var errs model.ErrorList
	bad := func(format string, a ...any) { errs.Add(model.Errorf(res, format, a...)) }
	set := func(key string, value any, display string) {
		d.settings = append(d.settings, setting{key: key, value: value, display: display})
	}
	s := spec.Defaults

	if w := s.Windows; w != nil {
		if w.Buttons != nil {
			seen := map[string]bool{}
			for _, b := range w.Buttons {
				if b != "minimize" && b != "maximize" && b != "close" {
					bad("%s: unknown window button %q (expected minimize, maximize or close)", KeyButtons, b)
				}
				if seen[b] {
					bad("%s: button %q is listed twice", KeyButtons, b)
				}
				seen[b] = true
			}
			if len(w.Buttons) == 0 {
				bad("%s: at least one button is required", KeyButtons)
			}
			set(KeyButtons, w.Buttons, strings.Join(w.Buttons, ","))
		}
		if w.ButtonPlacement != "" {
			if w.ButtonPlacement != "left" && w.ButtonPlacement != "right" {
				bad("%s: expected left or right, got %q", KeyButtonPlacement, w.ButtonPlacement)
			}
			set(KeyButtonPlacement, w.ButtonPlacement, w.ButtonPlacement)
		}
	}

	if a := s.Appearance; a != nil {
		if wp := a.Wallpaper; wp != nil {
			readAsset := func(ref string) *Asset {
				as, err := assets.Read(res, ref)
				if err != nil {
					bad("%s: %v", KeyWallpaper, err)
					return nil
				}
				if !assetNameRE.MatchString(path.Base(as.Path)) {
					bad("%s: %s must be a .png, .jpg, .jpeg or .svg file with a simple name", KeyWallpaper, as.Path)
					return nil
				}
				return &Asset{Path: as.Path, SHA256: as.SHA256, File: as.File}
			}
			if wp.Light == "" {
				bad("%s: light is required", KeyWallpaper)
			} else if light := readAsset(wp.Light); light != nil {
				v := WallpaperValue{Light: *light}
				display := light.Path
				if wp.Dark != "" {
					v.Dark = readAsset(wp.Dark)
					if v.Dark != nil {
						display += " (dark: " + v.Dark.Path + ")"
					}
				}
				set(KeyWallpaper, v, display)
			}
		}
		if a.LoginLogo != "" {
			as, err := assets.Read(res, a.LoginLogo)
			switch {
			case err != nil:
				bad("%s: %v", KeyLoginLogo, err)
			case !logoNameRE.MatchString(path.Base(as.Path)):
				bad("%s: %s must be a .png or .svg file with a simple name", KeyLoginLogo, as.Path)
			default:
				set(KeyLoginLogo, Asset{Path: as.Path, SHA256: as.SHA256, File: as.File}, as.Path)
			}
		}
		if f := a.Fonts; f != nil {
			for key, font := range map[string]*Font{KeyFontInterface: f.Interface, KeyFontDocument: f.Document, KeyFontMonospace: f.Monospace} {
				if font == nil {
					continue
				}
				if !fontFamilyRE.MatchString(font.Family) || font.Size < 6 || font.Size > 72 {
					bad("%s: font needs a family name and a size between 6 and 72", key)
					continue
				}
				set(key, *font, fontName(*font))
			}
		}
		for key, v := range map[string]string{KeyIconTheme: a.IconTheme, KeyCursorTheme: a.CursorTheme} {
			if v == "" {
				continue
			}
			if !themeRE.MatchString(v) {
				bad("%s: invalid theme name %q", key, v)
			}
			set(key, v, v)
		}
		enum := func(key, v string, allowed []string) {
			if v == "" {
				return
			}
			if !slices.Contains(allowed, v) {
				bad("%s: expected %s, got %q", key, strings.Join(allowed, ", "), v)
			}
			set(key, v, v)
		}
		enum(KeyColorScheme, a.ColorScheme, colorSchemes)
		enum(KeyAccentColor, a.AccentColor, accentColors)
	}

	if sh := s.Shell; sh != nil {
		if sh.Favorites != nil {
			seen := map[string]bool{}
			for _, f := range sh.Favorites {
				if !desktopIDRE.MatchString(f) {
					bad("%s: %q is not a desktop file id (for example firefox.desktop)", KeyFavorites, f)
				}
				if seen[f] {
					bad("%s: %q is listed twice", KeyFavorites, f)
				}
				seen[f] = true
			}
			set(KeyFavorites, sh.Favorites, strings.Join(sh.Favorites, ", "))
		}
		if sh.HotCorners != nil {
			set(KeyHotCorners, *sh.HotCorners, strconv.FormatBool(*sh.HotCorners))
		}
		if sh.Workspaces != nil {
			switch w := sh.Workspaces.(type) {
			case string:
				if w != "dynamic" {
					bad("%s: expected dynamic or a number from 1 to %d, got %q", KeyWorkspaces, MaxWorkspaces, w)
				}
				set(KeyWorkspaces, Workspaces{}, "dynamic")
			case float64:
				if w != float64(int(w)) || w < 1 || w > MaxWorkspaces {
					bad("%s: expected dynamic or a number from 1 to %d, got %v", KeyWorkspaces, MaxWorkspaces, w)
				}
				set(KeyWorkspaces, Workspaces{Count: int(w)}, strconv.Itoa(int(w)))
			default:
				bad("%s: expected dynamic or a number from 1 to %d", KeyWorkspaces, MaxWorkspaces)
			}
		}
		if sh.AppFolders != nil {
			var ids []string
			seenID := map[string]bool{}
			for _, f := range sh.AppFolders {
				if !folderIDRE.MatchString(f.ID) {
					bad("%s: invalid folder id %q (letters, digits, _ and -)", KeyAppFolders, f.ID)
				}
				if seenID[f.ID] {
					bad("%s: folder %q is listed twice", KeyAppFolders, f.ID)
				}
				seenID[f.ID] = true
				ids = append(ids, f.ID)
				if !folderNameRE.MatchString(f.Name) {
					bad("%s: folder %q needs a single-line name", KeyAppFolders, f.ID)
				}
				if len(f.Apps) == 0 && len(f.Categories) == 0 {
					bad("%s: folder %q needs apps or categories", KeyAppFolders, f.ID)
				}
				seenApp := map[string]bool{}
				for _, a := range f.Apps {
					if !desktopIDRE.MatchString(a) {
						bad("%s: folder %q: %q is not a desktop file id", KeyAppFolders, f.ID, a)
					}
					if seenApp[a] {
						bad("%s: folder %q: %q is listed twice", KeyAppFolders, f.ID, a)
					}
					seenApp[a] = true
				}
				for _, c := range f.Categories {
					if !categoryRE.MatchString(c) {
						bad("%s: folder %q: invalid category %q", KeyAppFolders, f.ID, c)
					}
				}
			}
			if len(sh.AppFolders) == 0 {
				bad("%s: at least one folder is required", KeyAppFolders)
			}
			set(KeyAppFolders, sh.AppFolders, strings.Join(ids, ", "))
		}
	}

	if c := s.Clock; c != nil {
		if c.ShowWeekday != nil {
			set(KeyClockWeekday, *c.ShowWeekday, strconv.FormatBool(*c.ShowWeekday))
		}
		if c.Format != "" {
			if !slices.Contains(clockFormats, c.Format) {
				bad("%s: expected 24h or 12h, got %q", KeyClockFormat, c.Format)
			}
			set(KeyClockFormat, c.Format, c.Format)
		}
	}

	if k := s.Keyboard; k != nil {
		if k.NumLock != nil {
			set(KeyNumLock, *k.NumLock, strconv.FormatBool(*k.NumLock))
		}
		if k.Terminal != "" {
			if !desktopIDRE.MatchString(k.Terminal) {
				bad("%s: %q is not a desktop file id (for example org.gnome.Ptyxis.desktop)", KeyTerminal, k.Terminal)
			}
			set(KeyTerminal, k.Terminal, k.Terminal+" (Ctrl+Alt+T)")
		}
	}

	if se := s.Session; se != nil {
		if se.Idle != nil && se.Idle.BlankAfter != "" {
			if secs, err := parseDuration(se.Idle.BlankAfter, true); err != nil {
				bad("%s: %v", KeyBlankAfter, err)
			} else {
				set(KeyBlankAfter, secs, se.Idle.BlankAfter)
			}
		}
		if l := se.Lock; l != nil {
			if l.Enabled != nil {
				set(KeyLockEnabled, *l.Enabled, strconv.FormatBool(*l.Enabled))
			}
			if l.Delay != "" {
				if secs, err := parseDuration(l.Delay, false); err != nil {
					bad("%s: %v", KeyLockDelay, err)
				} else {
					set(KeyLockDelay, secs, l.Delay)
				}
			}
		}
	}

	if dk := s.Dock; dk != nil {
		if dk.Enabled != nil {
			set(KeyDockEnabled, *dk.Enabled, strconv.FormatBool(*dk.Enabled))
		}
		if dk.Position != "" {
			switch dk.Position {
			case "bottom", "left", "right", "top":
			default:
				bad("%s: expected bottom, left, right or top, got %q", KeyDockPosition, dk.Position)
			}
			set(KeyDockPosition, dk.Position, dk.Position)
		}
		if dk.Behavior != "" {
			switch dk.Behavior {
			case "fixed", "autohide", "intellihide":
			default:
				bad("%s: expected fixed, autohide or intellihide, got %q", KeyDockBehavior, dk.Behavior)
			}
			set(KeyDockBehavior, dk.Behavior, dk.Behavior)
		}
		if dk.IconSize != nil {
			if *dk.IconSize < 16 || *dk.IconSize > 64 {
				bad("%s: must be between 16 and 64", KeyDockIconSize)
			}
			set(KeyDockIconSize, *dk.IconSize, strconv.Itoa(*dk.IconSize))
		}
		if dk.ShowTrash != nil {
			set(KeyDockShowTrash, *dk.ShowTrash, strconv.FormatBool(*dk.ShowTrash))
		}
	}

	if sw := s.Software; sw != nil && sw.Updates != "" {
		if !slices.Contains(updateModes, sw.Updates) {
			bad("%s: expected %s, got %q", KeySoftwareUpdates, strings.Join(updateModes, ", "), sw.Updates)
		}
		set(KeySoftwareUpdates, sw.Updates, sw.Updates)
	}

	seenLock := map[string]bool{}
	for _, l := range spec.Locks {
		if !knownKeys[l] {
			bad("locks: unknown setting %q", l)
		}
		if seenLock[l] {
			bad("locks: %q is listed twice", l)
		}
		seenLock[l] = true
	}
	d.locks = spec.Locks
	if len(d.settings) == 0 && len(d.locks) == 0 {
		bad("defaults or locks must define at least one setting")
	}
	res.Object = d
	return errs.Err()
}

func (gnomeProfile) Contribute(res *model.Resource, s *compose.Scope) error {
	d := res.Object.(*decoded)
	for _, st := range d.settings {
		s.SetScalar(DomainSettings, st.key, st.value, st.display)
	}
	for _, l := range d.locks {
		s.AddMember(DomainLocks, l)
	}
	return nil
}

func fontName(f Font) string {
	return f.Family + " " + strconv.FormatFloat(f.Size, 'f', -1, 64)
}

// parseDuration accepts Go duration syntax in whole seconds, and "never"
// when allowed (encoded as 0, GNOME's value for "never").
func parseDuration(s string, allowNever bool) (Seconds, error) {
	if s == "never" {
		if !allowNever {
			return 0, fmt.Errorf("\"never\" is not valid here")
		}
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (for example 30s, 10m, 1h)", s)
	}
	if d < 0 || d%time.Second != 0 || d > 24*time.Hour {
		return 0, fmt.Errorf("duration %q must be whole seconds between 0 and 24h", s)
	}
	if d == 0 && allowNever {
		return 0, fmt.Errorf("duration 0 is ambiguous here; use \"never\"")
	}
	return Seconds(d / time.Second), nil
}
