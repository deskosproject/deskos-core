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
	KeyAppBrowser      = "applications.browser"
	KeyAppFileManager  = "applications.fileManager"
	KeyAppTextEditor   = "applications.textEditor"
	KeyAppImageViewer  = "applications.imageViewer"
	KeyAppVideoPlayer  = "applications.videoPlayer"
	KeyAppPDFViewer    = "applications.pdfViewer"
	KeyWallpaper       = "appearance.wallpaper"
	KeyLoginLogo       = "appearance.loginLogo"
	KeyLockShield      = "appearance.lockShield"
	KeyFontInterface   = "appearance.fonts.interface"
	KeyFontDocument    = "appearance.fonts.document"
	KeyFontMonospace   = "appearance.fonts.monospace"
	KeyFontTitlebar    = "appearance.fonts.titlebar"
	KeyFontTerminal    = "appearance.fonts.terminal"
	KeyTextScaling     = "appearance.fonts.textScaling"
	KeyFontAA          = "appearance.fonts.antialiasing"
	KeyFontHinting     = "appearance.fonts.hinting"
	KeyFontRGBA        = "appearance.fonts.rgbaOrder"
	KeyFontLCD         = "appearance.fonts.lcdfilter"
	KeyIconTheme       = "appearance.iconTheme"
	KeyCursorTheme     = "appearance.cursorTheme"
	KeyColorScheme     = "appearance.colorScheme"
	KeyAccentColor     = "appearance.accentColor"
	KeyPalette         = "appearance.palette"
	KeyFavorites       = "shell.favorites"
	KeyBlankAfter      = "session.idle.blankAfter"
	KeyIdleAction      = "session.idle.action"
	KeyIdleAfter       = "session.idle.after"
	KeyIdleBatteryAct  = "session.idle.batteryAction"
	KeyIdleBatteryAft  = "session.idle.batteryAfter"
	KeyIdleDim         = "session.idle.dim"
	KeyDimBrightness   = "session.idle.dimBrightness"
	KeyLockEnabled     = "session.lock.enabled"
	KeyLockDelay       = "session.lock.delay"
	KeyDockPosition    = "dock.position"
	KeyDockBehavior    = "dock.behavior"
	KeyDockIconSize    = "dock.iconSize"
	KeyDockShowTrash   = "dock.showTrash"
	KeySoftwareUpdates = "software.updates"
	KeyHotCorners      = "shell.hotCorners"
	KeyWelcomeTour     = "shell.welcomeTour"
	KeyWorkspaces      = "shell.workspaces"
	KeyAppFolders      = "shell.appFolders"
	KeyClockWeekday    = "clock.showWeekday"
	KeyClockFormat     = "clock.format"
	KeyNumLock         = "keyboard.numLock"
	KeyTerminal        = "keyboard.terminal"
	KeyXKBOptions      = "keyboard.xkbOptions"
	KeyKeyboardLayouts = "keyboard.layouts"
	KeyProxyMode       = "network.proxy.mode"
	KeyProxyAutoconfig = "network.proxy.autoconfigUrl"
	KeyProxyIgnore     = "network.proxy.ignoreHosts"
	KeyProxySame       = "network.proxy.useSameProxy"
	KeyProxyHTTP       = "network.proxy.http"
	KeyProxyHTTPS      = "network.proxy.https"
	KeyProxyFTP        = "network.proxy.ftp"
	KeyProxySOCKS      = "network.proxy.socks"

	KeySearchExternal = "search.external"
	KeySearchDisabled = "search.disabled"

	KeyNotifBanners    = "notifications.banners"
	KeyNotifLockScreen = "notifications.onLockScreen"

	KeyPrivacyRememberRecent = "privacy.rememberRecentFiles"
	KeyPrivacyRecentMaxAge   = "privacy.recentFilesMaxAge"
	KeyPrivacyRememberApp    = "privacy.rememberAppUsage"
	KeyPrivacyOldTempFiles   = "privacy.removeOldTempFiles"
	KeyPrivacyOldTrashFiles  = "privacy.removeOldTrashFiles"
	KeyPrivacyOldFilesAge    = "privacy.oldFilesAge"
	KeyPrivacyReportProblems = "privacy.reportTechnicalProblems"
	KeyPrivacyUsageStats     = "privacy.sendSoftwareUsageStats"
	KeyPrivacyCamera         = "privacy.disableCamera"
	KeyPrivacyMicrophone     = "privacy.disableMicrophone"
	KeyPrivacySoundOutput    = "privacy.disableSoundOutput"
	KeyPrivacyHideIdentity   = "privacy.hideIdentity"
	KeyPrivacyScreen         = "privacy.privacyScreen"
	KeyPowerButton           = "power.buttonAction"
	KeyDevTools              = "shell.developmentTools"
	KeyShowUserList          = "login.showUserList"
	KeyBanner                = "login.banner"

	// The GNOME lockdown keys. Setting one to true disables the capability;
	// locking it makes the choice read-only. The names mirror the dconf keys so
	// an administrator can map them to the GNOME system administration guide.
	KeyLockdownDisablePrinting            = "lockdown.disablePrinting"
	KeyLockdownDisablePrintSetup          = "lockdown.disablePrintSetup"
	KeyLockdownDisableSaveToDisk          = "lockdown.disableSaveToDisk"
	KeyLockdownDisableLogOut              = "lockdown.disableLogOut"
	KeyLockdownDisableUserSwitching       = "lockdown.disableUserSwitching"
	KeyLockdownDisableApplicationHandlers = "lockdown.disableApplicationHandlers"
	KeyLockdownDisableCommandLine         = "lockdown.disableCommandLine"
	KeyLockdownDisableShowPassword        = "lockdown.disableShowPassword"
	KeyLockdownDisableLockScreen          = "lockdown.disableLockScreen"
	KeyLockdownRemovableStorageReadOnly   = "lockdown.mountRemovableStorageReadOnly"
	KeyLockdownDisableUserAdministration  = "lockdown.disableUserAdministration"
)

