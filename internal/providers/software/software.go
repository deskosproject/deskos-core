// Package software provides the software.deskos.org kinds: PackageSet,
// RpmRepository, BinaryArtifact, FlatpakRemote and FlatpakSet.
package software

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/providers/assets"
	"github.com/deskosproject/deskos-core/internal/registry"
)

func gvk(kind string) model.GVK {
	return model.GVK{Group: model.GroupSoftware, Version: model.V1Alpha1, Kind: kind}
}

var (
	PackageSetGVK     = gvk("PackageSet")
	RpmRepositoryGVK  = gvk("RpmRepository")
	BinaryArtifactGVK = gvk("BinaryArtifact")
	FlatpakRemoteGVK  = gvk("FlatpakRemote")
	FlatpakSetGVK     = gvk("FlatpakSet")
)

// Composition domains owned by this package.
var (
	DomainPackages       = compose.Domain{ID: "rpm-package", Label: "RPM package"}
	DomainPackageGroups  = compose.Domain{ID: "package-group", Label: "package group"}
	DomainUnits          = compose.Domain{ID: "systemd-unit", Label: "systemd unit"}
	DomainRepositories   = compose.Domain{ID: "rpm-repository", Label: "RPM repository"}
	DomainRpmFiles       = compose.Domain{ID: "rpm-file", Label: "RPM file"}
	DomainBinaries       = compose.Domain{ID: "binary-destination", Label: "binary install destination"}
	DomainDesktopEntries = compose.Domain{ID: "desktop-entry", Label: "desktop entry"}
	DomainFlatpakRemotes = compose.Domain{ID: "flatpak-remote", Label: "Flatpak remote"}
	DomainFlatpakApps    = compose.Domain{ID: "flatpak-application", Label: "Flatpak application"}
)

// Providers returns the software.deskos.org providers.
func Providers() []registry.Provider {
	return []registry.Provider{packageSet{}, rpmRepository{}, binaryArtifact{}, flatpakRemote{}, flatpakSet{}}
}

var (
	pkgRE     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
	groupRE   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	unitRE    = regexp.MustCompile(`^[A-Za-z0-9@_.:-]+\.(service|socket|timer|path)$`)
	repoIDRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
	versionRE = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+-]*$`)
	sha256RE  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	commandRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)
	appIDRE   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*){2,}$`)
	branchRE  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	collIDRE  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*)+$`)
	textRE    = regexp.MustCompile(`^[^\x00-\x1f\x7f]*$`)
)

// httpsURL validates an https URL without whitespace or control characters.
// allowVars permits the dnf variables $releasever and $basearch.
func httpsURL(raw string, allowVars bool) error {
	if raw == "" {
		return fmt.Errorf("URL is required")
	}
	if !textRE.MatchString(raw) || strings.ContainsAny(raw, " '\"\\`") {
		return fmt.Errorf("URL %q contains forbidden characters", raw)
	}
	check := raw
	if allowVars {
		check = strings.NewReplacer("$releasever", "x", "$basearch", "x").Replace(raw)
	}
	if strings.Contains(check, "$") {
		return fmt.Errorf("URL %q contains unsupported variables (only $releasever and $basearch are allowed)", raw)
	}
	u, err := url.Parse(check)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("URL %q must be an https URL", raw)
	}
	return nil
}

// pinnedURL validates an https URL without variables whose path has no
// floating segment such as latest or main.
func pinnedURL(raw string) error {
	if err := httpsURL(raw, false); err != nil {
		return err
	}
	u, _ := url.Parse(raw)
	for _, seg := range strings.Split(u.Path, "/") {
		if floatingVersions[strings.ToLower(seg)] {
			return fmt.Errorf("URL %q points at a floating location (%q); use a versioned URL", raw, seg)
		}
	}
	return nil
}

// checksum validates a pinned SHA-256 checksum.
func checksum(sum string) error {
	if sum == "" {
		return fmt.Errorf("is required: downloads must be pinned by checksum")
	}
	if !sha256RE.MatchString(sum) {
		return fmt.Errorf("%q must be 64 lowercase hex characters", sum)
	}
	return nil
}

// ---------------------------------------------------------------- PackageSet

// PackageSetSpec describes system software of the managed baseline.
type PackageSetSpec struct {
	Groups       []string      `json:"groups,omitempty"`
	Packages     []string      `json:"packages,omitempty"`
	Repositories []string      `json:"repositories,omitempty"`
	EnableUnits  []string      `json:"enableUnits,omitempty"`
	RpmFiles     []RpmFileSpec `json:"rpmFiles,omitempty"`
}

