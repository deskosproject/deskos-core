// Theme is the thirteenth public kind (ADR 0013). It maps a named,
// reusable palette — the palette scheme — onto the appearance
// surfaces GNOME itself defines, so a workstation picks a look by name
// instead of repeating every setting.
package desktop

import (
	"path"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/providers/assets"
	"github.com/deskosproject/deskos-core/internal/theme"
)

var ThemeGVK = model.GVK{Group: model.GroupDesktop, Version: model.V1Alpha1, Kind: "Theme"}

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
// overrides, as written in the resource.
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

// value returns the shared palette, named after the resource.
func (p Palette) value(name string) theme.Palette {
	return theme.Palette{
		Name:       name,
		Background: p.Background, Foreground: p.Foreground,
		Black: p.Black, Red: p.Red, Green: p.Green, Yellow: p.Yellow,
		Blue: p.Blue, Magenta: p.Magenta, Cyan: p.Cyan, White: p.White,
	}
}

type themeKind struct{}

func (themeKind) GVK() model.GVK { return ThemeGVK }
func (themeKind) Schema() string { return "desktop.deskos.org/v1alpha1/theme.json" }

func (themeKind) Decode(res *model.Resource) error {
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
		if name, ok := theme.NearestAccent(spec.Accent); ok {
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
			if f.val != "" && !theme.HexColor.MatchString(f.val) {
				bad("%s.%s: expected a #RRGGBB color, got %q", KeyPalette, f.name, f.val)
				ok = false
			}
		}
		if ok {
			set(KeyPalette, p.value(res.Metadata.Name), "background="+p.Background+" foreground="+p.Foreground)
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

func (themeKind) Contribute(res *model.Resource, s *compose.Scope) error {
	d := res.Object.(*decoded)
	for _, st := range d.settings {
		s.SetScalar(DomainSettings, st.key, st.value, st.display)
	}
	return nil
}