// extensionKeyPrefix names one extension decision in the composed settings. Each
// decision is its own scalar, keyed by the UUID it decides about, so the layer
// rules apply unchanged: the highest layer wins and one layer cannot decide twice
// (docs/adr/0017).
const extensionKeyPrefix = "shell.extension."

// ExtensionSetting is the semantic setting key of one extension decision.
func ExtensionSetting(uuid string) string { return extensionKeyPrefix + uuid }

// KeyExtensions names the list of extension decisions. Each decision composes under
// its own key (ExtensionSetting); this is the name a lock uses, because the plan
// writes one dconf key for the whole list (docs/adr/0017).
const KeyExtensions = "shell.extensions"

// idleActions are the values GNOME's power plugin accepts for an idle machine.
// "logout" is how a workstation ends a session that nobody is using.
var idleActions = []string{"blank", "suspend", "shutdown", "hibernate", "interactive", "nothing", "logout"}

// powerButtonActions are the values GNOME's power plugin accepts.
var powerButtonActions = []string{"nothing", "suspend", "hibernate", "interactive"}

// fontEnum are the values fontconfig and Xsettings understand for each rendering
// setting.
var fontEnum = map[string][]string{
	KeyFontAA:      {"none", "grayscale", "rgba"},
	KeyFontHinting: {"none", "slight", "medium", "full"},
	KeyFontRGBA:    {"rgb", "bgr", "vrgb", "vbgr"},
	KeyFontLCD:     {"none", "default", "light", "legacy"},
}

var knownKeys = map[string]bool{
	KeyButtons: true, KeyButtonPlacement: true, KeyWallpaper: true, KeyLoginLogo: true,
	KeyLockShield:    true,
	KeyFontInterface: true, KeyFontDocument: true, KeyFontMonospace: true,
	KeyFontTitlebar: true, KeyFontTerminal: true, KeyTextScaling: true,
	KeyFontAA: true, KeyFontHinting: true, KeyFontRGBA: true, KeyFontLCD: true, KeyIconTheme: true,
	KeyCursorTheme: true, KeyFavorites: true, KeyBlankAfter: true, KeyIdleDim: true,
	KeyIdleAction: true, KeyIdleAfter: true, KeyIdleBatteryAct: true, KeyIdleBatteryAft: true,
	KeyDimBrightness: true, KeyLockEnabled: true,
	KeyLockDelay: true, KeyDockPosition: true, KeyDockBehavior: true,
	KeyDockIconSize: true, KeyDockShowTrash: true, KeyColorScheme: true, KeyAccentColor: true,
	KeySoftwareUpdates: true, KeyHotCorners: true, KeyWelcomeTour: true, KeyWorkspaces: true, KeyAppFolders: true, KeyClockWeekday: true, KeyClockFormat: true, KeyNumLock: true, KeyTerminal: true, KeyXKBOptions: true, KeyKeyboardLayouts: true,
	KeyProxyMode: true, KeyProxyAutoconfig: true, KeyProxyIgnore: true, KeyProxySame: true,
	KeyProxyHTTP: true, KeyProxyHTTPS: true, KeyProxyFTP: true, KeyProxySOCKS: true,
	KeyPrivacyRememberRecent: true, KeyPrivacyRecentMaxAge: true, KeyPrivacyRememberApp: true,
	KeyPrivacyOldTempFiles: true, KeyPrivacyOldTrashFiles: true, KeyPrivacyOldFilesAge: true,
	KeyPrivacyReportProblems: true, KeyPrivacyUsageStats: true, KeyPrivacyCamera: true,
	KeyPrivacyMicrophone: true, KeyPrivacySoundOutput: true, KeyPrivacyHideIdentity: true,
	KeyPrivacyScreen:  true,
	KeySearchExternal: true, KeySearchDisabled: true,
	KeyNotifBanners: true, KeyNotifLockScreen: true,
	KeyPowerButton: true, KeyExtensions: true, KeyShowUserList: true, KeyBanner: true,
	KeyDevTools:                true,
	KeyLockdownDisablePrinting: true, KeyLockdownDisablePrintSetup: true,
	KeyLockdownDisableSaveToDisk: true, KeyLockdownDisableLogOut: true,
	KeyLockdownDisableUserSwitching: true, KeyLockdownDisableApplicationHandlers: true,
	KeyLockdownDisableCommandLine: true, KeyLockdownDisableShowPassword: true,
	KeyLockdownDisableLockScreen: true, KeyLockdownRemovableStorageReadOnly: true,
	KeyLockdownDisableUserAdministration: true,
}

