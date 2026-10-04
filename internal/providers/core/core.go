// Package core provides the core.deskos.org kinds: Platform, Profile and
// Workstation.
package core

import (
	"regexp"
	"slices"
	"strings"

	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/registry"
)

var (
	PlatformGVK    = model.GVK{Group: model.GroupCore, Version: model.V1Alpha1, Kind: "Platform"}
	ProfileGVK     = model.GVK{Group: model.GroupCore, Version: model.V1Alpha1, Kind: "Profile"}
	WorkstationGVK = model.GVK{Group: model.GroupCore, Version: model.V1Alpha1, Kind: "Workstation"}
)

// PlatformSpec holds the facts about an OS implementation target that the
// compiler needs. Platform-specific behavior is read from here and nowhere
// else.
type PlatformSpec struct {
	DisplayName   string         `json:"displayName"`
	Family        string         `json:"family"`
	Release       string         `json:"release"`
	Architectures []string       `json:"architectures"`
	Bootc         BootcBase      `json:"bootc"`
	Distribution  Distribution   `json:"distribution"`
	PackageGroups []PackageGroup `json:"packageGroups,omitempty"`
	// UnavailablePackages records known packages the platform does not
	// provide from a supported repository, so requests fail at compile time.
	UnavailablePackages []UnavailablePackage `json:"unavailablePackages,omitempty"`
	DisplayManager      string               `json:"displayManager,omitempty"`
	Gnome               *GnomePlatform       `json:"gnome,omitempty"`
	Boot                *BootPlatform        `json:"boot,omitempty"`
	Updates             *UpdatesPlatform     `json:"updates,omitempty"`
	Flatpak             FlatpakPlatform      `json:"flatpak"`
}

// BootcBase is the official bootc base image. Digest pins it when set.
type BootcBase struct {
	Image  string `json:"image"`
	Digest string `json:"digest,omitempty"`
}

// Distribution records publication constraints of derived artifacts.
type Distribution struct {
	Redistributable      bool `json:"redistributable"`
	RequiresSubscription bool `json:"requiresSubscription"`
}

// PackageGroup maps a DeskOS package-group name to platform RPM groups.
// ExcludePackages are group members the platform does not install with it.
type PackageGroup struct {
	Name            string   `json:"name"`
	RpmGroups       []string `json:"rpmGroups"`
	ExcludePackages []string `json:"excludePackages,omitempty"`
	Graphical       bool     `json:"graphical,omitempty"`
}

// BootPlatform holds the platform's boot splash facts: kernel arguments that
// select Plymouth's graphical splash and a quiet boot, the packages that
// provide Plymouth and its default theme, and the dracut module that puts
// Plymouth into the initramfs.
type BootPlatform struct {
	SplashKernelArgument string   `json:"splashKernelArgument"`
	QuietKernelArgument  string   `json:"quietKernelArgument"`
	PlymouthPackages     []string `json:"plymouthPackages"`
	PlymouthDracutModule string   `json:"plymouthDracutModule"`
	// SpinnerFrames is the stock two-step animation a DeskOS theme reuses.
	SpinnerFrames *PlymouthFrames `json:"spinnerFrames,omitempty"`
}

// PlymouthFrames is a package-owned Plymouth image directory.
type PlymouthFrames struct {
	Package  string `json:"package"`
	ImageDir string `json:"imageDir"`
}

// UpdatesPlatform holds the platform's image update facts.
type UpdatesPlatform struct {
	// ImageUpdateUnits are platform units that update the image on their own.
	ImageUpdateUnits []string `json:"imageUpdateUnits"`
}

// UnavailablePackage is a package the platform deliberately does not supply.
type UnavailablePackage struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// GnomePlatform holds GNOME integration facts of the platform.
type GnomePlatform struct {
	DconfDatabase string `json:"dconfDatabase"`
	// LoginScreen reports that GDM's dconf profile reads DconfDatabase, so
	// org.gnome.login-screen defaults written there reach the login screen.
	LoginScreen       bool             `json:"loginScreen,omitempty"`
	EnabledExtensions []string         `json:"enabledExtensions,omitempty"`
	Extensions        []GnomeExtension `json:"extensions,omitempty"`
	AppLauncher       *AppLauncher     `json:"appLauncher,omitempty"`
}

// AppLauncher is the command that starts an application by desktop file id.
type AppLauncher struct {
	Package string `json:"package"`
	Command string `json:"command"`
}

// GnomeExtension is a GNOME Shell extension shipped by the platform.
type GnomeExtension struct {
	Name    string `json:"name"`
	Package string `json:"package"`
	UUID    string `json:"uuid"`
}

// FlatpakPlatform describes the platform's Flatpak capabilities.
type FlatpakPlatform struct {
	Package    string `json:"package,omitempty"`
	Preinstall bool   `json:"preinstall"`
}

