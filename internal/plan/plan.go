// Package plan defines the typed DeskOS Plan, the intermediate
// representation between composition and artifact backends.
//
// The Plan knows nothing about YAML or Containerfile syntax and contains no
// shell. Backends consume only this structure.
package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
)

// Format identifies the plan encoding version.
const Format = "plan.deskos.org/v1alpha1"

// Plan is the complete effective intent of one workstation artifact, split by
// lifecycle phase: what the image contains, what is materialized when a
// machine is provisioned, and what is left to enrollment.
type Plan struct {
	Format       string       `json:"format"`
	Workstation  Workstation  `json:"workstation"`
	Platform     Platform     `json:"platform"`
	Profiles     []Profile    `json:"profiles"`
	Artifact     Artifact     `json:"artifact"`
	Provisioning Provisioning `json:"provisioning"`
	Enrollment   Enrollment   `json:"enrollment"`
	Warnings     []string     `json:"warnings,omitempty"`
}

type Workstation struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Source      string `json:"source"`
}

type Platform struct {
	Name                 string   `json:"name"`
	DisplayName          string   `json:"displayName"`
	Family               string   `json:"family"`
	Release              string   `json:"release"`
	Architectures        []string `json:"architectures"`
	Redistributable      bool     `json:"redistributable"`
	RequiresSubscription bool     `json:"requiresSubscription"`
	Source               string   `json:"source"`
}

type Profile struct {
	Name   string `json:"name"`
	Layer  string `json:"layer"`
	Source string `json:"source"`
}

// Artifact is image-owned content.
type Artifact struct {
	BaseImage       BaseImage                `json:"baseImage"`
	Labels          []Label                  `json:"labels"`
	RpmRepositories []RpmRepository          `json:"rpmRepositories"`
	RpmGroups       []RpmGroupInstall        `json:"rpmGroups"`
	RpmPackages     []RpmInstall             `json:"rpmPackages"`
	RpmFiles        []RpmFileInstall         `json:"rpmFiles"`
	Binaries        []VerifiedBinaryInstall  `json:"binaries"`
	Files           []FileInstall            `json:"files"`
	Dconf           *DconfDatabase           `json:"dconf,omitempty"`
	GSettings       []GSettingsVendorDefault `json:"gsettingsVendorDefaults"`
	KernelArguments []KernelArgument         `json:"kernelArguments"`
	Initramfs       *InitramfsRegeneration   `json:"initramfs,omitempty"`
	PlymouthTheme   *PlymouthTheme           `json:"plymouthTheme,omitempty"`
	SystemdUnits    []SystemdEnable          `json:"systemdUnits"`
	DefaultTarget   *DefaultTarget           `json:"defaultTarget,omitempty"`
}

type BaseImage struct {
	Image  string `json:"image"`
	Digest string `json:"digest,omitempty"`
	// Ref is the reference used to build: image@digest when pinned.
	Ref string `json:"ref"`
}

type Label struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type RpmRepository struct {
	ID          string             `json:"id"`
	DisplayName string             `json:"displayName"`
	BaseURL     string             `json:"baseURL"`
	Enabled     bool               `json:"enabled"`
	GPGCheck    bool               `json:"gpgCheck"`
	GPGKeys     []string           `json:"gpgKeys"`
	Provenance  []model.Provenance `json:"provenance"`
}

// RpmGroupInstall installs platform RPM groups; ExcludePackages are group members excluded from that install.
type RpmGroupInstall struct {
	Name            string             `json:"name"`
	RpmGroups       []string           `json:"rpmGroups"`
	ExcludePackages []string           `json:"excludePackages,omitempty"`
	Provenance      []model.Provenance `json:"provenance"`
}

type RpmInstall struct {
	Name       string             `json:"name"`
	Provenance []model.Provenance `json:"provenance"`
}

// RpmFileInstall downloads one RPM without a repository, verifies its
// SHA-256 and its signature by GPGKey (an ASCII-armored public key), and
// installs it with the platform package manager.
type RpmFileInstall struct {
	URL          string             `json:"url"`
	SHA256       string             `json:"sha256"`
	GPGKey       string             `json:"gpgKey"`
	GPGKeySHA256 string             `json:"gpgKeySHA256"`
	Provenance   []model.Provenance `json:"provenance"`
}

