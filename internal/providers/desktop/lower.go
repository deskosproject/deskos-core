package desktop

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
)

// BackgroundsDir is the image-owned location of wallpaper assets.
const BackgroundsDir = "/usr/share/deskos/backgrounds"

// BrandingDir is the image-owned location of logo assets.
const BrandingDir = "/usr/share/deskos/branding"

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
	l := &lowering{c: c, p: p, db: &plan.DconfDatabase{Name: pl.Gnome.DconfDatabase}, bySett: map[string][]string{}}
	get := func(key string) (compose.Scalar, bool) { return c.Result.Scalar(DomainSettings, key) }

	// Window controls: two semantic keys produce one GNOME key.
	if b, ok := get(KeyButtons); ok {
		buttons := strings.Join(b.Value.([]string), ",")
		layout := "appmenu:" + buttons
		prov := b.Provenance
		const key = "/org/gnome/desktop/wm/preferences/button-layout"
		if pos, ok := get(KeyButtonPlacement); ok {
			if pos.Value.(string) == "left" {
				layout = buttons + ":appmenu"
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
	if v, ok := get(KeyBlankAfter); ok {
		l.set(KeyBlankAfter, "/org/gnome/desktop/session/idle-delay", "u", gvUint32(v.Value.(Seconds)), v.Provenance)
	}
	if v, ok := get(KeyLockEnabled); ok {
		l.set(KeyLockEnabled, "/org/gnome/desktop/screensaver/lock-enabled", "b", strconv.FormatBool(v.Value.(bool)), v.Provenance)
	}
	if v, ok := get(KeyLockDelay); ok {
		l.set(KeyLockDelay, "/org/gnome/desktop/screensaver/lock-delay", "u", gvUint32(v.Value.(Seconds)), v.Provenance)
	}

	if wp, ok := get(KeyWallpaper); ok {
		v := wp.Value.(WallpaperValue)
		light := l.installAsset(v.Light, wp.Provenance)
		dark := light
		if v.Dark != nil {
			dark = l.installAsset(*v.Dark, wp.Provenance)
		}
		l.set(KeyWallpaper, "/org/gnome/desktop/background/picture-uri", "s", gvString(light), wp.Provenance)
		l.set(KeyWallpaper, "/org/gnome/desktop/background/picture-uri-dark", "s", gvString(dark), wp.Provenance)
		l.set(KeyWallpaper, "/org/gnome/desktop/screensaver/picture-uri", "s", gvString(light), wp.Provenance)
		// GNOME Initial Setup's dconf profile skips the DeskOS database, so
		// the first-boot session gets the wallpaper as a schema default.
		for key, uri := range map[string]string{"picture-uri": light, "picture-uri-dark": dark} {
			p.Artifact.GSettings = append(p.Artifact.GSettings, plan.GSettingsVendorDefault{
				Schema: "org.gnome.desktop.background", Key: key, Type: "s", Value: gvString(uri),
				Setting: KeyWallpaper, Provenance: wp.Provenance,
			})
		}
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

	for key, gkey := range map[string]string{
		KeyFontInterface: "font-name",
		KeyFontDocument:  "document-font-name",
		KeyFontMonospace: "monospace-font-name",
	} {
		if v, ok := get(key); ok {
			l.set(key, "/org/gnome/desktop/interface/"+gkey, "s", gvString(fontName(v.Value.(Font))), v.Provenance)
		}
	}
	for key, gkey := range map[string]string{KeyIconTheme: "icon-theme", KeyCursorTheme: "cursor-theme"} {
		if v, ok := get(key); ok {
			l.set(key, "/org/gnome/desktop/interface/"+gkey, "s", gvString(v.Value.(string)), v.Provenance)
		}
	}

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

// dockOptions are settings that only exist while the dock extension runs.
var dockOptions = []string{KeyDockPosition, KeyDockBehavior, KeyDockIconSize, KeyDockShowTrash}

// lowerDock applies the dock semantics: options need an effective
// dock.enabled true; dock.enabled false at a layer masks options from lower
// layers (reported as a warning) and rejects options from its own or higher
// layers; an unset dock.enabled with options is an error.
func (l *lowering) lowerDock(get func(string) (compose.Scalar, bool)) {
	const schema = "/org/gnome/shell/extensions/dash-to-dock/"
	en, hasEn := get(KeyDockEnabled)
	enabled := hasEn && en.Value.(bool)
	var active []compose.Scalar
	for _, key := range dockOptions {
		v, ok := get(key)
		switch {
		case !ok:
		case !hasEn:
			l.errs.Add(fmt.Errorf("%s requires %s: true, which no profile sets\n  set by: %s", key, KeyDockEnabled, describe(v.Provenance)))
		case enabled:
			active = append(active, v)
		case v.Layer < en.Layer:
			l.p.Warnings = append(l.p.Warnings, fmt.Sprintf("%s from %s layer is masked: %s is false at %s layer (%s)",
				key, v.Layer, KeyDockEnabled, en.Layer, describeShort(en.Provenance)))
		default:
			l.errs.Add(fmt.Errorf("%s is set at %s layer but %s is false at %s layer; a disabled dock cannot apply it\n  set by: %s\n  disabled by: %s",
				key, v.Layer, KeyDockEnabled, en.Layer, describe(v.Provenance), describe(en.Provenance)))
		}
	}
	pl := l.c.PlatformSpec
	if !enabled {
		if hasEn {
			l.disableDock(en)
		}
		return
	}
	ext, ok := pl.GnomeExtension(DockExtension)
	if !ok {
		l.errs.Add(fmt.Errorf("%s does not provide the GNOME extension %q required by %s\n  set by: %s",
			l.c.Platform.ID(), DockExtension, KeyDockEnabled, describe(en.Provenance)))
		return
	}
	l.p.AddPackage(ext.Package, en.Provenance...)
	exts := append([]string(nil), pl.Gnome.EnabledExtensions...)
	if !contains(exts, ext.UUID) {
		exts = append(exts, ext.UUID)
	}
	l.set(KeyDockEnabled, "/org/gnome/shell/enabled-extensions", "as", gvStrings(exts), en.Provenance)
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

// disableDock handles an effective dock.enabled false. Nothing is written
// unless the platform enables the dock by default or the setting is locked;
// then enabled-extensions is set to the platform list without the dock.
func (l *lowering) disableDock(en compose.Scalar) {
	pl := l.c.PlatformSpec
	ext, ok := pl.GnomeExtension(DockExtension)
	if !ok {
		return
	}
	locked := false
	for _, m := range l.c.Result.Set(DomainLocks) {
		if m.Name == KeyDockEnabled {
			locked = true
		}
	}
	if !contains(pl.Gnome.EnabledExtensions, ext.UUID) && !locked {
		return
	}
	var exts []string
	for _, e := range pl.Gnome.EnabledExtensions {
		if e != ext.UUID {
			exts = append(exts, e)
		}
	}
	l.set(KeyDockEnabled, "/org/gnome/shell/enabled-extensions", "as", gvStrings(exts), en.Provenance)
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