// PackageGroup returns the named mapping.
func (p *PlatformSpec) PackageGroup(name string) (PackageGroup, bool) {
	for _, g := range p.PackageGroups {
		if g.Name == name {
			return g, true
		}
	}
	return PackageGroup{}, false
}

// Unavailable reports why a package cannot be installed on the platform.
func (p *PlatformSpec) Unavailable(name string) (string, bool) {
	for _, u := range p.UnavailablePackages {
		if u.Name == name {
			return u.Reason, true
		}
	}
	return "", false
}

// GnomeExtension returns the named extension.
func (p *PlatformSpec) GnomeExtension(name string) (GnomeExtension, bool) {
	if p.Gnome == nil {
		return GnomeExtension{}, false
	}
	for _, e := range p.Gnome.Extensions {
		if e.Name == name {
			return e, true
		}
	}
	return GnomeExtension{}, false
}

// BaseImageRef returns the FROM reference, digest-pinned when a digest is set.
func (p *PlatformSpec) BaseImageRef() string {
	if p.Bootc.Digest != "" {
		return p.Bootc.Image + "@" + p.Bootc.Digest
	}
	return p.Bootc.Image
}

// ProfileSpec is a reusable fragment of intent at an explicit layer.
type ProfileSpec struct {
	Layer       model.Layer       `json:"layer"`
	Description string            `json:"description,omitempty"`
	Resources   []model.ObjectRef `json:"resources"`
}

// WorkstationSpec is a concrete artifact target.
type WorkstationSpec struct {
	DisplayName string   `json:"displayName,omitempty"`
	PlatformRef string   `json:"platformRef"`
	Profiles    []string `json:"profiles"`
}