// VerifiedBinaryInstall downloads an artifact, verifies its SHA-256,
// optionally extracts one member and installs it with a fixed mode.
type VerifiedBinaryInstall struct {
	Artifact    string             `json:"artifact"`
	Version     string             `json:"version"`
	URL         string             `json:"url"`
	SHA256      string             `json:"sha256"`
	Archive     string             `json:"archive"`
	Member      string             `json:"member,omitempty"`
	Destination string             `json:"destination"`
	Mode        string             `json:"mode"`
	Provenance  []model.Provenance `json:"provenance"`
}

// FileInstall places a repository asset in the image. Files derived from
// other typed IR (repository files, dconf keyfiles) are generated by backends.
type FileInstall struct {
	Path       string             `json:"path"`
	Mode       string             `json:"mode"`
	SHA256     string             `json:"sha256"`
	Asset      string             `json:"asset,omitempty"`
	Provenance []model.Provenance `json:"provenance"`

	// AssetFile is the absolute host path of Asset; never serialized.
	AssetFile string `json:"-"`
}

// DconfDatabase is a system dconf database carrying defaults and locks.
type DconfDatabase struct {
	Name     string         `json:"name"`
	Defaults []DconfDefault `json:"defaults"`
	Locks    []DconfLock    `json:"locks"`
}

// DconfDefault is one key default. Value is GVariant text format.
type DconfDefault struct {
	Key        string             `json:"key"`
	Type       string             `json:"type"`
	Value      string             `json:"value"`
	Setting    string             `json:"setting"`
	Provenance []model.Provenance `json:"provenance"`
}

// GSettingsVendorDefault changes a schema default through a GLib vendor
// override. It reaches sessions whose dconf profile does not read the
// DeskOS dconf database, such as GNOME Initial Setup. Value is GVariant text.
type GSettingsVendorDefault struct {
	Schema     string             `json:"schema"`
	Key        string             `json:"key"`
	Type       string             `json:"type"`
	Value      string             `json:"value"`
	Setting    string             `json:"setting"`
	Provenance []model.Provenance `json:"provenance"`
}

// KernelArgument is a default kernel argument carried by the image
// (bootc kargs.d); bootc applies it at install and on updates.
type KernelArgument struct {
	Arg        string             `json:"arg"`
	Setting    string             `json:"setting"`
	Provenance []model.Provenance `json:"provenance"`
}

// InitramfsRegeneration rebuilds the initramfs of every kernel in the image
// with the given dracut modules added.
type InitramfsRegeneration struct {
	DracutModules []string           `json:"dracutModules"`
	Provenance    []model.Provenance `json:"provenance"`
}

// PlymouthTheme is an image-owned two-step Plymouth theme made the default
// theme. Its directory holds the theme file, the watermark (a Files entry)
// and a copy of the frames of FramesFrom, a package-owned image directory.
type PlymouthTheme struct {
	Name       string             `json:"name"`
	Dir        string             `json:"dir"`
	FramesFrom string             `json:"framesFrom"`
	Watermark  string             `json:"watermark"`
	Setting    string             `json:"setting"`
	Provenance []model.Provenance `json:"provenance"`
}

type DconfLock struct {
	Key        string             `json:"key"`
	Setting    string             `json:"setting"`
	Provenance []model.Provenance `json:"provenance"`
}

type SystemdEnable struct {
	Unit       string             `json:"unit"`
	Provenance []model.Provenance `json:"provenance"`
}

type DefaultTarget struct {
	Target     string             `json:"target"`
	Provenance []model.Provenance `json:"provenance"`
}

// Provisioning is intent materialized when a machine is provisioned. The
// descriptors are image-owned; the content they describe is not.
type Provisioning struct {
	FlatpakRemotes      []FlatpakRemote     `json:"flatpakRemotes"`
	FlatpakApplications []FlatpakPreinstall `json:"flatpakApplications"`
	// FlatpakPreinstallUnit is the systemd unit that runs the upstream
	// `flatpak preinstall` at startup; empty when nothing is preinstalled.
	FlatpakPreinstallUnit string `json:"flatpakPreinstallUnit,omitempty"`
}

type FlatpakRemote struct {
	Name         string             `json:"name"`
	Title        string             `json:"title"`
	URL          string             `json:"url"`
	Homepage     string             `json:"homepage,omitempty"`
	Comment      string             `json:"comment,omitempty"`
	Description  string             `json:"description,omitempty"`
	CollectionID string             `json:"collectionID,omitempty"`
	GPGKey       string             `json:"gpgKey"`
	GPGKeySHA256 string             `json:"gpgKeySHA256"`
	Provenance   []model.Provenance `json:"provenance"`
}