// RpmFileSpec is a signed RPM downloaded without a repository.
type RpmFileSpec struct {
	URL        string `json:"url"`
	SHA256     string `json:"sha256"`
	GPGKeyFile string `json:"gpgKeyFile"`
}

// RpmFile is the composed definition of one RPM file, keyed by URL.
type RpmFile struct {
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	GPGKey       string `json:"gpgKey"`
	GPGKeySHA256 string `json:"gpgKeySHA256"`
}

// PackageSetObject is the decoded PackageSet with its RPM file keys read.
type PackageSetObject struct {
	PackageSetSpec
	Files []RpmFile
}

const (
	armorBegin = "-----BEGIN PGP PUBLIC KEY BLOCK-----"
	armorEnd   = "-----END PGP PUBLIC KEY BLOCK-----"
)

var armorRE = regexp.MustCompile(`^[\x20-\x7e\n]*$`)

type packageSet struct{}

func (packageSet) GVK() model.GVK { return PackageSetGVK }
func (packageSet) Schema() string { return "software.deskos.org/v1alpha1/packageset.json" }

func (packageSet) Decode(res *model.Resource) error {
	var s PackageSetSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if len(s.Groups) == 0 && len(s.Packages) == 0 && len(s.RpmFiles) == 0 {
		errs.Add(model.Errorf(res, "at least one group, package or RPM file is required"))
	}
	for _, g := range s.Groups {
		if !groupRE.MatchString(g) {
			errs.Add(model.Errorf(res, "invalid package group name %q", g))
		}
	}
	for _, p := range s.Packages {
		if !pkgRE.MatchString(p) {
			errs.Add(model.Errorf(res, "invalid package name %q", p))
		}
	}
	for _, r := range s.Repositories {
		if !model.ValidName(r) {
			errs.Add(model.Errorf(res, "invalid RpmRepository reference %q", r))
		}
	}
	for _, u := range s.EnableUnits {
		if !unitRE.MatchString(u) {
			errs.Add(model.Errorf(res, "invalid systemd unit %q (expected .service, .socket, .timer or .path)", u))
		}
	}
	obj := &PackageSetObject{PackageSetSpec: s}
	seen := map[string]bool{}
	for i, f := range s.RpmFiles {
		if err := pinnedURL(f.URL); err != nil {
			errs.Add(model.Errorf(res, "rpmFiles[%d].url: %v", i, err))
		}
		if seen[f.URL] {
			errs.Add(model.Errorf(res, "rpmFiles[%d].url %q is listed twice", i, f.URL))
		}
		seen[f.URL] = true
		if err := checksum(f.SHA256); err != nil {
			errs.Add(model.Errorf(res, "rpmFiles[%d].sha256 %v", i, err))
		}
		key, err := assets.Read(res, f.GPGKeyFile)
		if err != nil {
			errs.Add(model.Errorf(res, "rpmFiles[%d].gpgKeyFile: %v", i, err))
			continue
		}
		text := string(key.Data)
		if !armorRE.MatchString(text) || !strings.HasPrefix(strings.TrimSpace(text), armorBegin) || !strings.HasSuffix(strings.TrimSpace(text), armorEnd) {
			errs.Add(model.Errorf(res, "rpmFiles[%d].gpgKeyFile %s must be an ASCII-armored OpenPGP public key", i, key.Path))
			continue
		}
		obj.Files = append(obj.Files, RpmFile{URL: f.URL, SHA256: f.SHA256, GPGKey: text, GPGKeySHA256: key.SHA256})
	}
	res.Object = obj
	return errs.Err()
}

func (packageSet) References(res *model.Resource) []model.ObjectRef {
	var refs []model.ObjectRef
	for _, r := range res.Object.(*PackageSetObject).Repositories {
		refs = append(refs, model.ObjectRef{APIVersion: RpmRepositoryGVK.APIVersion(), Kind: RpmRepositoryGVK.Kind, Name: r})
	}
	return refs
}

func (packageSet) Contribute(res *model.Resource, s *compose.Scope) error {
	spec := res.Object.(*PackageSetObject)
	for _, g := range spec.Groups {
		s.AddMember(DomainPackageGroups, g)
	}
	for _, p := range spec.Packages {
		s.AddMember(DomainPackages, p)
	}
	for _, u := range spec.EnableUnits {
		s.AddMember(DomainUnits, u)
	}
	for _, f := range spec.Files {
		s.AddKeyed(DomainRpmFiles, f.URL, f, fmt.Sprintf("sha256 %s, gpg key sha256 %s", f.SHA256, f.GPGKeySHA256))
	}
	return nil
}

// ------------------------------------------------------------- RpmRepository