var (
	imageRE       = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(:[0-9]+)?(/[a-z0-9]+([._-][a-z0-9]+)*)+(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127})?$`)
	digestRE      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	rpmGroupRE    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	unitRE        = regexp.MustCompile(`^[A-Za-z0-9@_.:-]+\.service$`)
	updateUnitRE  = regexp.MustCompile(`^[A-Za-z0-9_.:-]+\.(service|timer)$`)
	uuidRE        = regexp.MustCompile(`^[A-Za-z0-9._@-]+$`)
	pkgRE         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
	kargRE        = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)
	plymouthDirRE = regexp.MustCompile(`^/usr/share/plymouth/themes/[a-z0-9][a-z0-9_.-]*$`)
)

type platformProvider struct{}
type profileProvider struct{}
type workstationProvider struct{}

// Providers returns the core.deskos.org providers.
func Providers() []registry.Provider {
	return []registry.Provider{platformProvider{}, profileProvider{}, workstationProvider{}}
}

func (platformProvider) GVK() model.GVK { return PlatformGVK }
func (platformProvider) Schema() string { return "core.deskos.org/v1alpha1/platform.json" }

func (platformProvider) Decode(res *model.Resource) error {
	var s PlatformSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if s.DisplayName == "" || s.Family == "" || s.Release == "" {
		errs.Add(model.Errorf(res, "displayName, family and release are required"))
	}
	if !slices.Equal(s.Architectures, []string{"x86_64"}) {
		errs.Add(model.Errorf(res, "architectures must be [x86_64] in v1alpha1"))
	}
	if !imageRE.MatchString(s.Bootc.Image) {
		errs.Add(model.Errorf(res, "bootc.image %q is not a valid image reference", s.Bootc.Image))
	}
	if s.Bootc.Digest != "" && !digestRE.MatchString(s.Bootc.Digest) {
		errs.Add(model.Errorf(res, "bootc.digest %q must be sha256:<64 hex>", s.Bootc.Digest))
	}
	seen := map[string]bool{}
	for _, g := range s.PackageGroups {
		if seen[g.Name] {
			errs.Add(model.Errorf(res, "package group %q is mapped twice", g.Name))
		}
		seen[g.Name] = true
		for _, rg := range g.RpmGroups {
			if !rpmGroupRE.MatchString(rg) {
				errs.Add(model.Errorf(res, "package group %q: invalid RPM group id %q", g.Name, rg))
			}
		}
		excluded := map[string]bool{}
		for _, x := range g.ExcludePackages {
			if !pkgRE.MatchString(x) {
				errs.Add(model.Errorf(res, "package group %q: invalid excluded package %q", g.Name, x))
			}
			if excluded[x] {
				errs.Add(model.Errorf(res, "package group %q: package %q is excluded twice", g.Name, x))
			}
			excluded[x] = true
		}
		if g.Graphical && s.DisplayManager == "" {
			errs.Add(model.Errorf(res, "package group %q is graphical but displayManager is not set", g.Name))
		}
	}
	for _, u := range s.UnavailablePackages {
		if !pkgRE.MatchString(u.Name) || u.Reason == "" {
			errs.Add(model.Errorf(res, "unavailablePackages: %q needs a valid name and a reason", u.Name))
		}
	}
	if s.DisplayManager != "" && !unitRE.MatchString(s.DisplayManager) {
		errs.Add(model.Errorf(res, "displayManager %q must be a .service unit", s.DisplayManager))
	}
	if g := s.Gnome; g != nil {
		if !model.ValidName(g.DconfDatabase) {
			errs.Add(model.Errorf(res, "gnome.dconfDatabase %q is not a valid database name", g.DconfDatabase))
		}
		for _, e := range g.Extensions {
			if !uuidRE.MatchString(e.UUID) || !pkgRE.MatchString(e.Package) || e.Name == "" {
				errs.Add(model.Errorf(res, "gnome extension %q: name, package and uuid are required", e.Name))
			}
		}
	}
	if b := s.Boot; b != nil {
		if !kargRE.MatchString(b.SplashKernelArgument) || !kargRE.MatchString(b.QuietKernelArgument) {
			errs.Add(model.Errorf(res, "boot.splashKernelArgument and boot.quietKernelArgument must be simple kernel arguments"))
		}
		if !rpmGroupRE.MatchString(b.PlymouthDracutModule) || len(b.PlymouthPackages) == 0 {
			errs.Add(model.Errorf(res, "boot needs plymouthPackages and plymouthDracutModule"))
		}
		for _, p := range b.PlymouthPackages {
			if !pkgRE.MatchString(p) {
				errs.Add(model.Errorf(res, "boot.plymouthPackages: invalid package %q", p))
			}
		}
		if f := b.SpinnerFrames; f != nil && (!pkgRE.MatchString(f.Package) || !plymouthDirRE.MatchString(f.ImageDir)) {
			errs.Add(model.Errorf(res, "boot.spinnerFrames needs a package and an imageDir under /usr/share/plymouth/themes/"))
		}
	}
	if u := s.Updates; u != nil {
		seen := map[string]bool{}
		for _, unit := range u.ImageUpdateUnits {
			if !updateUnitRE.MatchString(unit) || seen[unit] {
				errs.Add(model.Errorf(res, "updates.imageUpdateUnits: %q must be a unique .service or .timer unit", unit))
			}
			seen[unit] = true
		}
	}
	if s.Flatpak.Preinstall && !pkgRE.MatchString(s.Flatpak.Package) {
		errs.Add(model.Errorf(res, "flatpak.package is required when flatpak.preinstall is true"))
	}
	res.Object = &s
	return errs.Err()
}

func (profileProvider) GVK() model.GVK { return ProfileGVK }
func (profileProvider) Schema() string { return "core.deskos.org/v1alpha1/profile.json" }

func (profileProvider) Decode(res *model.Resource) error {
	var s ProfileSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if s.Layer == 0 {
		errs.Add(model.Errorf(res, "layer is required"))
	}
	seen := map[string]bool{}
	for _, r := range s.Resources {
		if r.Kind == "" || !model.ValidName(r.Name) {
			errs.Add(model.Errorf(res, "invalid resource reference %s", r))
			continue
		}
		key := strings.ToLower(r.APIVersion + "/" + r.String())
		if seen[key] {
			errs.Add(model.Errorf(res, "resource %s is listed twice", r))
		}
		seen[key] = true
	}
	res.Object = &s
	return errs.Err()
}

// References lists the resources the profile includes.
func (profileProvider) References(res *model.Resource) []model.ObjectRef {
	return res.Object.(*ProfileSpec).Resources
}

func (workstationProvider) GVK() model.GVK { return WorkstationGVK }
func (workstationProvider) Schema() string { return "core.deskos.org/v1alpha1/workstation.json" }

func (workstationProvider) Decode(res *model.Resource) error {
	var s WorkstationSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if !model.ValidName(s.PlatformRef) {
		errs.Add(model.Errorf(res, "platformRef %q is not a valid name", s.PlatformRef))
	}
	if len(s.Profiles) == 0 {
		errs.Add(model.Errorf(res, "at least one profile is required"))
	}
	seen := map[string]bool{}
	for _, p := range s.Profiles {
		if seen[p] {
			errs.Add(model.Errorf(res, "profile %q is listed twice", p))
		}
		seen[p] = true
	}
	res.Object = &s
	return errs.Err()
}

// References lists the platform and profiles.
func (workstationProvider) References(res *model.Resource) []model.ObjectRef {
	s := res.Object.(*WorkstationSpec)
	refs := []model.ObjectRef{{APIVersion: PlatformGVK.APIVersion(), Kind: "Platform", Name: s.PlatformRef}}
	for _, p := range s.Profiles {
		refs = append(refs, model.ObjectRef{APIVersion: ProfileGVK.APIVersion(), Kind: "Profile", Name: p})
	}
	return refs
}