type FlatpakPreinstall struct {
	ID           string             `json:"id"`
	Branch       string             `json:"branch"`
	Remote       string             `json:"remote"`
	CollectionID string             `json:"collectionID,omitempty"`
	Provenance   []model.Provenance `json:"provenance"`
}

// Enrollment is reserved for organizational enrollment (identity join,
// device certificates). Nothing is emitted in v1alpha1.
type Enrollment struct{}

// Lowerer translates composed semantic intent of one domain into Plan IR,
// using only facts from the platform.
type Lowerer interface {
	Name() string
	Lower(c *compose.Composition, p *Plan) error
}

// AddPackage adds an RPM package, merging provenance with existing entries.
func (p *Plan) AddPackage(name string, prov ...model.Provenance) {
	p.Artifact.RpmPackages = append(p.Artifact.RpmPackages, RpmInstall{Name: name, Provenance: prov})
}

// EnableUnit adds a systemd unit to enable.
func (p *Plan) EnableUnit(unit string, prov ...model.Provenance) {
	p.Artifact.SystemdUnits = append(p.Artifact.SystemdUnits, SystemdEnable{Unit: unit, Provenance: prov})
}

// Normalize sorts every collection, merges duplicate set entries and
// rejects conflicting file destinations. It makes the Plan canonical.
func (p *Plan) Normalize() error {
	a := &p.Artifact
	sort.Slice(a.Labels, func(i, j int) bool { return a.Labels[i].Name < a.Labels[j].Name })
	sort.Slice(a.RpmRepositories, func(i, j int) bool { return a.RpmRepositories[i].ID < a.RpmRepositories[j].ID })
	sort.Slice(a.RpmGroups, func(i, j int) bool { return a.RpmGroups[i].Name < a.RpmGroups[j].Name })
	for i := range a.RpmGroups {
		a.RpmGroups[i].ExcludePackages = sortedUnique(a.RpmGroups[i].ExcludePackages)
	}
	a.RpmPackages = mergeByName(a.RpmPackages, func(x RpmInstall) string { return x.Name },
		func(x *RpmInstall) *[]model.Provenance { return &x.Provenance })
	a.SystemdUnits = mergeByName(a.SystemdUnits, func(x SystemdEnable) string { return x.Unit },
		func(x *SystemdEnable) *[]model.Provenance { return &x.Provenance })
	sort.Slice(a.RpmFiles, func(i, j int) bool { return a.RpmFiles[i].URL < a.RpmFiles[j].URL })
	sort.Slice(a.Binaries, func(i, j int) bool { return a.Binaries[i].Destination < a.Binaries[j].Destination })
	sort.Slice(a.Files, func(i, j int) bool { return a.Files[i].Path < a.Files[j].Path })
	for i := 1; i < len(a.Files); i++ {
		x, y := a.Files[i-1], a.Files[i]
		if x.Path == y.Path {
			if x.SHA256 != y.SHA256 {
				return fmt.Errorf("conflicting content for image file %s\n  from %s\n  and  %s", x.Path, x.Asset, y.Asset)
			}
			a.Files[i].Provenance = append(a.Files[i].Provenance, x.Provenance...)
			a.Files = append(a.Files[:i-1], a.Files[i:]...)
			i--
		}
	}
	for i := range a.Files {
		a.Files[i].Provenance = uniq(a.Files[i].Provenance)
	}
	sort.Slice(a.GSettings, func(i, j int) bool {
		x, y := a.GSettings[i], a.GSettings[j]
		if x.Schema != y.Schema {
			return x.Schema < y.Schema
		}
		return x.Key < y.Key
	})
	for i := 1; i < len(a.GSettings); i++ {
		x, y := a.GSettings[i-1], a.GSettings[i]
		if x.Schema == y.Schema && x.Key == y.Key {
			if x.Type != y.Type || x.Value != y.Value {
				return fmt.Errorf("conflicting GSettings vendor defaults for %s %s: %s (%s) and %s (%s)", x.Schema, x.Key, x.Value, x.Setting, y.Value, y.Setting)
			}
			a.GSettings[i].Provenance = append(a.GSettings[i].Provenance, x.Provenance...)
			a.GSettings = append(a.GSettings[:i-1], a.GSettings[i:]...)
			i--
		}
	}
	for i := range a.GSettings {
		a.GSettings[i].Provenance = uniq(a.GSettings[i].Provenance)
	}
	sort.Slice(a.KernelArguments, func(i, j int) bool { return a.KernelArguments[i].Arg < a.KernelArguments[j].Arg })
	for i := 1; i < len(a.KernelArguments); i++ {
		if a.KernelArguments[i].Arg == a.KernelArguments[i-1].Arg {
			a.KernelArguments[i].Provenance = append(a.KernelArguments[i].Provenance, a.KernelArguments[i-1].Provenance...)
			a.KernelArguments = append(a.KernelArguments[:i-1], a.KernelArguments[i:]...)
			i--
		}
	}
	for i := range a.KernelArguments {
		a.KernelArguments[i].Provenance = uniq(a.KernelArguments[i].Provenance)
	}
	if a.Initramfs != nil {
		sort.Strings(a.Initramfs.DracutModules)
		a.Initramfs.Provenance = uniq(a.Initramfs.Provenance)
	}
	if t := a.PlymouthTheme; t != nil {
		t.Provenance = uniq(t.Provenance)
		if a.Initramfs == nil {
			return fmt.Errorf("boot splash theme %s needs an initramfs regeneration", t.Name)
		}
		if !hasFile(a.Files, t.Watermark) {
			return fmt.Errorf("boot splash theme %s: watermark %s is not an image file of the plan", t.Name, t.Watermark)
		}
	}
	if a.Dconf != nil {
		sort.Slice(a.Dconf.Defaults, func(i, j int) bool { return a.Dconf.Defaults[i].Key < a.Dconf.Defaults[j].Key })
		sort.Slice(a.Dconf.Locks, func(i, j int) bool { return a.Dconf.Locks[i].Key < a.Dconf.Locks[j].Key })
	}
	pr := &p.Provisioning
	sort.Slice(pr.FlatpakRemotes, func(i, j int) bool { return pr.FlatpakRemotes[i].Name < pr.FlatpakRemotes[j].Name })
	sort.Slice(pr.FlatpakApplications, func(i, j int) bool { return pr.FlatpakApplications[i].ID < pr.FlatpakApplications[j].ID })
	sort.Strings(p.Warnings)
	nonNil(p)
	return nil
}