// RpmRepositorySpec describes an RPM repository. Its signing keys are local
// asset files, read into the plan and placed in the image, so a dnf
// transaction never trusts a key fetched from the network at build time.
type RpmRepositorySpec struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"displayName"`
	BaseURL     string   `json:"baseURL"`
	Enabled     *bool    `json:"enabled,omitempty"`
	GPGCheck    *bool    `json:"gpgCheck,omitempty"`
	GPGKeyFiles []string `json:"gpgKeyFiles,omitempty"`
}

// RepoKey is one repository signing key read from a local asset: its bytes
// travel in the plan and its file is placed in the image.
type RepoKey struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}

// Repository is the normalized, composed repository definition.
type Repository struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"displayName"`
	BaseURL     string    `json:"baseURL"`
	Enabled     bool      `json:"enabled"`
	GPGCheck    bool      `json:"gpgCheck"`
	GPGKeys     []RepoKey `json:"gpgKeys"`
}

type rpmRepository struct{}

func (rpmRepository) GVK() model.GVK { return RpmRepositoryGVK }
func (rpmRepository) Schema() string { return "software.deskos.org/v1alpha1/rpmrepository.json" }

func (rpmRepository) Decode(res *model.Resource) error {
	var s RpmRepositorySpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if !repoIDRE.MatchString(s.ID) {
		errs.Add(model.Errorf(res, "invalid repository id %q", s.ID))
	}
	if s.DisplayName == "" || !textRE.MatchString(s.DisplayName) {
		errs.Add(model.Errorf(res, "displayName is required and must be a single line"))
	}
	if err := httpsURL(s.BaseURL, true); err != nil {
		errs.Add(model.Errorf(res, "baseURL: %v", err))
	}
	r := Repository{ID: s.ID, DisplayName: s.DisplayName, BaseURL: s.BaseURL, Enabled: true, GPGCheck: true}
	for _, f := range s.GPGKeyFiles {
		key, err := assets.Read(res, f)
		if err != nil {
			errs.Add(model.Errorf(res, "gpgKeyFiles: %v", err))
			continue
		}
		text := string(key.Data)
		if !armorRE.MatchString(text) || !strings.HasPrefix(strings.TrimSpace(text), armorBegin) || !strings.HasSuffix(strings.TrimSpace(text), armorEnd) {
			errs.Add(model.Errorf(res, "gpgKeyFiles %s must be an ASCII-armored OpenPGP public key", key.Path))
			continue
		}
		r.GPGKeys = append(r.GPGKeys, RepoKey{Name: path.Base(key.Path), Content: text, SHA256: key.SHA256})
	}
	if s.Enabled != nil {
		r.Enabled = *s.Enabled
	}
	if s.GPGCheck != nil {
		r.GPGCheck = *s.GPGCheck
	}
	if r.GPGCheck && len(r.GPGKeys) == 0 {
		errs.Add(model.Errorf(res, "gpgKeyFiles are required when gpgCheck is enabled"))
	}
	if r.GPGKeys == nil {
		r.GPGKeys = []RepoKey{}
	}
	res.Object = &r
	return errs.Err()
}

func (rpmRepository) Contribute(res *model.Resource, s *compose.Scope) error {
	r := res.Object.(*Repository)
	s.AddKeyed(DomainRepositories, r.ID, *r, model.Canonical(r))
	return nil
}

// ------------------------------------------------------------ BinaryArtifact

// BinaryArtifactSpec describes a checksum-pinned upstream binary or archive.
type BinaryArtifactSpec struct {
	Version      string            `json:"version"`
	Source       BinarySource      `json:"source"`
	Archive      string            `json:"archive"`
	Files        []BinaryMember    `json:"files"`
	DesktopEntry *DesktopEntrySpec `json:"desktopEntry,omitempty"`
}

type BinarySource struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type BinaryMember struct {
	Path        string `json:"path,omitempty"`
	Destination string `json:"destination"`
	Mode        string `json:"mode,omitempty"`
}

// BinaryInstall is one composed, verified binary installation.
type BinaryInstall struct {
	Artifact    string `json:"artifact"`
	Version     string `json:"version"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Archive     string `json:"archive"`
	Member      string `json:"member,omitempty"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
}

// BinaryDir is the only directory BinaryArtifact may install into.
const BinaryDir = "/usr/local/bin"

var floatingVersions = map[string]bool{"latest": true, "stable": true, "current": true, "nightly": true, "master": true, "main": true}

type binaryArtifact struct{}

func (binaryArtifact) GVK() model.GVK { return BinaryArtifactGVK }
func (binaryArtifact) Schema() string { return "software.deskos.org/v1alpha1/binaryartifact.json" }