// Enumerations of gsettings-desktop-schemas 47.1 (GDesktopColorScheme, GDesktopAccentColor).
var (
	colorSchemes = []string{"default", "prefer-dark", "prefer-light"}
	accentColors = []string{"blue", "teal", "green", "yellow", "orange", "red", "pink", "purple", "slate"}
	updateModes  = []string{"automatic", "manual", "disabled"}
	clockFormats = []string{"24h", "12h"}
	xkbOptions   = []string{
		"caps:none", "compose:caps", "compose:menu", "compose:ralt",
		"compose:rctrl", "ctrl:nocaps", "grp:alt_shift_toggle",
	}
)

// MaxWorkspaces is mutter's limit for num-workspaces.
const MaxWorkspaces = 36

// GnomeProfileSpec is the public GnomeProfile spec.
type GnomeProfileSpec struct {
	Defaults Settings `json:"defaults"`
	Locks    []string `json:"locks,omitempty"`
}

type Settings struct {
	Windows       *Windows       `json:"windows,omitempty"`
	Applications  *Applications  `json:"applications,omitempty"`
	Appearance    *Appearance    `json:"appearance,omitempty"`
	Shell         *Shell         `json:"shell,omitempty"`
	Session       *Session       `json:"session,omitempty"`
	Dock          *Dock          `json:"dock,omitempty"`
	Software      *Software      `json:"software,omitempty"`
	Clock         *Clock         `json:"clock,omitempty"`
	Keyboard      *Keyboard      `json:"keyboard,omitempty"`
	Power         *Power         `json:"power,omitempty"`
	Lockdown      *Lockdown      `json:"lockdown,omitempty"`
	Login         *Login         `json:"login,omitempty"`
	Network       *Network       `json:"network,omitempty"`
	Privacy       *Privacy       `json:"privacy,omitempty"`
	Search        *Search        `json:"search,omitempty"`
	Notifications *Notifications `json:"notifications,omitempty"`
}

// Search is the shell's search surface: whether third-party providers may answer,
// and which providers do not. A provider is named by its desktop id, which is what
// GNOME lists in its own settings.
type Search struct {
	External *bool    `json:"external,omitempty"`
	Disabled []string `json:"disabled,omitempty"`
}

// Notifications is what the desktop shows and where. A banner with a subject line
// on a locked screen is a leak in an unattended office, so both are decisions.
type Notifications struct {
	Banners      *bool `json:"banners,omitempty"`
	OnLockScreen *bool `json:"onLockScreen,omitempty"`
}

// Network holds the settings of the machine's network, as far as an image owns
// them. A connection profile is not here: it names a network and carries secrets,
// which belong to provisioning and to the fleet (docs/architecture.md).
type Network struct {
	Proxy *Proxy `json:"proxy,omitempty"`
}

// Proxy is the system proxy. The mode decides what the rest means: none ignores
// everything, auto reads a PAC file, and manual uses the servers. A password is
// deliberately absent: it is a secret, and the image would publish it.
type Proxy struct {
	Mode          string       `json:"mode"`
	AutoconfigURL string       `json:"autoconfigUrl,omitempty"`
	IgnoreHosts   []string     `json:"ignoreHosts,omitempty"`
	UseSameProxy  *bool        `json:"useSameProxy,omitempty"`
	HTTP          *ProxyServer `json:"http,omitempty"`
	HTTPS         *ProxyServer `json:"https,omitempty"`
	FTP           *ProxyServer `json:"ftp,omitempty"`
	SOCKS         *ProxyServer `json:"socks,omitempty"`
}