func mergeByName[T any](in []T, name func(T) string, prov func(*T) *[]model.Provenance) []T {
	byName := map[string]int{}
	var out []T
	for _, x := range in {
		if i, ok := byName[name(x)]; ok {
			*prov(&out[i]) = append(*prov(&out[i]), *prov(&x)...)
			continue
		}
		byName[name(x)] = len(out)
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return name(out[i]) < name(out[j]) })
	for i := range out {
		*prov(&out[i]) = uniq(*prov(&out[i]))
	}
	return out
}

func uniq(ps []model.Provenance) []model.Provenance {
	sort.Slice(ps, func(i, j int) bool { return ps[i].Less(ps[j]) })
	out := ps[:0]
	for i, x := range ps {
		if i > 0 && x == ps[i-1] {
			continue
		}
		out = append(out, x)
	}
	return out
}

// nonNil replaces nil slices with empty ones so JSON shows [] consistently.
func nonNil(p *Plan) {
	a := &p.Artifact
	if a.Labels == nil {
		a.Labels = []Label{}
	}
	if a.RpmRepositories == nil {
		a.RpmRepositories = []RpmRepository{}
	}
	if a.RpmGroups == nil {
		a.RpmGroups = []RpmGroupInstall{}
	}
	if a.RpmPackages == nil {
		a.RpmPackages = []RpmInstall{}
	}
	if a.RpmFiles == nil {
		a.RpmFiles = []RpmFileInstall{}
	}
	if a.Binaries == nil {
		a.Binaries = []VerifiedBinaryInstall{}
	}
	if a.Files == nil {
		a.Files = []FileInstall{}
	}
	if a.KernelArguments == nil {
		a.KernelArguments = []KernelArgument{}
	}
	if a.GSettings == nil {
		a.GSettings = []GSettingsVendorDefault{}
	}
	if a.SystemdUnits == nil {
		a.SystemdUnits = []SystemdEnable{}
	}
	if p.Profiles == nil {
		p.Profiles = []Profile{}
	}
	if p.Provisioning.FlatpakRemotes == nil {
		p.Provisioning.FlatpakRemotes = []FlatpakRemote{}
	}
	if p.Provisioning.FlatpakApplications == nil {
		p.Provisioning.FlatpakApplications = []FlatpakPreinstall{}
	}
}

// JSON returns the canonical encoding of the plan.
func (p *Plan) JSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func hasFile(files []FileInstall, p string) bool {
	for _, f := range files {
		if f.Path == p {
			return true
		}
	}
	return false
}

func sortedUnique(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return slices.Compact(out)
}