func (binaryArtifact) Decode(res *model.Resource) error {
	var s BinaryArtifactSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if !versionRE.MatchString(s.Version) || floatingVersions[strings.ToLower(s.Version)] {
		errs.Add(model.Errorf(res, "version %q must be an exact, immutable version", s.Version))
	}
	if err := pinnedURL(s.Source.URL); err != nil {
		errs.Add(model.Errorf(res, "source.url: %v", err))
	}
	if err := checksum(s.Source.SHA256); err != nil {
		errs.Add(model.Errorf(res, "source.sha256 %v", err))
	}
	switch s.Archive {
	case "none":
		if len(s.Files) != 1 || s.Files[0].Path != "" {
			errs.Add(model.Errorf(res, "archive none requires exactly one file entry without path"))
		}
	case "tar.gz":
		for _, f := range s.Files {
			if !safeMember(f.Path) {
				errs.Add(model.Errorf(res, "archive member %q must be a clean relative path", f.Path))
			}
		}
	default:
		errs.Add(model.Errorf(res, "archive %q is not supported (expected none or tar.gz)", s.Archive))
	}
	if len(s.Files) == 0 {
		errs.Add(model.Errorf(res, "at least one file is required"))
	}
	seen := map[string]bool{}
	for i := range s.Files {
		f := &s.Files[i]
		if !safeDestination(f.Destination) {
			errs.Add(model.Errorf(res, "destination %q is not allowed: executables may only be installed directly in %s", f.Destination, BinaryDir))
		}
		if seen[f.Destination] {
			errs.Add(model.Errorf(res, "destination %q is used twice", f.Destination))
		}
		seen[f.Destination] = true
		if f.Mode == "" {
			f.Mode = "0755"
		}
		if f.Mode != "0755" && f.Mode != "0555" {
			errs.Add(model.Errorf(res, "mode %q is not allowed (expected 0755 or 0555)", f.Mode))
		}
	}
	obj := &BinaryArtifactObject{BinaryArtifactSpec: s}
	if s.DesktopEntry != nil {
		obj.Entry = decodeDesktopEntry(res, &s, &errs)
	}
	res.Object = obj
	return errs.Err()
}

func safeMember(p string) bool {
	return p != "" && textRE.MatchString(p) && !strings.ContainsAny(p, "\\*?[") &&
		!strings.HasPrefix(p, "/") && path.Clean(p) == p && p != "." && !strings.HasPrefix(p, "../")
}

func safeDestination(d string) bool {
	return path.Clean(d) == d && path.Dir(d) == BinaryDir && commandRE.MatchString(path.Base(d))
}

func (binaryArtifact) Contribute(res *model.Resource, s *compose.Scope) error {
	obj := res.Object.(*BinaryArtifactObject)
	spec := &obj.BinaryArtifactSpec
	for _, f := range spec.Files {
		b := BinaryInstall{
			Artifact:    res.Metadata.Name,
			Version:     spec.Version,
			URL:         spec.Source.URL,
			SHA256:      spec.Source.SHA256,
			Archive:     spec.Archive,
			Member:      f.Path,
			Destination: f.Destination,
			Mode:        f.Mode,
		}
		s.AddKeyed(DomainBinaries, f.Destination, b, fmt.Sprintf("%s %s from %s (sha256 %s)", b.Artifact, b.Version, b.URL, b.SHA256))
	}
	if e := obj.Entry; e != nil {
		s.AddKeyed(DomainDesktopEntries, e.ID, *e, fmt.Sprintf("%q runs %s, icon %s (sha256 %s)", e.Name, e.Exec, e.IconAsset, e.IconSHA256))
	}
	return nil
}

// ------------------------------------------------------------- FlatpakRemote

// FlatpakRemoteSpec describes a system Flatpak remote. The remote name is
// metadata.name.
type FlatpakRemoteSpec struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	Homepage     string `json:"homepage,omitempty"`
	Comment      string `json:"comment,omitempty"`
	Description  string `json:"description,omitempty"`
	CollectionID string `json:"collectionID,omitempty"`
	GPGKeyFile   string `json:"gpgKeyFile"`
}

