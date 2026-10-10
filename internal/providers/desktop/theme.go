// Theme is the thirteenth public kind (ADR 0013). It maps a named,
// reusable palette — the scheme palette themes use — onto the appearance
// surfaces GNOME itself defines, so a workstation picks a look by name
// instead of repeating every setting.
package desktop

import (
	"math"
	"path"
	"regexp"
	"strconv"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/providers/assets"
)

var ThemeGVK = model.GVK{Group: model.GroupDesktop, Version: model.V1Alpha1, Kind: "Theme"}

// accentPalette is libadwaita's nine accent colors (AdwAccentColor), the
// values org.gnome.desktop.interface accent-color accepts. A Theme's free
// #RRGGBB accent is mapped to the closest one.
var accentPalette = []struct {
	name    string
	r, g, b int
}{
	{"blue", 0x35, 0x84, 0xe4},
	{"teal", 0x21, 0x90, 0xa4},
	{"green", 0x3a, 0x94, 0x4a},
	{"yellow", 0xc8, 0x88, 0x00},
	{"orange", 0xed, 0x5b, 0x00},
	{"red", 0xe6, 0x2d, 0x42},
	{"pink", 0xd5, 0x61, 0x99},
	{"purple", 0x91, 0x41, 0xac},
	{"slate", 0x6f, 0x83, 0x96},
}

// ThemeSpec is the public Theme spec.
type ThemeSpec struct {
	Mode        string     `json:"mode,omitempty"`
	Accent      string     `json:"accent,omitempty"`
	Palette     *Palette   `json:"palette,omitempty"`
	IconTheme   string     `json:"iconTheme,omitempty"`
	CursorTheme string     `json:"cursorTheme,omitempty"`
	Fonts       *Fonts     `json:"fonts,omitempty"`
	Wallpaper   *Wallpaper `json:"wallpaper,omitempty"`
}

// Palette is the free color set behind the generated GTK and terminal
// overrides. background and foreground feed GTK; the ANSI colors, when the
// whole normal set is present, also produce a Ptyxis terminal palette.
type Palette struct {
	Background string `json:"background,omitempty"`
	Foreground string `json:"foreground,omitempty"`
	Black      string `json:"black,omitempty"`
	Red        string `json:"red,omitempty"`
	Green      string `json:"green,omitempty"`
	Yellow     string `json:"yellow,omitempty"`
	Blue       string `json:"blue,omitempty"`
	Magenta    string `json:"magenta,omitempty"`
	Cyan       string `json:"cyan,omitempty"`
	White      string `json:"white,omitempty"`
}

// PaletteValue is the composed palette; Name is the Theme's resource name and
// Accent is its free hex.
type PaletteValue struct {
	Name       string `json:"name"`
	Accent     string `json:"accent,omitempty"`
	Background string `json:"background,omitempty"`
	Foreground string `json:"foreground,omitempty"`
	Black      string `json:"black,omitempty"`
	Red        string `json:"red,omitempty"`
	Green      string `json:"green,omitempty"`
	Yellow     string `json:"yellow,omitempty"`
	Blue       string `json:"blue,omitempty"`
	Magenta    string `json:"magenta,omitempty"`
	Cyan       string `json:"cyan,omitempty"`
	White      string `json:"white,omitempty"`
}

// terminalReady reports whether the palette carries what a Ptyxis palette
// requires: a background, a foreground and the six normal ANSI colors.
func (p PaletteValue) terminalReady() bool {
	for _, c := range []string{p.Background, p.Foreground, p.Red, p.Green, p.Yellow, p.Blue, p.Magenta, p.Cyan} {
		if c == "" {
			return false
		}
	}
	return true
}

var hexColorRE = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type theme struct{}

func (theme) GVK() model.GVK { return ThemeGVK }
func (theme) Schema() string { return "desktop.deskos.org/v1alpha1/theme.json" }

