// Package system provides the system.deskos.org kinds: BootProfile, how the
// artifact boots and shuts down (graphical splash or text, quiet or verbose,
// the splash watermark), and UpdatePolicy, the unattended updates the image
// schedules. Both are artifact intent lowered to image content (kernel
// arguments, initramfs, systemd units); the compiler runs nothing on the
// machine.
package system

import (
	"bytes"
	"fmt"
	"image/png"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
	"github.com/deskosproject/deskos-core/internal/providers/assets"
	"github.com/deskosproject/deskos-core/internal/registry"
)

var BootProfileGVK = model.GVK{Group: "system.deskos.org", Version: model.V1Alpha1, Kind: "BootProfile"}

// DomainBoot holds layered boot settings.
var DomainBoot = compose.Domain{ID: "boot-setting", Label: "boot setting"}

const (
	KeySplash    = "boot.splash"
	KeyQuiet     = "boot.quiet"
	KeyWatermark = "boot.watermark"
)

// The DeskOS splash theme is image-owned and never replaces a package theme.
const (
	ThemeName = "deskos"
	ThemeDir  = "/usr/share/plymouth/themes/" + ThemeName
)

// MaxWatermarkSize bounds the watermark in pixels; Plymouth draws it unscaled.
const MaxWatermarkSize = 1024

// BootProfileSpec is the public BootProfile spec.
type BootProfileSpec struct {
	Splash    string `json:"splash,omitempty"`
	Quiet     *bool  `json:"quiet,omitempty"`
	Watermark string `json:"watermark,omitempty"`
}

// Asset is a composed image asset value.
type Asset struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	File   string `json:"-"`
}

type decoded struct {
	spec      BootProfileSpec
	watermark *Asset
}

var pngNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.png$`)

// Providers returns the system.deskos.org providers.
func Providers() []registry.Provider { return []registry.Provider{bootProfile{}, updatePolicy{}} }

type bootProfile struct{}

func (bootProfile) GVK() model.GVK { return BootProfileGVK }
func (bootProfile) Schema() string { return "system.deskos.org/v1alpha1/bootprofile.json" }

func (bootProfile) Decode(res *model.Resource) error {
	var s BootProfileSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if s.Splash != "" && s.Splash != "graphical" && s.Splash != "text" {
		errs.Add(model.Errorf(res, "splash must be graphical or text, got %q", s.Splash))
	}
	if s.Splash == "" && s.Quiet == nil && s.Watermark == "" {
		errs.Add(model.Errorf(res, "set at least one of splash, quiet or watermark"))
	}
	d := &decoded{spec: s}
	if s.Watermark != "" {
		as, err := assets.Read(res, s.Watermark)
		switch {
		case err != nil:
			errs.Add(model.Errorf(res, "%s: %v", KeyWatermark, err))
		case !pngNameRE.MatchString(path.Base(as.Path)):
			errs.Add(model.Errorf(res, "%s: %s must be a .png file with a simple name", KeyWatermark, as.Path))
		default:
			cfg, err := png.DecodeConfig(bytes.NewReader(as.Data))
			switch {
			case err != nil:
				errs.Add(model.Errorf(res, "%s: %s is not a PNG image: %v", KeyWatermark, as.Path, err))
			case cfg.Width > MaxWatermarkSize || cfg.Height > MaxWatermarkSize:
				errs.Add(model.Errorf(res, "%s: %s is %dx%d; at most %dx%d pixels", KeyWatermark, as.Path, cfg.Width, cfg.Height, MaxWatermarkSize, MaxWatermarkSize))
			default:
				d.watermark = &Asset{Path: as.Path, SHA256: as.SHA256, File: as.File}
			}
		}
	}
	res.Object = d
	return errs.Err()
}

func (bootProfile) Contribute(res *model.Resource, sc *compose.Scope) error {
	d := res.Object.(*decoded)
	s := d.spec
	if d.watermark != nil {
		sc.SetScalar(DomainBoot, KeyWatermark, *d.watermark, d.watermark.Path)
	}
	if s.Splash != "" {
		sc.SetScalar(DomainBoot, KeySplash, s.Splash, s.Splash)
	}
	if s.Quiet != nil {
		sc.SetScalar(DomainBoot, KeyQuiet, *s.Quiet, strconv.FormatBool(*s.Quiet))
	}
	return nil
}

// Lowerer translates boot and update settings into plan IR.
type Lowerer struct{}

func (Lowerer) Name() string { return "system" }

func (Lowerer) Lower(c *compose.Composition, p *plan.Plan) error {
	var errs model.ErrorList
	errs.Add(lowerBoot(c, p))
	errs.Add(lowerUpdates(c, p))
	return errs.Err()
}

// lowerBoot translates boot settings into kernel arguments and, for the
// graphical splash, Plymouth packages, an optional DeskOS theme carrying
// the watermark, and an initramfs that contains them.
func lowerBoot(c *compose.Composition, p *plan.Plan) error {
	settings := c.Result.Scalars(DomainBoot)
	if len(settings) == 0 {
		return nil
	}
	b := c.PlatformSpec.Boot
	if b == nil {
		var who []string
		for _, s := range settings {
			for _, pr := range s.Provenance {
				who = append(who, pr.Describe()+" ("+pr.Source+")")
			}
		}
		return fmt.Errorf("%s declares no boot facts; BootProfile settings cannot be applied\n  set by: %s", c.Platform.ID(), strings.Join(who, "; "))
	}
	splash, hasSplash := c.Result.Scalar(DomainBoot, KeySplash)
	graphical := hasSplash && splash.Value.(string) == "graphical"
	if graphical {
		p.Artifact.KernelArguments = append(p.Artifact.KernelArguments, plan.KernelArgument{Arg: b.SplashKernelArgument, Setting: KeySplash, Provenance: splash.Provenance})
		for _, pkg := range b.PlymouthPackages {
			p.AddPackage(pkg, splash.Provenance...)
		}
		p.Artifact.Initramfs = &plan.InitramfsRegeneration{DracutModules: []string{b.PlymouthDracutModule}, Provenance: splash.Provenance}
	}
	if wm, ok := c.Result.Scalar(DomainBoot, KeyWatermark); ok {
		switch {
		case !hasSplash:
			return fmt.Errorf("%s requires splash: graphical, which no BootProfile sets\n  set by: %s", KeyWatermark, describe(wm.Provenance))
		case !graphical && wm.Layer < splash.Layer:
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s from %s layer is masked: %s is text at %s layer (%s)",
				KeyWatermark, wm.Layer, KeySplash, splash.Layer, describeShort(splash.Provenance)))
		case !graphical:
			return fmt.Errorf("%s is set at %s layer but %s is text at %s layer; a text splash cannot show it\n  set by: %s\n  text by: %s",
				KeyWatermark, wm.Layer, KeySplash, splash.Layer, describe(wm.Provenance), describe(splash.Provenance))
		case b.SpinnerFrames == nil:
			return fmt.Errorf("%s declares no boot.spinnerFrames; %s cannot be applied\n  set by: %s", c.Platform.ID(), KeyWatermark, describe(wm.Provenance))
		default:
			a := wm.Value.(Asset)
			dest := path.Join(ThemeDir, "watermark.png")
			p.Artifact.Files = append(p.Artifact.Files, plan.FileInstall{
				Path: dest, Mode: "0644", SHA256: a.SHA256, Asset: a.Path, AssetFile: a.File, Provenance: wm.Provenance,
			})
			p.AddPackage(b.SpinnerFrames.Package, wm.Provenance...)
			p.Artifact.PlymouthTheme = &plan.PlymouthTheme{
				Name: ThemeName, Dir: ThemeDir, FramesFrom: b.SpinnerFrames.ImageDir, Watermark: dest,
				Setting: KeyWatermark, Provenance: wm.Provenance,
			}
			p.Artifact.Initramfs.Provenance = append(p.Artifact.Initramfs.Provenance, wm.Provenance...)
		}
	}
	if s, ok := c.Result.Scalar(DomainBoot, KeyQuiet); ok && s.Value.(bool) {
		p.Artifact.KernelArguments = append(p.Artifact.KernelArguments, plan.KernelArgument{Arg: b.QuietKernelArgument, Setting: KeyQuiet, Provenance: s.Provenance})
	}
	return nil
}

func describe(ps []model.Provenance) string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Describe()+" ("+p.Source+")")
	}
	return strings.Join(out, "; ")
}

func describeShort(ps []model.Provenance) string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Resource)
	}
	return strings.Join(out, ", ")
}