// ProxyServer is one proxy endpoint; port 0 is not a port.
type ProxyServer struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Privacy is what the desktop remembers and reports. GNOME's defaults are already
// modest; an organization that answers a compliance questionnaire needs to say so
// in the image instead of in a checklist.
type Privacy struct {
	RememberRecentFiles     *bool   `json:"rememberRecentFiles,omitempty"`
	RecentFilesMaxAge       *int    `json:"recentFilesMaxAge,omitempty"`
	RememberAppUsage        *bool   `json:"rememberAppUsage,omitempty"`
	RemoveOldTempFiles      *bool   `json:"removeOldTempFiles,omitempty"`
	RemoveOldTrashFiles     *bool   `json:"removeOldTrashFiles,omitempty"`
	OldFilesAge             *uint32 `json:"oldFilesAge,omitempty"`
	ReportTechnicalProblems *bool   `json:"reportTechnicalProblems,omitempty"`
	SendSoftwareUsageStats  *bool   `json:"sendSoftwareUsageStats,omitempty"`
	DisableCamera           *bool   `json:"disableCamera,omitempty"`
	DisableMicrophone       *bool   `json:"disableMicrophone,omitempty"`
	DisableSoundOutput      *bool   `json:"disableSoundOutput,omitempty"`
	HideIdentity            *bool   `json:"hideIdentity,omitempty"`
	PrivacyScreen           *bool   `json:"privacyScreen,omitempty"`
}

// Login is the greeter: the user list and the text banner. Both lower to the
// database GDM reads, which the platform declares with gnome.loginScreen, so a
// platform without a greeter fails composition instead of writing a key nobody
// reads. A banner with no text turns the message off; leaving the field out
// changes nothing.
type Login struct {
	ShowUserList *bool   `json:"showUserList,omitempty"`
	Banner       *string `json:"banner,omitempty"`
}

// Lockdown is the set of GNOME capabilities a managed profile can turn off.
// Every field maps to one org.gnome.desktop.lockdown key; a nil field leaves
// the key alone. Setting a field and listing its semantic key under locks is
// what makes the value read-only.
type Lockdown struct {
	DisablePrinting               *bool `json:"disablePrinting,omitempty"`
	DisablePrintSetup             *bool `json:"disablePrintSetup,omitempty"`
	DisableSaveToDisk             *bool `json:"disableSaveToDisk,omitempty"`
	DisableLogOut                 *bool `json:"disableLogOut,omitempty"`
	DisableUserSwitching          *bool `json:"disableUserSwitching,omitempty"`
	DisableApplicationHandlers    *bool `json:"disableApplicationHandlers,omitempty"`
	DisableCommandLine            *bool `json:"disableCommandLine,omitempty"`
	DisableShowPassword           *bool `json:"disableShowPassword,omitempty"`
	DisableLockScreen             *bool `json:"disableLockScreen,omitempty"`
	MountRemovableStorageReadOnly *bool `json:"mountRemovableStorageReadOnly,omitempty"`
	DisableUserAdministration     *bool `json:"disableUserAdministration,omitempty"`
}

// Power is the hardware power button's action.
type Power struct {
	ButtonAction string `json:"buttonAction,omitempty"`
}

type Windows struct {
	Buttons         []string `json:"buttons,omitempty"`
	ButtonPlacement string   `json:"buttonPlacement,omitempty"`
}

// Applications assigns the desktop entries that handle the common document
// roles. The lowering owns the role-to-MIME table; a profile names entries only.
type Applications struct {
	Browser     string `json:"browser,omitempty"`
	FileManager string `json:"fileManager,omitempty"`
	TextEditor  string `json:"textEditor,omitempty"`
	ImageViewer string `json:"imageViewer,omitempty"`
	VideoPlayer string `json:"videoPlayer,omitempty"`
	PDFViewer   string `json:"pdfViewer,omitempty"`
}

type Appearance struct {
	Wallpaper   *Wallpaper `json:"wallpaper,omitempty"`
	LoginLogo   string     `json:"loginLogo,omitempty"`
	LockShield  string     `json:"lockShield,omitempty"`
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
	// Titlebar is the window title font. Omitting it keeps GNOME's default, which
	// is to use the interface font.
	Titlebar *Font `json:"titlebar,omitempty"`
	// Terminal is the font of the terminal the platform ships. A platform that
	// declares no terminal cannot carry it, and composition says so.
	Terminal *Font `json:"terminal,omitempty"`
	// TextScaling is GNOME's text-scaling-factor: a multiplier over every size,
	// not a size. 1.0 is the default, and the range is GNOME's.
	TextScaling *float64 `json:"textScaling,omitempty"`
	// Rendering, applied by fontconfig to every application, mirrored in the
	// interface keys GNOME 47 reads and in the Xsettings keys X11 applications use.
	Antialiasing string `json:"antialiasing,omitempty"`
	Hinting      string `json:"hinting,omitempty"`
	RGBAOrder    string `json:"rgbaOrder,omitempty"`
	LCDFilter    string `json:"lcdfilter,omitempty"`
}