func (theme) Decode(res *model.Resource) error {
	var spec ThemeSpec
	if err := model.DecodeSpec(res, &spec); err != nil {
		return err
	}
	d := &decoded{}
	var errs model.ErrorList
	bad := func(format string, a ...any) { errs.Add(model.Errorf(res, format, a...)) }
	set := func(key string, value any, display string) {
		d.settings = append(d.settings, setting{key: key, value: value, display: display})
	}

	if spec.Mode != "" {
		if spec.Mode != "dark" && spec.Mode != "light" {
			bad("%s: expected dark or light, got %q", KeyColorScheme, spec.Mode)
		} else {
			scheme := "prefer-" + spec.Mode
			set(KeyColorScheme, scheme, scheme)
		}
	}

	if spec.Accent != "" {
		if name, ok := nearestAccent(spec.Accent); ok {
			set(KeyAccentColor, name, name+" ("+spec.Accent+")")
		} else {
			bad("%s: expected a #RRGGBB color, got %q", KeyAccentColor, spec.Accent)
		}
	}

	if spec.Palette != nil {
		p := spec.Palette
		fields := []struct{ name, val string }{
			{"background", p.Background}, {"foreground", p.Foreground},
			{"black", p.Black}, {"red", p.Red}, {"green", p.Green}, {"yellow", p.Yellow},
			{"blue", p.Blue}, {"magenta", p.Magenta}, {"cyan", p.Cyan}, {"white", p.White},
		}
		ok := true
		for _, f := range fields {
			if f.val != "" && !hexColorRE.MatchString(f.val) {
				bad("%s.%s: expected a #RRGGBB color, got %q", KeyPalette, f.name, f.val)
				ok = false
			}
		}
		if ok {
			pv := PaletteValue{
				Name: res.Metadata.Name, Accent: spec.Accent,
				Background: p.Background, Foreground: p.Foreground,
				Black: p.Black, Red: p.Red, Green: p.Green, Yellow: p.Yellow,
				Blue: p.Blue, Magenta: p.Magenta, Cyan: p.Cyan, White: p.White,
			}
			set(KeyPalette, pv, "background="+pv.Background+" foreground="+pv.Foreground)
		}
	}

	for key, v := range map[string]string{KeyIconTheme: spec.IconTheme, KeyCursorTheme: spec.CursorTheme} {
		if v == "" {
			continue
		}
		if !themeRE.MatchString(v) {
			bad("%s: invalid theme name %q", key, v)
		}
		set(key, v, v)
	}

	if f := spec.Fonts; f != nil {
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

	if wp := spec.Wallpaper; wp != nil {
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
		switch {
		case wp.Light == "":
			bad("%s: light is required", KeyWallpaper)
		default:
			if light := readAsset(wp.Light); light != nil {
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
	}

	if len(d.settings) == 0 {
		bad("a theme must set at least one of mode, accent, iconTheme, cursorTheme, fonts or wallpaper")
	}
	res.Object = d
	return errs.Err()
}

func (theme) Contribute(res *model.Resource, s *compose.Scope) error {
	d := res.Object.(*decoded)
	for _, st := range d.settings {
		s.SetScalar(DomainSettings, st.key, st.value, st.display)
	}
	return nil
}

// nearestAccent maps a #RRGGBB color to the closest of GNOME's nine accents
// by hue, with a slate fallback for near-achromatic colors. Hue matches how
// people read a palette (a light green is still "green").
func nearestAccent(color string) (string, bool) {
	if len(color) != 7 || color[0] != '#' {
		return "", false
	}
	r, err1 := strconv.ParseUint(color[1:3], 16, 8)
	g, err2 := strconv.ParseUint(color[3:5], 16, 8)
	b, err3 := strconv.ParseUint(color[5:7], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return "", false
	}
	hue, sat := hueSaturation(int(r), int(g), int(b))
	if sat < 0.15 {
		return "slate", true
	}
	best, bestDist := "slate", 360.0
	for _, c := range accentPalette {
		ch, _ := hueSaturation(c.r, c.g, c.b)
		dist := math.Abs(hue - ch)
		if dist > 180 {
			dist = 360 - dist
		}
		if dist < bestDist {
			bestDist, best = dist, c.name
		}
	}
	return best, true
}

// hueSaturation returns the HSL hue in degrees and saturation in [0,1].
func hueSaturation(r, g, b int) (hue, sat float64) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	d := max - min
	if d == 0 {
		return 0, 0
	}
	if l := (max + min) / 2; l > 0.5 {
		sat = d / (2 - max - min)
	} else {
		sat = d / (max + min)
	}
	switch max {
	case rf:
		hue = 60 * math.Mod((gf-bf)/d, 6)
	case gf:
		hue = 60 * ((bf-rf)/d + 2)
	default:
		hue = 60 * ((rf-gf)/d + 4)
	}
	if hue < 0 {
		hue += 360
	}
	return hue, sat
}