// Remote is the composed remote definition.
type Remote struct {
	Name         string `json:"name"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	Homepage     string `json:"homepage,omitempty"`
	Comment      string `json:"comment,omitempty"`
	Description  string `json:"description,omitempty"`
	CollectionID string `json:"collectionID,omitempty"`
	GPGKey       string `json:"gpgKey"`
	GPGKeySHA256 string `json:"gpgKeySHA256"`
}

type flatpakRemote struct{}

func (flatpakRemote) GVK() model.GVK { return FlatpakRemoteGVK }
func (flatpakRemote) Schema() string { return "software.deskos.org/v1alpha1/flatpakremote.json" }

func (flatpakRemote) Decode(res *model.Resource) error {
	var s FlatpakRemoteSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	for name, v := range map[string]string{"title": s.Title, "homepage": s.Homepage, "comment": s.Comment, "description": s.Description} {
		if !textRE.MatchString(v) {
			errs.Add(model.Errorf(res, "%s must be a single line", name))
		}
	}
	if s.Title == "" {
		errs.Add(model.Errorf(res, "title is required"))
	}
	if err := httpsURL(s.URL, false); err != nil {
		errs.Add(model.Errorf(res, "url: %v", err))
	}
	if s.Homepage != "" {
		if err := httpsURL(s.Homepage, false); err != nil {
			errs.Add(model.Errorf(res, "homepage: %v", err))
		}
	}
	if s.CollectionID != "" && !collIDRE.MatchString(s.CollectionID) {
		errs.Add(model.Errorf(res, "invalid collectionID %q", s.CollectionID))
	}
	r := Remote{Name: res.Metadata.Name, Title: s.Title, URL: s.URL, Homepage: s.Homepage, Comment: s.Comment, Description: s.Description, CollectionID: s.CollectionID}
	key, err := assets.Read(res, s.GPGKeyFile)
	if err != nil {
		errs.Add(model.Errorf(res, "gpgKeyFile: %v", err))
	} else if len(key.Data) == 0 || key.Data[0]&0x80 == 0 {
		errs.Add(model.Errorf(res, "gpgKeyFile %s must be a binary OpenPGP public key (not ASCII-armored)", key.Path))
	} else {
		sum := sha256.Sum256(key.Data)
		r.GPGKey = base64.StdEncoding.EncodeToString(key.Data)
		r.GPGKeySHA256 = hex.EncodeToString(sum[:])
	}
	res.Object = &r
	return errs.Err()
}

func (flatpakRemote) Contribute(res *model.Resource, s *compose.Scope) error {
	r := res.Object.(*Remote)
	s.AddKeyed(DomainFlatpakRemotes, r.Name, *r, fmt.Sprintf("%s (gpg key sha256 %s)", r.URL, r.GPGKeySHA256))
	return nil
}

// ---------------------------------------------------------------- FlatpakSet

// FlatpakSetSpec lists system-scoped applications from one remote.
type FlatpakSetSpec struct {
	Remote       string       `json:"remote"`
	Applications []FlatpakApp `json:"applications"`
}

type FlatpakApp struct {
	ID     string `json:"id"`
	Branch string `json:"branch,omitempty"`
}

// App is the composed application definition.
type App struct {
	ID     string `json:"id"`
	Branch string `json:"branch"`
	Remote string `json:"remote"`
}

type flatpakSet struct{}

func (flatpakSet) GVK() model.GVK { return FlatpakSetGVK }
func (flatpakSet) Schema() string { return "software.deskos.org/v1alpha1/flatpakset.json" }

func (flatpakSet) Decode(res *model.Resource) error {
	var s FlatpakSetSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if !model.ValidName(s.Remote) {
		errs.Add(model.Errorf(res, "invalid FlatpakRemote reference %q", s.Remote))
	}
	if len(s.Applications) == 0 {
		errs.Add(model.Errorf(res, "at least one application is required"))
	}
	for i := range s.Applications {
		a := &s.Applications[i]
		if !appIDRE.MatchString(a.ID) {
			errs.Add(model.Errorf(res, "invalid application id %q", a.ID))
		}
		if a.Branch == "" {
			a.Branch = "stable"
		}
		if !branchRE.MatchString(a.Branch) {
			errs.Add(model.Errorf(res, "invalid branch %q for %s", a.Branch, a.ID))
		}
	}
	res.Object = &s
	return errs.Err()
}

func (flatpakSet) References(res *model.Resource) []model.ObjectRef {
	return []model.ObjectRef{{APIVersion: FlatpakRemoteGVK.APIVersion(), Kind: FlatpakRemoteGVK.Kind, Name: res.Object.(*FlatpakSetSpec).Remote}}
}

func (flatpakSet) Contribute(res *model.Resource, s *compose.Scope) error {
	spec := res.Object.(*FlatpakSetSpec)
	for _, a := range spec.Applications {
		app := App{ID: a.ID, Branch: a.Branch, Remote: spec.Remote}
		s.AddKeyed(DomainFlatpakApps, a.ID, app, fmt.Sprintf("%s//%s from remote %s", app.ID, app.Branch, app.Remote))
	}
	return nil
}