type Font struct {
	Family string  `json:"family"`
	Size   float64 `json:"size"`
}

type Shell struct {
	Favorites        []string    `json:"favorites,omitempty"`
	DevelopmentTools *bool       `json:"developmentTools,omitempty"`
	HotCorners       *bool       `json:"hotCorners,omitempty"`
	WelcomeTour      *bool       `json:"welcomeTour,omitempty"`
	Workspaces       any         `json:"workspaces,omitempty"`
	AppFolders       []AppFolder `json:"appFolders,omitempty"`
	Extensions       []Extension `json:"extensions,omitempty"`
}

// Extension turns one platform-declared GNOME Shell extension on or off. Naming a
// UUID is the decision: a UUID a profile does not name keeps whatever the platform
// decided (docs/adr/0017).
type Extension struct {
	UUID    string `json:"uuid"`
	Enabled bool   `json:"enabled"`
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
	NumLock  *bool            `json:"numLock,omitempty"`
	Terminal string           `json:"terminal,omitempty"`
	Layouts  []KeyboardLayout `json:"layouts,omitempty"`
	// XKBOptions are the xkb options, independent of the layouts: a layout says
	// which keys a board has, an option changes what some of them do.
	XKBOptions []string `json:"xkbOptions,omitempty"`
}

// KeyboardLayout is one xkb layout a session switches between. The first one is
// the layout a session starts in; the order is the switch order.
type KeyboardLayout struct {
	Layout  string `json:"layout"`
	Variant string `json:"variant,omitempty"`
}

// Workspaces is the composed workspace mode; Count is 0 for dynamic workspaces.
type Workspaces struct {
	Count int `json:"count"`
}

type Session struct {
	Idle *Idle `json:"idle,omitempty"`
	Lock *Lock `json:"lock,omitempty"`
}

// Idle is what the machine does with an idle session: blank after a while, and
// dim the screen before it blanks. Brightness only applies while Dim is on; it is
// a percentage of the normal level, so 30 is dimmer than 60.
type Idle struct {
	BlankAfter    string `json:"blankAfter,omitempty"`
	Dim           *bool  `json:"dim,omitempty"`
	DimBrightness *int   `json:"dimBrightness,omitempty"`
	// Action and After are what an idle machine does on AC and when: suspend,
	// hibernate, shut down, log out, blank, ask, or nothing. They are declared
	// together, because a timeout with no action and an action with no timeout are
	// both a policy that does nothing. BatteryAction and BatteryAfter are the same
	// pair on battery.
	Action        string `json:"action,omitempty"`
	After         string `json:"after,omitempty"`
	BatteryAction string `json:"batteryAction,omitempty"`
	BatteryAfter  string `json:"batteryAfter,omitempty"`
}

type Lock struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Delay   string `json:"delay,omitempty"`
}

// Dock holds the settings DeskOS models for Dash to Dock. Whether the dock runs is
// not here: the dock is enabled by deciding about its extension, like any other
// (docs/adr/0017).
type Dock struct {
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
	fontFamilyRE    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _-]*$`)
	themeRE         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._+-]*$`)
	desktopIDRE     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.desktop$`)
	assetNameRE     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.(png|jpg|jpeg|svg)$`)
	logoNameRE      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.(png|svg)$`)
	folderIDRE      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	categoryRE      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	folderNameRE    = regexp.MustCompile(`^[^\x00-\x1f\x7f]+$`)
	extensionUUIDRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*$`)
	// xkb names are lower case: a layout is us, es or latam, and a variant is
	// intl or nodeadkeys. An unknown name fails here, not in a session.
	xkbLayoutRE  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	xkbVariantRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)
	// A proxy host is a name or an address, never a URL: the scheme is the key.
	proxyHostRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$|^[0-9A-Fa-f:]+$`)
	proxyURLRE  = regexp.MustCompile(`^https?://[^[:space:]]+$`)
)

