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
	DisplayName    string   `json:"displayName"`
	Family         string   `json:"family"`
	Release        string   `json:"release"`
	Architectures  []string `json:"architectures"`
	PackageManager string   `json:"packageManager"`
	// Owner is the person or team accountable for the base this platform starts
	// from, and Tier is how that base is supported ("vendor" or "organization").
	// ADR 0014 requires both, and says an unnamed owner is not a tier but an
	// omission; recording them in the resource is what makes them reviewable.
	Owner         string         `json:"owner,omitempty"`
	Tier          string         `json:"tier,omitempty"`
	Bootc         BootcBase      `json:"bootc"`
	Distribution  Distribution   `json:"distribution"`
	PackageGroups []PackageGroup `json:"packageGroups,omitempty"`
	// RemovePackages are RPM packages the platform image must not carry. The
	// build removes them after the package transactions and fails if one remains
	// (docs/adr/0022). They are not group exclusions: a group exclusion only
	// changes what that transaction installs.
	RemovePackages []string `json:"removePackages,omitempty"`
	// UnavailablePackages records known packages the platform does not
	// provide from a supported repository, so requests fail at compile time.
	UnavailablePackages []UnavailablePackage `json:"unavailablePackages,omitempty"`
	DisplayManager      string               `json:"displayManager,omitempty"`
	Gnome               *GnomePlatform       `json:"gnome,omitempty"`
	Boot                *BootPlatform        `json:"boot,omitempty"`
	Updates             *UpdatesPlatform     `json:"updates,omitempty"`
	Flatpak             FlatpakPlatform      `json:"flatpak"`
	Trust               *TrustPlatform       `json:"trust,omitempty"`
	IconCache           *IconCachePlatform   `json:"iconCache,omitempty"`
	// Downloads says whether the base can fetch a verified artifact, which the
	// build does for every BinaryArtifact (docs/adr/0019).
	Downloads *DownloadsPlatform `json:"downloads,omitempty"`
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
	Name string `json:"name"`
	// RpmGroups and Members are the two shapes a platform's group mapping can
	// take: RPM group ids, or the platform's own metapackage names. A platform
	// declares one, according to its package manager.
	RpmGroups       []string `json:"rpmGroups,omitempty"`
	Members         []string `json:"members,omitempty"`
	ExcludePackages []string `json:"excludePackages,omitempty"`
	Graphical       bool     `json:"graphical,omitempty"`
}

// AllMembers returns the group's members, whichever shape the platform used.
func (g PackageGroup) AllMembers() []string {
	if len(g.Members) > 0 {
		return g.Members
	}
	return g.RpmGroups
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
	// KeepsVendorDefaults reports that this platform's desktop is the vendor's
	// own: its base ships no system dconf profile, so a default DeskOS wrote
	// would not reach a session. DeskOS writes none here, and composition
	// records each GNOME default a profile asked for and did not get.
	KeepsVendorDefaults bool `json:"keepsVendorDefaults,omitempty"`
	// LoginScreen reports that GDM's dconf profile reads DconfDatabase, so
	// org.gnome.login-screen defaults written there reach the login screen.
	LoginScreen       bool             `json:"loginScreen,omitempty"`
	EnabledExtensions []string         `json:"enabledExtensions,omitempty"`
	Extensions        []GnomeExtension `json:"extensions,omitempty"`
	AppLauncher       *AppLauncher     `json:"appLauncher,omitempty"`
	// AppFolders reports that GNOME Shell keeps app folders from the dconf defaults.
	AppFolders bool `json:"appFolders,omitempty"`
}

// AppLauncher is the command that starts an application by desktop file id.
type AppLauncher struct {
	Package string `json:"package"`
	Command string `json:"command"`
}

// GnomeExtension is a GNOME Shell extension shipped by the platform. Exactly one
// delivery is named: the platform's package, or a pinned artifact for a platform
// whose repositories do not carry it.
type GnomeExtension struct {
	Name    string           `json:"name"`
	Package string           `json:"package,omitempty"`
	UUID    string           `json:"uuid"`
	Source  *ExtensionSource `json:"source,omitempty"`
}

// ExtensionSource is a pinned GNOME Shell extension release: the artifact the
// build downloads, verifies and installs.
type ExtensionSource struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
}