// Providers returns the desktop.deskos.org providers.
func Providers() []registry.Provider { return []registry.Provider{gnomeProfile{}, themeKind{}} }

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

	if apps := s.Applications; apps != nil {
		for _, app := range []struct{ key, id string }{
			{KeyAppBrowser, apps.Browser},
			{KeyAppFileManager, apps.FileManager},
			{KeyAppTextEditor, apps.TextEditor},
			{KeyAppImageViewer, apps.ImageViewer},
			{KeyAppVideoPlayer, apps.VideoPlayer},
			{KeyAppPDFViewer, apps.PDFViewer},
		} {
			if app.id == "" {
				continue
			}
			if !desktopIDRE.MatchString(app.id) {
				bad("%s: %q is not a desktop file id (for example google-chrome.desktop)", app.key, app.id)
				continue
			}
			set(app.key, app.id, app.id)
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
		if a.LockShield != "" {
			as, err := assets.Read(res, a.LockShield)
			switch {
			case err != nil:
				bad("%s: %v", KeyLockShield, err)
			case !assetNameRE.MatchString(path.Base(as.Path)):
				bad("%s: %s must be a .png, .jpg, .jpeg or .svg file with a simple name", KeyLockShield, as.Path)
			default:
				set(KeyLockShield, Asset{Path: as.Path, SHA256: as.SHA256, File: as.File}, as.Path)
			}
		}
		if f := a.Fonts; f != nil {
			for key, v := range map[string]string{
				KeyFontAA: f.Antialiasing, KeyFontHinting: f.Hinting,
				KeyFontRGBA: f.RGBAOrder, KeyFontLCD: f.LCDFilter,
			} {
				if v == "" {
					continue
				}
				if !slices.Contains(fontEnum[key], v) {
					bad("%s: expected %s, got %q", key, strings.Join(fontEnum[key], ", "), v)
					continue
				}
				set(key, v, v)
			}
			for key, font := range map[string]*Font{
				KeyFontInterface: f.Interface, KeyFontDocument: f.Document, KeyFontMonospace: f.Monospace,
				KeyFontTitlebar: f.Titlebar, KeyFontTerminal: f.Terminal,
			} {
				if font == nil {
					continue
				}
				if !fontFamilyRE.MatchString(font.Family) || font.Size < 6 || font.Size > 72 {
					bad("%s: font needs a family name and a size between 6 and 72", key)
					continue
				}
				set(key, *font, fontName(*font))
			}
			if f.TextScaling != nil {
				if *f.TextScaling < 0.5 || *f.TextScaling > 3 {
					bad("%s: text scaling must be between 0.5 and 3.0", KeyTextScaling)
				} else {
					set(KeyTextScaling, *f.TextScaling, strconv.FormatFloat(*f.TextScaling, 'g', -1, 64))
				}
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

	if lg := s.Login; lg != nil {
		if lg.ShowUserList != nil {
			set(KeyShowUserList, *lg.ShowUserList, strconv.FormatBool(*lg.ShowUserList))
		}
		if lg.Banner != nil {
			// GNOME has no length limit and folds a long message into two columns;
			// a NUL byte is the one thing it cannot carry.
			if strings.ContainsRune(*lg.Banner, 0) {
				bad("%s: the banner cannot contain a NUL byte", KeyBanner)
			} else {
				set(KeyBanner, *lg.Banner, *lg.Banner)
			}
		}
	}

	if n := s.Network; n != nil && n.Proxy != nil {
		px := n.Proxy
		switch px.Mode {
		case "none", "auto", "manual":
		default:
			bad("%s: expected none, auto or manual, got %q", KeyProxyMode, px.Mode)
		}
		switch {
		case px.Mode == "auto" && px.AutoconfigURL == "":
			bad("%s: mode auto needs autoconfigUrl (the PAC file)", KeyProxyAutoconfig)
		case px.Mode != "auto" && px.AutoconfigURL != "":
			bad("%s: autoconfigUrl is only read when mode is auto", KeyProxyAutoconfig)
		case px.AutoconfigURL != "" && !proxyURLRE.MatchString(px.AutoconfigURL):
			bad("%s: %q is not an http or https URL", KeyProxyAutoconfig, px.AutoconfigURL)
		}
		servers := map[string]*ProxyServer{
			KeyProxyHTTP: px.HTTP, KeyProxyHTTPS: px.HTTPS, KeyProxyFTP: px.FTP, KeyProxySOCKS: px.SOCKS,
		}
		declared := 0
		for key, srv := range servers {
			if srv == nil {
				continue
			}
			declared++
			if !proxyHostRE.MatchString(srv.Host) {
				bad("%s: %q is not a host name or address", key, srv.Host)
			}
			if srv.Port < 1 || srv.Port > 65535 {
				bad("%s: port %d is not between 1 and 65535", key, srv.Port)
			}
			set(key, *srv, srv.Host+":"+strconv.Itoa(srv.Port))
		}
		if px.Mode == "manual" && declared == 0 {
			bad("%s: mode manual needs at least one proxy server", KeyProxyMode)
		}
		if px.Mode != "manual" && declared > 0 {
			bad("%s: proxy servers are only read when mode is manual", KeyProxyMode)
		}
		for i, h := range px.IgnoreHosts {
			if strings.TrimSpace(h) == "" || strings.ContainsAny(h, " \t") {
				bad("%s[%d]: a host pattern is one token, for example *.example.org", KeyProxyIgnore, i)
			}
		}
		set(KeyProxyMode, px.Mode, px.Mode)
		if px.AutoconfigURL != "" {
			set(KeyProxyAutoconfig, px.AutoconfigURL, px.AutoconfigURL)
		}
		if px.IgnoreHosts != nil {
			set(KeyProxyIgnore, px.IgnoreHosts, strings.Join(px.IgnoreHosts, ", "))
		}
		if px.UseSameProxy != nil {
			set(KeyProxySame, *px.UseSameProxy, strconv.FormatBool(*px.UseSameProxy))
		}
	}

	if s.Search != nil {
		if ext := s.Search.External; ext != nil {
			set(KeySearchExternal, *ext, strconv.FormatBool(*ext))
		}
		if s.Search.Disabled != nil {
			seen := map[string]bool{}
			for _, id := range s.Search.Disabled {
				if !desktopIDRE.MatchString(id) {
					bad("%s: %q is not a desktop file id (for example org.gnome.Nautilus.desktop)", KeySearchDisabled, id)
				}
				if seen[id] {
					bad("%s: %q is listed twice", KeySearchDisabled, id)
				}
				seen[id] = true
			}
			set(KeySearchDisabled, s.Search.Disabled, strings.Join(s.Search.Disabled, ", "))
		}
	}

	if n := s.Notifications; n != nil {
		if b := n.Banners; b != nil {
			set(KeyNotifBanners, *b, strconv.FormatBool(*b))
		}
		if l := n.OnLockScreen; l != nil {
			set(KeyNotifLockScreen, *l, strconv.FormatBool(*l))
		}
	}

	if p := s.Privacy; p != nil {
		for key, v := range map[string]*bool{
			KeyPrivacyRememberRecent: p.RememberRecentFiles, KeyPrivacyRememberApp: p.RememberAppUsage,
			KeyPrivacyOldTempFiles: p.RemoveOldTempFiles, KeyPrivacyOldTrashFiles: p.RemoveOldTrashFiles,
			KeyPrivacyReportProblems: p.ReportTechnicalProblems, KeyPrivacyUsageStats: p.SendSoftwareUsageStats,
			KeyPrivacyCamera: p.DisableCamera, KeyPrivacyMicrophone: p.DisableMicrophone,
			KeyPrivacySoundOutput: p.DisableSoundOutput, KeyPrivacyHideIdentity: p.HideIdentity,
			KeyPrivacyScreen: p.PrivacyScreen,
		} {
			if v != nil {
				set(key, *v, strconv.FormatBool(*v))
			}
		}
		if p.RecentFilesMaxAge != nil {
			if *p.RecentFilesMaxAge < -1 {
				bad("%s: -1 keeps every recent file, 0 keeps none, a positive number is days", KeyPrivacyRecentMaxAge)
			} else {
				set(KeyPrivacyRecentMaxAge, *p.RecentFilesMaxAge, strconv.Itoa(*p.RecentFilesMaxAge))
			}
		}
		if p.OldFilesAge != nil {
			if *p.OldFilesAge < 1 || *p.OldFilesAge > 3650 {
				bad("%s: a number of days between 1 and 3650", KeyPrivacyOldFilesAge)
			} else {
				set(KeyPrivacyOldFilesAge, *p.OldFilesAge, strconv.FormatUint(uint64(*p.OldFilesAge), 10))
			}
		}
	}

	if sh := s.Shell; sh != nil {
		if dt := sh.DevelopmentTools; dt != nil {
			// GNOME Shell's Looking Glass and the integrated debugger.
			set(KeyDevTools, *dt, strconv.FormatBool(*dt))
		}
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
		if sh.WelcomeTour != nil {
			set(KeyWelcomeTour, *sh.WelcomeTour, strconv.FormatBool(*sh.WelcomeTour))
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
		if sh.Extensions != nil {
			// One scalar per UUID, so a profile's decision composes like every other
			// setting: the highest layer wins, and naming one UUID twice in the same
			// list is ambiguous rather than a silent last-one-wins (docs/adr/0017).
			seenUUID := map[string]bool{}
			for _, e := range sh.Extensions {
				if !extensionUUIDRE.MatchString(e.UUID) {
					bad("%s: %q is not an extension uuid (name@domain)", KeyExtensions, e.UUID)
				}
				if seenUUID[e.UUID] {
					bad("%s: %q is decided twice; leave the UUID out to inherit the platform's decision", KeyExtensions, e.UUID)
				}
				seenUUID[e.UUID] = true
				state := "enabled"
				if !e.Enabled {
					state = "disabled"
				}
				set(ExtensionSetting(e.UUID), e.Enabled, e.UUID+" "+state)
			}
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
		if k.Layouts != nil {
			if len(k.Layouts) == 0 {
				bad("%s: at least one layout is required", KeyKeyboardLayouts)
			}
			seen := map[string]bool{}
			names := make([]string, 0, len(k.Layouts))
			for _, l := range k.Layouts {
				if !xkbLayoutRE.MatchString(l.Layout) {
					bad("%s: %q is not an xkb layout name (for example us, es, latam)", KeyKeyboardLayouts, l.Layout)
					continue
				}
				if l.Variant != "" && !xkbVariantRE.MatchString(l.Variant) {
					bad("%s: %q is not an xkb variant name (for example intl, nodeadkeys)", KeyKeyboardLayouts, l.Variant)
					continue
				}
				id := l.Layout
				if l.Variant != "" {
					id += "+" + l.Variant
				}
				if seen[id] {
					bad("%s: %q is listed twice", KeyKeyboardLayouts, id)
					continue
				}
				seen[id] = true
				names = append(names, id)
			}
			set(KeyKeyboardLayouts, k.Layouts, strings.Join(names, ", "))
		}
		if k.XKBOptions != nil {
			if len(k.XKBOptions) == 0 {
				bad("%s: at least one option is required", KeyXKBOptions)
			}
			seen := map[string]bool{}
			for _, opt := range k.XKBOptions {
				if !slices.Contains(xkbOptions, opt) {
					bad("%s: expected %s, got %q", KeyXKBOptions, strings.Join(xkbOptions, ", "), opt)
				}
				if seen[opt] {
					bad("%s: %q is listed twice", KeyXKBOptions, opt)
				}
				seen[opt] = true
			}
			set(KeyXKBOptions, k.XKBOptions, strings.Join(k.XKBOptions, ", "))
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
		if se.Idle != nil {
			for _, pair := range []struct {
				actionKey, afterKey, action, after string
			}{
				{KeyIdleAction, KeyIdleAfter, se.Idle.Action, se.Idle.After},
				{KeyIdleBatteryAct, KeyIdleBatteryAft, se.Idle.BatteryAction, se.Idle.BatteryAfter},
			} {
				if pair.action == "" && pair.after == "" {
					continue
				}
				if pair.action == "" || pair.after == "" {
					bad("%s and %s are declared together: a timeout without an action, or an action without a timeout, does nothing",
						pair.actionKey, pair.afterKey)
					continue
				}
				if !slices.Contains(idleActions, pair.action) {
					bad("%s: expected %s, got %q", pair.actionKey, strings.Join(idleActions, ", "), pair.action)
					continue
				}
				secs, err := parseDuration(pair.after, true)
				if err != nil {
					bad("%s: %v", pair.afterKey, err)
					continue
				}
				if secs == 0 && pair.action != "nothing" {
					bad("%s: %q needs a timeout; use \"never\" only with nothing", pair.afterKey, pair.action)
					continue
				}
				set(pair.actionKey, pair.action, pair.action)
				set(pair.afterKey, secs, pair.after)
			}
			if d := se.Idle.Dim; d != nil {
				set(KeyIdleDim, *d, strconv.FormatBool(*d))
			}
			if b := se.Idle.DimBrightness; b != nil {
				// The schema bounds it to 1..100: it is a percentage of the level.
				set(KeyDimBrightness, *b, strconv.Itoa(*b))
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

	if pw := s.Power; pw != nil && pw.ButtonAction != "" {
		if !slices.Contains(powerButtonActions, pw.ButtonAction) {
			bad("%s: expected %s, got %q", KeyPowerButton, strings.Join(powerButtonActions, ", "), pw.ButtonAction)
		}
		set(KeyPowerButton, pw.ButtonAction, pw.ButtonAction)
	}

	if ld := s.Lockdown; ld != nil {
		for _, b := range []struct {
			key string
			val *bool
		}{
			{KeyLockdownDisablePrinting, ld.DisablePrinting},
			{KeyLockdownDisablePrintSetup, ld.DisablePrintSetup},
			{KeyLockdownDisableSaveToDisk, ld.DisableSaveToDisk},
			{KeyLockdownDisableLogOut, ld.DisableLogOut},
			{KeyLockdownDisableUserSwitching, ld.DisableUserSwitching},
			{KeyLockdownDisableApplicationHandlers, ld.DisableApplicationHandlers},
			{KeyLockdownDisableCommandLine, ld.DisableCommandLine},
			{KeyLockdownDisableShowPassword, ld.DisableShowPassword},
			{KeyLockdownDisableLockScreen, ld.DisableLockScreen},
			{KeyLockdownRemovableStorageReadOnly, ld.MountRemovableStorageReadOnly},
			{KeyLockdownDisableUserAdministration, ld.DisableUserAdministration},
		} {
			if b.val != nil {
				set(b.key, *b.val, strconv.FormatBool(*b.val))
			}
		}
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