// FlatpakPlatform describes the platform's Flatpak capabilities.
type FlatpakPlatform struct {
	Package    string `json:"package,omitempty"`
	Preinstall bool   `json:"preinstall"`
}

// TrustPlatform holds the platform's CA trust-store facts: the directory the
// image places trust anchors in, and the command that regenerates the store.
type TrustPlatform struct {
	AnchorsDir    string `json:"anchorsDir"`
	UpdateCommand string `json:"updateCommand"`
}

// IconCachePlatform is how a platform regenerates the GTK icon theme cache after
// the build copies icons into it, because no package trigger runs for files the
// build writes. Both facts are the platform's: the package that provides the tool,
// and the command line that refreshes one cache directory, which it takes as its
// last argument. Core knows neither name.
type IconCachePlatform struct {
	Package string `json:"package"`
	Command string `json:"command"`
}

// DownloadsPlatform says whether the platform's base can fetch a verified
// artifact, and the package that provides the fetcher when it cannot. Downloading
// is a build step, so a base that cannot fetch cannot build an image that contains
// a BinaryArtifact; the composition says which platform gap that is instead of the
// build failing on a missing command (docs/adr/0019). Neither name is Core's.
type DownloadsPlatform struct {
	// BaseFetches is whether the base ships the fetcher the build uses.
	BaseFetches bool `json:"baseFetches"`
	// Package provides that fetcher on a base that does not ship it, and is
	// required there: the image must install it, or nothing can fetch.
	Package string `json:"package,omitempty"`
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

// GnomeCatalogExtension returns the platform-declared extension with this UUID. The
// catalog, together with the platform's own enabled extensions, is what a profile may
// decide about (docs/adr/0017).
func (p *PlatformSpec) GnomeCatalogExtension(uuid string) (GnomeExtension, bool) {
	if p.Gnome == nil {
		return GnomeExtension{}, false
	}
	for _, e := range p.Gnome.Extensions {
		if e.UUID == uuid {
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
	// Flatpak is what this machine does with the Flatpak remotes the base
	// carries besides the ones the composition declares.
	Flatpak *WorkstationFlatpak `json:"flatpak,omitempty"`
}

// WorkstationFlatpak is the workstation's Flatpak intent.
type WorkstationFlatpak struct {
	// RemoveRemotes retires remotes the base ships. Retiring a source is a
	// decision, so each one carries the reason it was taken (docs/adr/0017).
	RemoveRemotes []RemoteRemoval `json:"removeRemotes,omitempty"`
}

// RemoteRemoval retires a Flatpak remote in the image. It is not `flatpak
// uninstall`: applications already installed from that remote stay on an updated
// machine, and DeskOS never forces (docs/adr/0017).
type RemoteRemoval struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
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
	versionRE     = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+-]*$`)
	sourceURLRE   = regexp.MustCompile(`^https://[^[:space:]]+$`)
	sha256RE      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	plymouthDirRE = regexp.MustCompile(`^/usr/share/plymouth/themes/[a-z0-9][a-z0-9_.-]*$`)
	trustDirRE    = regexp.MustCompile(`^/etc(/[A-Za-z0-9._-]*[A-Za-z0-9_-])+$`)
	trustCmdRE    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
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
	if s.PackageManager == model.PackageManagerDeb && len(s.RemovePackages) > 0 {
		errs.Add(model.Errorf(res, "removePackages is an RPM operation; %s is %s", s.DisplayName, s.PackageManager))
	}
	removed := map[string]bool{}
	for _, name := range s.RemovePackages {
		if !pkgRE.MatchString(name) {
			errs.Add(model.Errorf(res, "invalid removed package %q", name))
		}
		if removed[name] {
			errs.Add(model.Errorf(res, "package %q is removed twice", name))
		}
		removed[name] = true
	}
	seen := map[string]bool{}
	for _, g := range s.PackageGroups {
		if seen[g.Name] {
			errs.Add(model.Errorf(res, "package group %q is mapped twice", g.Name))
		}
		seen[g.Name] = true
		if (len(g.RpmGroups) > 0) == (len(g.Members) > 0) {
			errs.Add(model.Errorf(res, "package group %q: declare either rpmGroups or members, not both", g.Name))
		}
		if len(g.Members) > 0 && s.PackageManager != model.PackageManagerDeb {
			errs.Add(model.Errorf(res, "package group %q: members are for a deb platform; %s is %s", g.Name, s.DisplayName, s.PackageManager))
		}
		if len(g.RpmGroups) > 0 && s.PackageManager == model.PackageManagerDeb {
			errs.Add(model.Errorf(res, "package group %q: rpmGroups are for an RPM platform; %s is %s", g.Name, s.DisplayName, s.PackageManager))
		}
		if s.PackageManager == model.PackageManagerDeb && len(g.ExcludePackages) > 0 {
			errs.Add(model.Errorf(res, "package group %q: deb has no package exclusion flag", g.Name))
		}
		for _, rg := range g.AllMembers() {
			if !pkgRE.MatchString(rg) {
				errs.Add(model.Errorf(res, "package group %q: invalid member %q", g.Name, rg))
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
		switch {
		case g.KeepsVendorDefaults:
			if g.DconfDatabase != "" {
				errs.Add(model.Errorf(res, "gnome.dconfDatabase %q is declared although the platform keeps its vendor's desktop defaults and writes no database", g.DconfDatabase))
			}
			if g.LoginScreen {
				errs.Add(model.Errorf(res, "gnome.loginScreen is declared although the platform keeps its vendor's desktop defaults and writes no database for GDM to read"))
			}
		case !model.ValidName(g.DconfDatabase):
			errs.Add(model.Errorf(res, "gnome.dconfDatabase %q is not a valid database name", g.DconfDatabase))
		}
		seenExtension := map[string]bool{}
		for _, e := range g.Extensions {
			if !uuidRE.MatchString(e.UUID) || e.Name == "" {
				errs.Add(model.Errorf(res, "gnome extension %q: name and uuid are required", e.Name))
				continue
			}
			if seenExtension[e.UUID] {
				errs.Add(model.Errorf(res, "gnome extension %q: uuid %q is catalogued twice", e.Name, e.UUID))
			}
			seenExtension[e.UUID] = true
			switch {
			case e.Package != "" && e.Source != nil:
				errs.Add(model.Errorf(res, "gnome extension %q: name exactly one delivery, package or source, not both", e.Name))
			case e.Package != "":
				if !pkgRE.MatchString(e.Package) {
					errs.Add(model.Errorf(res, "gnome extension %q: %q is not a package name", e.Name, e.Package))
				}
			case e.Source != nil:
				src := e.Source
				if !versionRE.MatchString(src.Version) || !sourceURLRE.MatchString(src.URL) || !sha256RE.MatchString(src.SHA256) {
					errs.Add(model.Errorf(res, "gnome extension %q: source needs a version, an https url and a sha256:<64 hex> digest", e.Name))
				}
			default:
				errs.Add(model.Errorf(res, "gnome extension %q: neither a package nor a source is named", e.Name))
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
	if t := s.Trust; t != nil {
		if !trustDirRE.MatchString(t.AnchorsDir) {
			errs.Add(model.Errorf(res, "trust.anchorsDir %q must be an absolute directory under /etc", t.AnchorsDir))
		}
		if !trustCmdRE.MatchString(t.UpdateCommand) {
			errs.Add(model.Errorf(res, "trust.updateCommand %q must be a bare command name", t.UpdateCommand))
		}
	}
	if d := s.Downloads; d != nil && !d.BaseFetches && !pkgRE.MatchString(d.Package) {
		errs.Add(model.Errorf(res, "downloads.package %q is required when downloads.baseFetches is false: the base has nothing to fetch with, so the image must install it", d.Package))
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
	if f := s.Flatpak; f != nil {
		seenRemote := map[string]bool{}
		for _, r := range f.RemoveRemotes {
			if !model.ValidName(r.Name) {
				errs.Add(model.Errorf(res, "flatpak.removeRemotes: %q is not a valid remote name", r.Name))
			}
			if strings.TrimSpace(r.Reason) == "" {
				errs.Add(model.Errorf(res, "flatpak.removeRemotes: %q needs the reason it is retired", r.Name))
			}
			if seenRemote[r.Name] {
				errs.Add(model.Errorf(res, "flatpak.removeRemotes: %q is listed twice", r.Name))
			}
			seenRemote[r.Name] = true
		}
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
