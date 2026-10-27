// Package containerfile renders a Plan into a deterministic bootc build
// context: a Containerfile plus the files it copies.
//
// The backend consumes only the typed Plan. It never reads resource YAML.
// Commands in the Containerfile are generated from typed operations; values
// are validated upstream and single-quoted again here.
package containerfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
)

// Name is the backend identifier used by deskosctl render --backend.
const Name = "containerfile"

// Build context layout.
const (
	ContainerfilePath = "Containerfile"
	PlanPath          = "plan.json"
	ManifestPath      = "generated-manifest.json"
	reposDir          = "repos"
	// repoGPGDir is where repository signing keys are placed in the image;
	// the repository files reference them by file://, never by URL.
	repoGPGDir    = "/etc/pki/rpm-gpg"
	aptKeyringDir = "/etc/apt/keyrings"
	aptSourcesDir = "/etc/apt/sources.list.d"
	rpmKeysDir    = "rpm-keys"
	rootfsDir     = "rootfs"
	imagePlanPath = "/usr/share/deskos/plan.json"
	dconfFileName = "50-deskos"
	schemasDir    = "/usr/share/glib-2.0/schemas"
	// Numbered above distribution overrides (for example 10_ from centos-logos).
	gsettingsOverridePath = schemasDir + "/50_deskos.gschema.override"
	kargsPath             = "/usr/lib/bootc/kargs.d/50-deskos.toml"
	dracutConfPath        = "/usr/lib/dracut/dracut.conf.d/50-deskos.conf"
	systemdUnitDir        = "/usr/lib/systemd/system"
	flatpakRemotesDir     = "/usr/share/flatpak/remotes.d"
	// Where a distribution's own base declares its remotes; a retired one is
	// removed from both places (docs/adr/0017).
	flatpakEtcRemotesDir = "/etc/flatpak/remotes.d"
)

// File is one file of the build context.
type File struct {
	Path string
	Mode fs.FileMode
	Data []byte
}

// Render produces the complete build context in canonical order.
func Render(p *plan.Plan) ([]File, error) {
	mgr, err := managerFacts(p.Platform.PackageManager)
	if err != nil {
		return nil, err
	}
	if len(p.Artifact.RemovePackages) > 0 && mgr.remove == "" {
		return nil, fmt.Errorf("platform %s package manager %s has no tested package-removal path", p.Platform.Name, p.Platform.PackageManager)
	}
	planJSON, err := p.JSON()
	if err != nil {
		return nil, err
	}
	r := &renderer{p: p}
	r.add(PlanPath, 0o644, planJSON)

	seenKeys := map[string]bool{}
	for _, repo := range p.Artifact.RpmRepositories {
		if repo.ID == rhsmRepoID {
			return nil, fmt.Errorf("RPM repository id %q would be written as %s, which RPM builds reserve for subscription-manager", repo.ID, rhsmRepoFile)
		}
		for _, key := range repo.GPGKeys {
			if sum := sha256.Sum256([]byte(key.Content)); hex.EncodeToString(sum[:]) != key.SHA256 {
				return nil, fmt.Errorf("RPM repository %s: gpg key %s does not match its sha256", repo.ID, key.Name)
			}
			name := repoKeyName(key)
			// Two repositories may share a signing key; its file is written
			// once and both repository files point at it.
			if seenKeys[name] {
				continue
			}
			seenKeys[name] = true
			r.add(path.Join(reposDir, repoGPGDir, name), 0o644, []byte(key.Content))
		}
		r.add(path.Join(reposDir, "etc/yum.repos.d", repo.ID+".repo"), 0o644, repoFile(repo))
	}
	seenAptKeys := map[string]bool{}
	for _, src := range p.Artifact.AptRepositories {
		if len(src.GPGKeys) == 0 {
			// Never write a source apt would trust against every key it has.
			return nil, fmt.Errorf("APT source %s: no signing key, and an unsigned source is not rendered", src.ID)
		}
		for _, key := range src.GPGKeys {
			if sum := sha256.Sum256([]byte(key.Content)); hex.EncodeToString(sum[:]) != key.SHA256 {
				return nil, fmt.Errorf("APT source %s: gpg key %s does not match its sha256", src.ID, key.Name)
			}
			name := aptKeyName(key)
			if seenAptKeys[name] {
				continue
			}
			seenAptKeys[name] = true
			r.add(path.Join(reposDir, aptKeyringDir, name), 0o644, []byte(key.Content))
		}
		r.add(path.Join(reposDir, aptSourcesDir, aptSourceName(src.ID)), 0o644, aptSourceFile(src))
	}
	keys := map[string]bool{}
	for _, f := range p.Artifact.RpmFiles {
		if sum := sha256.Sum256([]byte(f.GPGKey)); hex.EncodeToString(sum[:]) != f.GPGKeySHA256 {
			return nil, fmt.Errorf("RPM file %s: gpg key does not match its sha256", f.URL)
		}
		if !keys[f.GPGKeySHA256] {
			keys[f.GPGKeySHA256] = true
			r.add(path.Join(rpmKeysDir, f.GPGKeySHA256+".asc"), 0o644, []byte(f.GPGKey))
		}
	}
	for _, f := range p.Artifact.Files {
		data, err := os.ReadFile(f.AssetFile)
		if err != nil {
			return nil, fmt.Errorf("asset %s: %w", f.Asset, err)
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != f.SHA256 {
			return nil, fmt.Errorf("asset %s changed after planning (sha256 mismatch)", f.Asset)
		}
		r.addImage(f.Path, parseMode(f.Mode), data)
	}
	for _, f := range p.Artifact.GeneratedFiles {
		r.addImage(f.Path, parseMode(f.Mode), []byte(f.Content))
	}
	for _, a := range p.Artifact.TrustAnchors {
		if err := r.addTrustCertificate(a.Asset, a.AssetFile, a.SHA256, a.Path); err != nil {
			return nil, err
		}
	}
	for _, b := range p.Artifact.TrustBlocklist {
		if err := r.addTrustCertificate(b.Asset, b.AssetFile, b.SHA256, b.Path); err != nil {
			return nil, err
		}
	}
	for _, e := range p.Artifact.DesktopEntries {
		r.addImage(e.Path, 0o644, desktopFile(e))
	}
	if len(p.Artifact.KernelArguments) > 0 {
		r.addImage(kargsPath, 0o644, kargsFile(p.Artifact.KernelArguments))
	}
	if ir := p.Artifact.Initramfs; ir != nil {
		r.addImage(dracutConfPath, 0o644, dracutConf(ir))
	}
	if t := p.Artifact.PlymouthTheme; t != nil {
		if p.Artifact.Initramfs == nil {
			return nil, fmt.Errorf("boot splash theme %s needs an initramfs regeneration", t.Name)
		}
		r.addImage(path.Join(t.Dir, t.Name+".plymouth"), 0o644, plymouthTheme(t))
	}
	if len(p.Artifact.GSettings) > 0 {
		r.addImage(gsettingsOverridePath, 0o644, gsettingsOverride(p.Artifact.GSettings))
	}
	if db := p.Artifact.Dconf; db != nil {
		// The greeter does not read the session's database: GDM has its own dconf
		// profile, so a login-screen key written beside the desktop defaults is
		// installed and never shown. Split the keys where each reader looks.
		session, greeter := splitLoginScreen(db.Defaults)
		dir := path.Join("/etc/dconf/db", db.Name+".d")
		if len(session) > 0 {
			r.addImage(path.Join(dir, dconfFileName), 0o644, dconfKeyfile(session))
		}
		if len(greeter) > 0 {
			gdDir := path.Join("/etc/dconf/db", db.LoginScreen+".d")
			r.addImage(path.Join(gdDir, dconfFileName), 0o644, dconfKeyfile(greeter))
		}
		if len(db.Locks) > 0 {
			r.addImage(path.Join(dir, "locks", dconfFileName), 0o644, dconfLocks(db))
		}
	}
	for _, rm := range p.Provisioning.FlatpakRemotes {
		r.addImage(path.Join(flatpakRemotesDir, rm.Name+".flatpakrepo"), 0o644, flatpakRepoFile(rm))
	}
	for _, app := range p.Provisioning.FlatpakApplications {
		r.addImage(path.Join("/usr/share/flatpak/preinstall.d", app.ID+".preinstall"), 0o644, preinstallFile(app))
	}
	if u := p.Provisioning.FlatpakPreinstallUnit; u != "" {
		r.addImage(path.Join(systemdUnitDir, u), 0o644, preinstallUnit())
	}
	if u := p.Provisioning.FlatpakAuthenticatorUnit; u != "" {
		r.addImage(path.Join(systemdUnitDir, u), 0o644,
			flatpakAuthenticatorUnit(p.Provisioning.FlatpakRemotes, p.Provisioning.FlatpakPreinstallUnit))
	}
	for _, u := range p.Artifact.ScheduledUpdates {
		svc, timer, err := updateUnits(u)
		if err != nil {
			return nil, err
		}
		r.addImage(path.Join(systemdUnitDir, u.Service), 0o644, svc)
		if timer != nil {
			r.addImage(path.Join(systemdUnitDir, u.Timer), 0o644, timer)
		}
	}
	r.addImage(imagePlanPath, 0o644, planJSON)

	r.add(ContainerfilePath, 0o644, containerfile(p, r.hasRepos, r.hasRootfs, mgr))

	sort.Slice(r.files, func(i, j int) bool { return r.files[i].Path < r.files[j].Path })
	for i := 1; i < len(r.files); i++ {
		if r.files[i].Path == r.files[i-1].Path {
			return nil, fmt.Errorf("build context path %s generated twice", r.files[i].Path)
		}
	}
	manifest, err := manifestFile(p, r.files)
	if err != nil {
		return nil, err
	}
	r.files = append(r.files, File{Path: ManifestPath, Mode: 0o644, Data: manifest})
	sort.Slice(r.files, func(i, j int) bool { return r.files[i].Path < r.files[j].Path })
	return r.files, nil
}

type renderer struct {
	p         *plan.Plan
	files     []File
	hasRepos  bool
	hasRootfs bool
}

func (r *renderer) add(p string, mode fs.FileMode, data []byte) {
	if strings.HasPrefix(p, reposDir+"/") {
		r.hasRepos = true
	}
	r.files = append(r.files, File{Path: p, Mode: mode, Data: data})
}

func (r *renderer) addImage(imagePath string, mode fs.FileMode, data []byte) {
	r.hasRootfs = true
	r.add(path.Join(rootfsDir, strings.TrimPrefix(imagePath, "/")), mode, data)
}

// addTrustCertificate copies one certificate asset the plan validated to the
// image path the platform chose, re-checking the digest so a changed file
// fails the render instead of shipping unnoticed.
func (r *renderer) addTrustCertificate(asset, assetFile, sha, imagePath string) error {
	data, err := os.ReadFile(assetFile)
	if err != nil {
		return fmt.Errorf("asset %s: %w", asset, err)
	}
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != sha {
		return fmt.Errorf("asset %s changed after planning (sha256 mismatch)", asset)
	}
	r.addImage(imagePath, 0o644, data)
	return nil
}

func parseMode(s string) fs.FileMode {
	m, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0o644
	}
	return fs.FileMode(m)
}

func manifestFile(p *plan.Plan, files []File) ([]byte, error) {
	type entry struct {
		Path   string `json:"path"`
		Mode   string `json:"mode"`
		SHA256 string `json:"sha256"`
	}
	m := struct {
		Format      string  `json:"format"`
		Backend     string  `json:"backend"`
		Workstation string  `json:"workstation"`
		Files       []entry `json:"files"`
	}{Format: "manifest.deskos.org/v1alpha1", Backend: Name, Workstation: p.Workstation.Name}
	for _, f := range files {
		sum := sha256.Sum256(f.Data)
		m.Files = append(m.Files, entry{Path: f.Path, Mode: fmt.Sprintf("%04o", f.Mode.Perm()), SHA256: hex.EncodeToString(sum[:])})
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ------------------------------------------------------------- generated files

const generatedHeader = "# Generated by deskosctl. Do not edit; change the DeskOS resources instead.\n"

func repoFile(r plan.RpmRepository) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	fmt.Fprintf(&b, "[%s]\n", r.ID)
	fmt.Fprintf(&b, "name=%s\n", r.DisplayName)
	fmt.Fprintf(&b, "baseurl=%s\n", r.BaseURL)
	fmt.Fprintf(&b, "enabled=%d\n", boolInt(r.Enabled))
	fmt.Fprintf(&b, "gpgcheck=%d\n", boolInt(r.GPGCheck))
	if len(r.GPGKeys) > 0 {
		refs := make([]string, 0, len(r.GPGKeys))
		for _, k := range r.GPGKeys {
			refs = append(refs, "file://"+path.Join(repoGPGDir, repoKeyName(k)))
		}
		fmt.Fprintf(&b, "gpgkey=%s\n", strings.Join(refs, " "))
	}
	return []byte(b.String())
}

// repoKeyName is the file name a repository's key takes in the image. It is
// derived from the key's sha256, never from the asset name, so a resource
// cannot inject repository directives through a crafted file name.
func repoKeyName(k plan.RepoKey) string {
	return k.SHA256 + ".asc"
}

// aptSourceName is the file an APT source is written to. The name is prefixed
// so it cannot collide with a source the base or a package owns (Ubuntu ships
// ubuntu.sources), and the id it carries is restricted to the characters apt
// accepts in this directory — anything else and apt ignores the file in
// silence, which is why the provider refuses those ids.
func aptSourceName(id string) string { return "deskos-" + id + ".sources" }

// aptKeyName is the file name an APT keyring takes in the image, derived from
// the key's sha256 for the same reason repoKeyName is.
func aptKeyName(k plan.RepoKey) string { return "deskos-" + k.SHA256 + ".asc" }

// aptSourceFile writes one deb822 source. Apt reads these fields itself, and
// the display name goes where apt documents that a third party describes a
// repository to a user. Signed-By is a whitespace-separated list of absolute
// keyring paths and is what makes the source signed: apt then requires the
// repository's own metadata to carry a signature by one of those keys, so a
// repository that stops signing stops working instead of quietly going
// unsigned.
func aptSourceFile(r plan.AptRepository) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	b.WriteString("Types: deb\n")
	fmt.Fprintf(&b, "URIs: %s\n", r.BaseURL)
	fmt.Fprintf(&b, "Suites: %s\n", r.Suite)
	if len(r.Components) > 0 {
		fmt.Fprintf(&b, "Components: %s\n", strings.Join(r.Components, " "))
	}
	if len(r.Architectures) > 0 {
		fmt.Fprintf(&b, "Architectures: %s\n", strings.Join(r.Architectures, " "))
	}
	if len(r.GPGKeys) > 0 {
		refs := make([]string, 0, len(r.GPGKeys))
		for _, k := range r.GPGKeys {
			refs = append(refs, path.Join(aptKeyringDir, aptKeyName(k)))
		}
		fmt.Fprintf(&b, "Signed-By: %s\n", strings.Join(refs, " "))
	}
	if r.DisplayName != "" {
		fmt.Fprintf(&b, "Description: %s\n", r.DisplayName)
	}
	return []byte(b.String())
}

// desktopFile writes a Desktop Entry Specification file; Exec is a validated
// /usr/local/bin path without reserved characters, so it needs no quoting.
func desktopFile(e plan.DesktopEntry) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	fmt.Fprintf(&b, "Name=%s\n", desktopEscape(e.Name))
	if e.Comment != "" {
		fmt.Fprintf(&b, "Comment=%s\n", desktopEscape(e.Comment))
	}
	fmt.Fprintf(&b, "Exec=%s\n", e.Exec)
	fmt.Fprintf(&b, "TryExec=%s\n", e.Exec)
	fmt.Fprintf(&b, "Icon=%s\n", e.Icon)
	b.WriteString("Terminal=false\n")
	fmt.Fprintf(&b, "Categories=%s;\n", strings.Join(e.Categories, ";"))
	if e.StartupWMClass != "" {
		fmt.Fprintf(&b, "StartupWMClass=%s\n", e.StartupWMClass)
	}
	return []byte(b.String())
}

// desktopEscape escapes backslashes; control characters and surrounding
// whitespace, which need the other escapes, are rejected upstream.
func desktopEscape(s string) string { return strings.ReplaceAll(s, `\`, `\\`) }

// loginScreenPrefix is the settings tree GDM reads, and the session does not.
const loginScreenPrefix = "/org/gnome/login-screen/"

// splitLoginScreen separates the keys the greeter reads from the rest. A key
// under the login-screen tree belongs to the database GDM reads, not to the
// session's.
func splitLoginScreen(defaults []plan.DconfDefault) (session, greeter []plan.DconfDefault) {
	for _, d := range defaults {
		if strings.HasPrefix(d.Key, loginScreenPrefix) {
			greeter = append(greeter, d)
			continue
		}
		session = append(session, d)
	}
	return session, greeter
}

func dconfKeyfile(defaults []plan.DconfDefault) []byte {
	groups := map[string][]string{}
	for _, d := range defaults {
		dir, key := path.Split(d.Key)
		g := strings.Trim(dir, "/")
		groups[g] = append(groups[g], key+"="+d.Value)
	}
	names := make([]string, 0, len(groups))
	for g := range groups {
		names = append(names, g)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString(generatedHeader)
	for i, g := range names {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "[%s]\n", g)
		lines := groups[g]
		sort.Slice(lines, func(i, j int) bool {
			ki, _, _ := strings.Cut(lines[i], "=")
			kj, _, _ := strings.Cut(lines[j], "=")
			return ki < kj
		})
		for _, l := range lines {
			b.WriteString(l + "\n")
		}
	}
	return []byte(b.String())
}

func gsettingsOverride(defaults []plan.GSettingsVendorDefault) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	schema := ""
	for _, d := range defaults {
		if d.Schema != schema {
			if schema != "" {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "[%s]\n", d.Schema)
			schema = d.Schema
		}
		fmt.Fprintf(&b, "%s=%s\n", d.Key, d.Value)
	}
	return []byte(b.String())
}

func kargsFile(args []plan.KernelArgument) []byte {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = strconv.Quote(a.Arg)
	}
	return []byte(generatedHeader + "kargs = [" + strings.Join(quoted, ", ") + "]\n")
}

func dracutConf(ir *plan.InitramfsRegeneration) []byte {
	return []byte(generatedHeader + "add_dracutmodules+=\" " + strings.Join(ir.DracutModules, " ") + " \"\n")
}

// plymouthTheme is the stock spinner theme's two-step layout with the
// theme's own ImageDir, so the watermark is the theme's and no mode uses
// the firmware (BGRT) logo.
func plymouthTheme(t *plan.PlymouthTheme) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	b.WriteString("[Plymouth Theme]\n")
	fmt.Fprintf(&b, "Name=%s\n", t.Name)
	b.WriteString("Description=DeskOS spinner with a DeskOS watermark\n")
	b.WriteString("ModuleName=two-step\n\n")
	b.WriteString("[two-step]\n")
	b.WriteString("Font=Red Hat Text 12\n")
	b.WriteString("TitleFont=Red Hat Text Light 30\n")
	fmt.Fprintf(&b, "ImageDir=%s\n", t.Dir)
	b.WriteString(`DialogHorizontalAlignment=.5
DialogVerticalAlignment=.382
TitleHorizontalAlignment=.5
TitleVerticalAlignment=.382
HorizontalAlignment=.5
VerticalAlignment=.7
WatermarkHorizontalAlignment=.5
WatermarkVerticalAlignment=.96
Transition=none
TransitionDuration=0.0
BackgroundStartColor=0x000000
BackgroundEndColor=0x000000
ProgressBarBackgroundColor=0x606060
ProgressBarForegroundColor=0xffffff
MessageBelowAnimation=true
`)
	for _, mode := range []string{"boot-up", "shutdown", "reboot"} {
		fmt.Fprintf(&b, "\n[%s]\nUseEndAnimation=false\nUseFirmwareBackground=false\n", mode)
	}
	for _, m := range []struct{ mode, title string }{
		{"updates", "Installing Updates..."},
		{"system-upgrade", "Upgrading System..."},
		{"firmware-upgrade", "Upgrading Firmware..."},
		{"system-reset", "Resetting System..."},
	} {
		fmt.Fprintf(&b, "\n[%s]\nSuppressMessages=true\nProgressBarShowPercentComplete=true\nUseProgressBar=true\nUseFirmwareBackground=false\nTitle=%s\nSubTitle=Do not turn off your computer\n", m.mode, m.title)
	}
	return []byte(b.String())
}

func dconfLocks(db *plan.DconfDatabase) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	seen := map[string]bool{}
	for _, l := range db.Locks {
		if !seen[l.Key] {
			b.WriteString(l.Key + "\n")
			seen[l.Key] = true
		}
	}
	return []byte(b.String())
}

// flatpakRepoFile renders the /usr/share/flatpak/remotes.d entry of a remote.
// The file format is documented in flatpakrepo(5): it carries a URL, a title
// and an OSTree signing key, but it cannot express an OCI authenticator or the
// credentials behind it, which is why those get their own unit.
func flatpakRepoFile(r plan.FlatpakRemote) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	b.WriteString("[Flatpak Repo]\n")
	b.WriteString("Version=1\n")
	fmt.Fprintf(&b, "Title=%s\n", r.Title)
	fmt.Fprintf(&b, "Url=%s\n", r.RemoteURL())
	if r.Homepage != "" {
		fmt.Fprintf(&b, "Homepage=%s\n", r.Homepage)
	}
	if r.Comment != "" {
		fmt.Fprintf(&b, "Comment=%s\n", r.Comment)
	}
	if r.Description != "" {
		fmt.Fprintf(&b, "Description=%s\n", r.Description)
	}
	// An OCI remote is verified by the registry and its images rather than by
	// an OSTree summary key, and it has no collection to deploy sideloads into.
	if r.Transport != plan.FlatpakTransportOCI {
		if r.CollectionID != "" {
			fmt.Fprintf(&b, "DeploySideloadCollectionID=%s\n", r.CollectionID)
		}
		fmt.Fprintf(&b, "GPGKey=%s\n", r.GPGKey)
	}
	return []byte(b.String())
}

// flatpakAuthenticatorUnit points OCI remotes at their authenticator. The
// remote file cannot carry one, so it is applied on the machine once the remote
// exists: remote-add is a no-op when the remotes.d entry was already imported,
// and remote-modify stores the authenticator in the remote's own config. The
// credentials are not part of the artifact; the authenticator reads them from
// the machine. The unit only runs local commands, so it adds no network wait to
// any boot target, and it is ordered before the preinstall unit when there is
// one, because the apps that unit installs may need the authenticator.
func flatpakAuthenticatorUnit(remotes []plan.FlatpakRemote, preinstallUnit string) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	b.WriteString(`[Unit]
Description=Point OCI Flatpak remotes at their authenticator
Documentation=man:flatpak-remote-modify(1)
After=multi-user.target
`)
	if preinstallUnit != "" {
		fmt.Fprintf(&b, "Before=%s\n", preinstallUnit)
	}
	b.WriteString(`
[Service]
Type=oneshot
`)
	for _, r := range remotes {
		if r.Authenticator == "" {
			continue
		}
		fmt.Fprintf(&b, "ExecStart=/usr/bin/flatpak remote-add --system --if-not-exists --from %s %s\n",
			r.Name, path.Join(flatpakRemotesDir, r.Name+".flatpakrepo"))
		fmt.Fprintf(&b, "ExecStart=/usr/bin/flatpak remote-modify --system --authenticator-name=%s %s\n",
			r.Authenticator, r.Name)
	}
	b.WriteString(`
[Install]
WantedBy=multi-user.target
`)
	return []byte(b.String())
}

func preinstallFile(a plan.FlatpakPreinstall) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	fmt.Fprintf(&b, "[Flatpak Preinstall %s]\n", a.ID)
	fmt.Fprintf(&b, "Branch=%s\n", a.Branch)
	b.WriteString("IsRuntime=false\n")
	if a.CollectionID != "" {
		fmt.Fprintf(&b, "CollectionID=%s\n", a.CollectionID)
	}
	return []byte(b.String())
}

// preinstallUnit runs upstream `flatpak preinstall` once per boot. Ordering it
// after multi-user.target stops that target from waiting on network-online for
// it, Type=exec avoids waiting for downloads, and GDM is not ordered after it.
//
// First boots fail for two reasons that a second boot cures: a pull interrupted
// mid-download leaves a ref `flatpak preinstall` will not refresh, and the
// Flathub summary can be stale enough that a declared app is "not found in the
// remote". `flatpak repair` removes the broken ref and `flatpak update
// --appstream` refreshes the summary before each attempt. The unit then retries
// until it succeeds, because a laptop may connect its network minutes after the
// unit starts; there is no start limit, so it keeps retrying (once per minute)
// instead of waiting for the next boot. It stops within 20 s (TimeoutStopSec),
// so a shutdown during the first-boot download does not hang on the stop job.
// Nothing else waits for it.
func preinstallUnit() []byte {
	return []byte(generatedHeader + `[Unit]
Description=Install operating system Flatpaks declared in preinstall.d
Documentation=man:flatpak-preinstall(1)
Wants=network-online.target
After=network-online.target multi-user.target
StartLimitIntervalSec=0

[Service]
Type=exec
ExecStartPre=-/usr/bin/flatpak repair --system
ExecStartPre=-/usr/bin/flatpak update --appstream --system
ExecStart=/usr/bin/flatpak preinstall --system --assumeyes --noninteractive
Restart=on-failure
RestartSec=60
# A shutdown during the first-boot download must not wait systemd's default
# 90 s: the container image's own boot check sends ACPI powerdown while the
# unit is still pulling, and the boot hangs on the stop job. The next boot
# repairs and retries what was interrupted.
TimeoutStopSec=20

[Install]
WantedBy=multi-user.target
`)
}

var (
	updateUnitRE  = regexp.MustCompile(`^deskos-[a-z0-9-]+\.(service|timer)$`)
	updateDelayRE = regexp.MustCompile(`^[1-9][0-9]*(min|h)$`)
)

// curlFetch is how the build downloads something it will verify afterwards, and it is
// the same everywhere on purpose: a fetch that stalls or blips is the same failure
// whether it brings a binary artifact or an rpm file, and two hand-copied flag lists
// are two lists that drift. `--retry` retries transient failures and never a 404 or a
// checksum that disagrees, because those are not transient; `--connect-timeout` and
// `--speed-limit`/`--speed-time` bound the attempts that are not errors but waits, so
// neither a host that never answers nor a server that accepts and goes quiet can hang a
// build until the job's own timeout. Measured 2026-10-18 in the Ubuntu base: 30s for a
// host that does not answer the handshake, 60s for a transfer under 1 KB/s.
const curlFetch = "curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \\\n" +
	"        --retry 3 --retry-delay 2 --retry-connrefused --retry-max-time 90 \\\n" +
	"        --connect-timeout 30 --speed-limit 1000 --speed-time 60"

// extensionExtract is the fixed program that validates and extracts one GNOME
// Shell extension archive. It runs before anything lands in the image: member
// names, types and sizes are checked here, and so are the UUID and the Shell
// version the archive claims to support (docs/adr/0021). It is core code, not a
// resource field: no plan can supply a command.
const extensionExtract = `import json,os,sys,zipfile; z=zipfile.ZipFile(sys.argv[1]); out=sys.argv[2]; uuid=sys.argv[3]; major=sys.argv[4]; infos=z.infolist(); len(infos)<=2000 or sys.exit("extension archive has too many entries"); keys=[os.path.normpath(i.filename) for i in infos]; len(set(keys))==len(keys) or sys.exit("extension archive repeats a path"); all(not k.startswith("/") and k!=".." and not k.startswith("../") for k in keys) or sys.exit("unsafe path in extension archive"); all(((i.external_attr>>16)&0o170000) in (0,0o040000,0o100000) for i in infos) or sys.exit("non-regular entry in extension archive"); all(i.file_size<=20*1024*1024 for i in infos) or sys.exit("member too large in extension archive"); sum(i.file_size for i in infos)<=50*1024*1024 or sys.exit("extension archive is too large"); meta=json.loads(z.read("metadata.json")); meta.get("uuid")==uuid or sys.exit("extension archive uuid does not match "+uuid); major in (meta.get("shell-version") or []) or sys.exit("extension does not support GNOME Shell "+major+": it declares "+str(meta.get("shell-version"))); z.extractall(out)`

// updateUnits renders the service and timer of one scheduled update. The
// commands are fixed per kind and only stage or update; none reboots.
func updateUnits(u plan.ScheduledUpdate) (service, timer []byte, err error) {
	if u.Kind == plan.UpdateImageApply {
		return applyUnits(u)
	}
	var desc, doc, exec, cond string
	switch u.Kind {
	case plan.UpdateImage:
		desc, doc = "Download and stage the next image without rebooting", "man:bootc-upgrade(8)"
		exec = "/usr/bin/bootc upgrade --quiet"
		// No ConditionPathExists=/run/ostree-booted: only the ostree backend
		// creates that file. These units ship in a bootc image, and bootc
		// itself refuses a machine it cannot upgrade.
	case plan.UpdateFlatpak:
		desc, doc = "Update system Flatpak applications and runtimes", "man:flatpak-update(1)"
		exec = "/usr/bin/flatpak update --system --noninteractive --assumeyes"
	default:
		return nil, nil, fmt.Errorf("scheduled update kind %q is not supported", u.Kind)
	}
	if !updateUnitRE.MatchString(u.Service) || !strings.HasSuffix(u.Service, ".service") {
		return nil, nil, fmt.Errorf("scheduled %s update: invalid service name %q", u.Kind, u.Service)
	}
	if u.RequireACPower {
		cond += "ConditionACPower=true\n"
	}
	service = []byte(generatedHeader + "[Unit]\n" +
		"Description=" + desc + "\n" +
		"Documentation=" + doc + "\n" +
		cond +
		"Wants=network-online.target\n" +
		"After=network-online.target\n\n" +
		"[Service]\n" +
		"Type=oneshot\n" +
		"ExecStart=" + exec + "\n")
	// A service with no timer runs on demand (an endpoint command), not on a
	// schedule.
	if u.Timer == "" && u.Schedule == "" {
		return service, nil, nil
	}
	if u.Schedule != "daily" && u.Schedule != "weekly" {
		return nil, nil, fmt.Errorf("scheduled %s update: schedule %q is not daily or weekly", u.Kind, u.Schedule)
	}
	if !updateUnitRE.MatchString(u.Timer) || !strings.HasSuffix(u.Timer, ".timer") {
		return nil, nil, fmt.Errorf("scheduled %s update: invalid timer name %q", u.Kind, u.Timer)
	}
	if !updateDelayRE.MatchString(u.RandomizedDelay) {
		return nil, nil, fmt.Errorf("scheduled %s update: invalid randomized delay %q", u.Kind, u.RandomizedDelay)
	}
	timer = []byte(generatedHeader + "[Unit]\n" +
		"Description=" + desc + " (" + u.Schedule + ")\n" +
		"Documentation=" + doc + "\n\n" +
		"[Timer]\n" +
		"OnCalendar=" + u.Schedule + "\n" +
		"RandomizedDelaySec=" + u.RandomizedDelay + "\n" +
		"Persistent=true\n" +
		"Unit=" + u.Service + "\n\n" +
		"[Install]\n" +
		"WantedBy=timers.target\n")
	return service, timer, nil
}

// applyUnits renders the explicit apply timer. The service runs the fixed
// --apply command only after deskos has confirmed that the current local time
// still falls inside the requested day and window; a missed calendar event
// after suspend must not become a surprise reboot at wake-up.
func applyUnits(u plan.ScheduledUpdate) (service, timer []byte, err error) {
	if !updateUnitRE.MatchString(u.Service) || !strings.HasSuffix(u.Service, ".service") {
		return nil, nil, fmt.Errorf("scheduled apply: invalid service name %q", u.Service)
	}
	if !updateUnitRE.MatchString(u.Timer) || !strings.HasSuffix(u.Timer, ".timer") {
		return nil, nil, fmt.Errorf("scheduled apply: invalid timer name %q", u.Timer)
	}
	if len(u.Days) == 0 {
		return nil, nil, fmt.Errorf("scheduled apply: no days")
	}
	days, err := systemdDays(u.Days)
	if err != nil {
		return nil, nil, err
	}
	dayList := strings.Join(days, ",")
	start := u.At
	window := ""
	if u.Window != nil {
		start = u.Window.Start
		window = u.Window.Duration
		if _, err := time.ParseDuration(window); err != nil {
			return nil, nil, fmt.Errorf("scheduled apply: invalid window duration %q", window)
		}
	}
	if u.At == "" && u.Window == nil {
		return nil, nil, fmt.Errorf("scheduled apply: neither at nor window")
	}
	if u.At != "" && u.Window != nil {
		return nil, nil, fmt.Errorf("scheduled apply: both at and window")
	}
	if _, err := time.Parse("15:04", start); err != nil {
		return nil, nil, fmt.Errorf("scheduled apply: invalid start %q", start)
	}
	if u.Timezone != "" {
		if _, err := time.LoadLocation(u.Timezone); err != nil {
			return nil, nil, fmt.Errorf("scheduled apply: unknown time zone %q", u.Timezone)
		}
	}
	cond := ""
	if u.RequireACPower {
		cond += "ConditionACPower=true\n"
	}
	condition := "/usr/bin/deskos window --days " + shortenDays(days)
	if u.At != "" {
		condition += " --at " + u.At + " --grace 5m"
	} else {
		condition += " --start " + start + " --duration " + window
	}
	if u.Timezone != "" {
		condition += " --timezone " + u.Timezone
	}
	service = []byte(generatedHeader + "[Unit]\n" +
		"Description=Apply the scheduled DeskOS image and reboot\n" +
		"Documentation=man:bootc-upgrade(8)\n" +
		cond +
		"Wants=network-online.target\n" +
		"After=network-online.target\n\n" +
		"[Service]\n" +
		"Type=oneshot\n" +
		"ExecCondition=" + condition + "\n" +
		"ExecStart=/usr/bin/bootc upgrade --apply --quiet\n")
	timer = []byte(generatedHeader + "[Unit]\n" +
		"Description=Apply the scheduled DeskOS image and reboot\n" +
		"Documentation=man:bootc-upgrade(8)\n\n" +
		"[Timer]\n" +
		"OnCalendar=" + dayList + " *-*-* " + start + ":00")
	if u.Timezone != "" {
		timer = append(timer, []byte(" "+u.Timezone)...)
	}
	timer = append(timer, []byte("\nAccuracySec=1min\n")...)
	if window != "" {
		timer = append(timer, []byte("RandomizedDelaySec="+window+"\n")...)
	}
	timer = append(timer, []byte("Persistent=false\n"+
		"Unit="+u.Service+"\n\n"+
		"[Install]\n"+
		"WantedBy=timers.target\n")...)
	return service, timer, nil
}

// systemdDays returns the canonical Mon..Sun list used by OnCalendar.
func systemdDays(days []string) ([]string, error) {
	order := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	short := map[string]string{
		"monday": "Mon", "tuesday": "Tue", "wednesday": "Wed",
		"thursday": "Thu", "friday": "Fri", "saturday": "Sat", "sunday": "Sun",
	}
	seen := map[string]bool{}
	var out []string
	for _, day := range order {
		for _, requested := range days {
			if requested != day {
				continue
			}
			if seen[day] {
				return nil, fmt.Errorf("scheduled apply: day %q is listed twice", day)
			}
			seen[day] = true
			out = append(out, short[day])
		}
	}
	if len(out) != len(days) {
		return nil, fmt.Errorf("scheduled apply: unknown day in %v", days)
	}
	return out, nil
}

func shortenDays(days []string) string {
	out := make([]string, len(days))
	for i, day := range days {
		out[i] = strings.ToLower(day[:3])
	}
	return strings.Join(out, ",")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --------------------------------------------------------------- Containerfile

func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'" }

func containerfile(p *plan.Plan, hasRepos, hasRootfs bool, mgr pkgManager) []byte {
	a := p.Artifact
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# Generated by deskosctl from plan.json. Do not edit.\n#\n")
	w("# Workstation: %s\n", p.Workstation.Name)
	w("# Platform:    %s (%s)\n", p.Platform.Name, p.Platform.DisplayName)
	if !p.Platform.Redistributable {
		w("#\n# This artifact must not be publicly redistributed.\n")
	}
	if p.Platform.RequiresSubscription {
		w("# Building requires an entitled build host; credentials are supplied by\n# the build environment and are never part of this build context.\n")
	}
	w("\nFROM %s\n", a.BaseImage.Ref)

	// Repositories are on disk before the first package transaction: a
	// transaction that installs a member of a group, or a package, from a
	// declared repository needs the repository file to exist by then.
	if hasRepos {
		w("\n# Package repositories\nCOPY %s/ /\n", reposDir)
	}

	if len(a.PackageGroups) > 0 {
		w("\n# Package groups\n")
		pkgTransaction(w, mgr, nil, mgr.groupInstall, groupArgs(mgr, a.PackageGroups))
	}

	if len(a.Packages) > 0 {
		var args []string
		if p.Platform.PackageManager != model.PackageManagerDeb {
			args = enableRepos(a.RpmRepositories)
		}
		for _, pkg := range a.Packages {
			args = append(args, pkg.Name)
		}
		w("\n# Packages\n")
		pkgTransaction(w, mgr, nil, mgr.install, args)
	}

	if len(a.RpmFiles) > 0 {
		pre := []string{"mkdir -m 0700 " + rpmFilesDir}
		args := enableRepos(a.RpmRepositories)
		for _, f := range a.RpmFiles {
			rpm := path.Join(rpmFilesDir, f.SHA256+".rpm")
			// A keyring per file trusts only that file's key and leaves the image's rpmdb keys unchanged.
			rpmkeys := "rpmkeys --dbpath " + shq(path.Join(rpmFilesDir, "keyring-"+f.SHA256)) + " --define '_keyring rpmdb'"
			pre = append(pre,
				fmt.Sprintf("%s \\\n        -o %s %s", curlFetch, shq(rpm), shq(f.URL)),
				fmt.Sprintf("echo %s | sha256sum --check --strict --quiet -", shq(f.SHA256+"  "+rpm)),
				fmt.Sprintf("%s --import %s", rpmkeys, shq(path.Join(rpmKeysImageDir, f.GPGKeySHA256+".asc"))),
				// Without signature level, --checksig accepts an unsigned package whose digests match.
				fmt.Sprintf("%s --define '_pkgverify_level signature' --checksig %s", rpmkeys, shq(rpm)))
			args = append(args, rpm)
		}
		w("\n# RPM files (sha256- and signature-verified)\nCOPY %s/ %s/\n", rpmKeysDir, rpmKeysImageDir)
		pkgTransaction(w, mgr, pre, mgr.install, args, "rm -rf "+rpmFilesDir)
	}

	if len(a.RemovePackages) > 0 {
		w("\n# Packages the platform must not carry\n")
		pkgTransaction(w, mgr, nil, removeCommand(mgr, a.RemovePackages), nil)
	}

	for _, group := range groupBinaries(a.Binaries) {
		first := group[0]
		w("\n# Binary artifact %s %s (sha256-verified)\n", first.Artifact, first.Version)
		w("RUN set -eu; \\\n    tmp=\"$(mktemp -d)\"; \\\n")
		// The fetch retries transient failures and bounds the attempts that are not
		// errors but waits; see curlFetch for what was measured and why it softens
		// nothing (the checksum below still decides).
		w("    %s \\\n        -o \"$tmp/download\" %s; \\\n", curlFetch, shq(first.URL))
		w("    echo %s | sha256sum --check --strict --quiet -; \\\n", shq(first.SHA256+"  ")+`"$tmp/download"`)
		for _, bin := range group {
			src := `"$tmp/download"`
			if bin.Archive == "tar.gz" {
				w("    tar -xzf \"$tmp/download\" -C \"$tmp\" --no-same-owner -- %s; \\\n", shq(bin.Member))
				// The member must be a plain file. tar extracts a symlink or
				// hardlink member as such, and install would then copy whatever
				// it points at (possibly a file from the build environment).
				src = `"$tmp"/` + shq(bin.Member)
				w("    [ -f %s ] && [ ! -L %s ] && [ \"$(stat -c %%h %s)\" = 1 ] || { echo %s >&2; exit 1; }; \\\n",
					src, src, src, shq("archive member "+bin.Member+" is not a regular file"))
			}
			// The destination's directory must exist before install can write into it.
			// `install -D` creates the parents, but it cannot traverse a `/usr/local`
			// that is a dangling symlink: Debian and Ubuntu ship
			// `/usr/local -> ../var/usrlocal`, and that target is created at boot by a
			// bootc tmpfiles rule rather than by the base image (measured 2026-10-18 on
			// the Ubuntu base, whose `install -D ... /usr/local/bin/<binary>` failed with
			// "cannot create directory"). `readlink -m` canonicalises without requiring
			// existence, so the directory is created where the file will really land.
			w("    mkdir -p \"$(readlink -m \"$(dirname %s)\")\"; \\\n", shq(bin.Destination))
			w("    install -D -m %s %s %s; \\\n", bin.Mode, src, shq(bin.Destination))
		}
		w("    rm -rf \"$tmp\"\n")
	}

	for _, ext := range a.Extensions {
		w("\n# GNOME extension %s %s (sha256-verified, docs/adr/0021)\n", ext.UUID, ext.Version)
		w("RUN set -eu; \\\n    command -v gnome-shell > /dev/null || { echo 'no gnome-shell in the image: the extension cannot be installed' >&2; exit 1; }; \\\n")
		w("    major=\"$(gnome-shell --version | awk '{print $3}' | cut -d. -f1)\"; \\\n")
		w("    tmp=\"$(mktemp -d)\"; \\\n")
		w("    %s \\\n        -o \"$tmp/extension.zip\" %s; \\\n", curlFetch, shq(ext.URL))
		w("    echo %s | sha256sum --check --strict --quiet -; \\\n", shq(ext.SHA256+"  ")+`"$tmp/extension.zip"`)
		w("    dest=%s; \\\n", shq("/usr/share/gnome-shell/extensions/"+ext.UUID))
		w("    [ ! -e \"$dest\" ] && [ ! -L \"$dest\" ] || { echo \"$dest already exists in the image\" >&2; exit 1; }; \\\n")
		w("    python3 -c %s \"$tmp/extension.zip\" \"$tmp/extract\" %s \"$major\"; \\\n", shq(extensionExtract), shq(ext.UUID))
		w("    mkdir -p \"$(dirname \"$dest\")\"; \\\n")
		w("    cp -R \"$tmp/extract\" \"$dest\"; \\\n")
		w("    chmod -R u=rwX,go=rX \"$dest\"; \\\n")
		w("    rm -f \"$dest/schemas/gschemas.compiled\"; \\\n")
		w("    if compgen -G \"$dest/schemas/*.gschema.xml\" > /dev/null; then glib-compile-schemas \"$dest/schemas\"; fi; \\\n")
		w("    rm -rf \"$tmp\"\n")
	}

	if len(a.TrustAnchors) > 0 || len(a.TrustBlocklist) > 0 {
		// A package- or image-owned file is never replaced silently; a
		// colliding name must fail the build instead.
		w("\n# Trust store certificates: the destination must not already exist\n")
		w("RUN set -eu; \\\n    for f in")
		for _, t := range a.TrustAnchors {
			w(" %s", shq(t.Path))
		}
		for _, b := range a.TrustBlocklist {
			w(" %s", shq(b.Path))
		}
		w("; do \\\n        [ ! -e \"$f\" ] || { echo \"$f already exists in the image\" >&2; exit 1; }; \\\n    done\n")
	}

	if hasRootfs {
		w("\n# Image files\nCOPY %s/ /\n", rootfsDir)
	}

	var steps []string
	if len(a.GSettings) > 0 {
		// Strict only for the DeskOS override; distribution overrides keep the RPM's non-strict behavior.
		steps = append(steps,
			`tmp="$(mktemp -d)"`,
			fmt.Sprintf(`cp %s/*.xml %s "$tmp"/`, schemasDir, gsettingsOverridePath),
			`glib-compile-schemas --strict --dry-run "$tmp"`,
			`rm -rf "$tmp"`,
			"glib-compile-schemas "+schemasDir)
	}
	if a.Dconf != nil {
		steps = append(steps, "dconf update")
	}
	// A retired Flatpak remote leaves the image as a file removal, never as
	// `flatpak uninstall` and never with --force: applications already installed
	// from it stay on an updated machine (docs/adr/0017).
	for _, rm := range p.Provisioning.FlatpakRemovedRemotes {
		steps = append(steps, "rm -f "+
			shq(path.Join(flatpakRemotesDir, rm.Name+".flatpakrepo"))+" "+
			shq(path.Join(flatpakEtcRemotesDir, rm.Name+".flatpakrepo")))
	}
	for _, c := range a.IconCaches {
		steps = append(steps, c.Command+" "+shq(c.Dir))
	}
	if a.TrustStore != nil {
		steps = append(steps, shq(a.TrustStore.Command))
	}
	if len(a.SystemdUnits) > 0 {
		units := make([]string, 0, len(a.SystemdUnits))
		for _, u := range a.SystemdUnits {
			units = append(units, shq(u.Unit))
		}
		steps = append(steps, "systemctl enable "+strings.Join(units, " "))
	}
	if len(a.SystemdMasks) > 0 {
		units := make([]string, 0, len(a.SystemdMasks))
		for _, u := range a.SystemdMasks {
			units = append(units, shq(u.Unit))
		}
		steps = append(steps, "systemctl mask "+strings.Join(units, " "))
	}
	if a.DefaultTarget != nil {
		steps = append(steps, "systemctl set-default "+shq(a.DefaultTarget.Target))
	}
	if len(steps) > 0 {
		w("\n# System configuration\nRUN %s\n", strings.Join(steps, " \\\n    && "))
	}

	if t := a.PlymouthTheme; t != nil {
		dir, wm := shq(t.Dir), shq(path.Base(t.Watermark))
		w("\n# Boot splash theme %s: package-owned frames from %s, then made the default\n", t.Name, t.FramesFrom)
		w("RUN set -eu; \\\n")
		w("    for f in %s/*.png; do \\\n", shq(t.FramesFrom))
		w("        [ -f \"$f\" ] || continue; \\\n")
		w("        name=\"$(basename \"$f\")\"; \\\n")
		w("        [ \"$name\" != %s ] || continue; \\\n", wm)
		w("        [ ! -e %s/\"$name\" ] || { echo \"$name already exists in %s\" >&2; exit 1; }; \\\n", dir, t.Dir)
		w("        install -m 0644 \"$f\" %s/\"$name\"; \\\n", dir)
		w("    done; \\\n")
		w("    [ -e %s/throbber-0001.png ] || { echo 'no two-step throbber frames in %s' >&2; exit 1; }; \\\n", dir, t.FramesFrom)
		w("    plymouth-set-default-theme %s; \\\n", shq(t.Name))
		w("    [ \"$(plymouth-set-default-theme)\" = %s ] || { echo 'Plymouth default theme is not %s' >&2; exit 1; }\n", shq(t.Name), t.Name)
	}

	if ir := a.Initramfs; ir != nil {
		w("\n# Initramfs: rebuilt for every kernel in the image, with %s\n", strings.Join(ir.DracutModules, ", "))
		w("RUN set -eu; found=0; \\\n")
		// Fedora's dracut does not include the ostree or bootc modules unless asked
		// (50ostree's check returns 255 and 51bootc says it is never installed by
		// default). Without them the image cannot prepare its ostree root and the
		// disk stops in dracut's emergency shell (docs/research-notes.md). Ask for
		// whichever the base carries; a base with neither is not a bootc base.
		w("    mods=\"\"; \\\n")
		w("    if [ -d /usr/lib/dracut/modules.d/50ostree ]; then mods=\"$mods --add ostree\"; fi; \\\n")
		w("    if [ -d /usr/lib/dracut/modules.d/51bootc ]; then mods=\"$mods --add bootc\"; fi; \\\n")
		w("    for kdir in /usr/lib/modules/*/; do \\\n")
		w("        [ -e \"${kdir}vmlinuz\" ] || continue; \\\n")
		w("        kver=\"$(basename \"$kdir\")\"; \\\n")
		w("        env DRACUT_NO_XATTR=1 dracut --force --no-hostonly $mods \"${kdir}initramfs.img\" \"$kver\"; \\\n")
		w("        lsinitrd \"${kdir}initramfs.img\" | grep -q 'ostree-prepare-root' || { echo \"ostree-prepare-root missing from initramfs of $kver\" >&2; exit 1; }; \\\n")
		for _, m := range ir.DracutModules {
			if m == "plymouth" {
				w("        lsinitrd \"${kdir}initramfs.img\" | grep -q 'plymouthd' || { echo \"plymouth missing from initramfs of $kver\" >&2; exit 1; }; \\\n")
			}
		}
		if t := a.PlymouthTheme; t != nil {
			for _, f := range []string{path.Join(t.Dir, t.Name+".plymouth"), t.Watermark, path.Join(t.Dir, "throbber-0001.png")} {
				rel := strings.TrimPrefix(f, "/")
				w("        lsinitrd \"${kdir}initramfs.img\" | grep -qF %s || { echo \"%s missing from initramfs of $kver\" >&2; exit 1; }; \\\n", shq(rel), f)
			}
			w("        lsinitrd -f etc/plymouth/plymouthd.conf \"${kdir}initramfs.img\" | grep -qx %s || { echo \"initramfs of $kver does not select Plymouth theme %s\" >&2; exit 1; }; \\\n", shq("Theme="+t.Name), t.Name)
		}
		w("        found=$((found + 1)); \\\n")
		w("    done; \\\n")
		w("    [ \"$found\" -gt 0 ] || { echo 'no kernel found under /usr/lib/modules' >&2; exit 1; }\n")
	}

	w("\n# Cleanup and bootc validation\n")
	w("RUN rm -rf %s /tmp/* /var/tmp/* \\\n", mgr.caches)
	w("    && bootc container lint\n")

	// Labels last, so workstations on one platform share earlier layers.
	w("\nLABEL")
	for i, l := range a.Labels {
		sep := " \\\n     "
		if i == 0 {
			sep = ""
		}
		w("%s %s=%s", sep, l.Name, strconv.Quote(l.Value))
	}
	w("\n")
	return []byte(b.String())
}

// RHSM state written by the subscription-manager dnf plugins during a build.
// It identifies the build host's entitlement, so it must never reach a layer.
const (
	rhsmRepoFile = "/etc/yum.repos.d/redhat.repo"
	rhsmRepoID   = "redhat"
)

var rhsmStateDirs = []string{"/var/lib/rhsm", "/var/log/rhsm"}

// The keys stay in the image (plan.json carries them too); downloads are removed in the same RUN.
const (
	rpmKeysImageDir = "/usr/share/deskos/rpm-keys"
	rpmFilesDir     = "/tmp/deskos-rpm-files"
)

// groupVerb and installVerb are the transaction commands for a platform's package
// manager. An RPM group is a group; a deb platform's group mapping is a set of
// metapackage names, so it installs like any other package.
// pkgManager describes how a platform's package manager is driven.
//
// Adding a manager means adding an entry here and to the Platform schema. A
// platform that names one DeskOS does not know fails to render, instead of
// falling back to another manager's commands: a platform gap is reported, not
// hidden (invariant 9).
type pkgManager struct {
	// update refreshes the manager's package indexes. It is empty when the
	// manager refreshes them on its own, as dnf does.
	update string
	// install is the command that installs named packages.
	install string
	// groupInstall is the command that installs a package-group mapping.
	groupInstall string
	// remove is the command that removes named packages, when the manager has
	// a tested removal path. RPM platforms use it for Platform.removePackages.
	remove string
	// excludes reports whether the manager can exclude a group member.
	excludes bool
	// clean removes what one transaction left behind.
	clean string
	// caches are the paths an image must not carry after the last transaction.
	caches string
	// rhsm reports whether the platform keeps RHSM state during a build.
	rhsm bool
}

func managerFacts(mgr string) (pkgManager, error) {
	switch mgr {
	case model.PackageManagerRpm:
		return pkgManager{
			install:      "dnf -y install",
			groupInstall: "dnf -y group install",
			// --noautoremove: the named package goes; its dependencies stay.
			// Autoremove removed flatpak as an "unused" dependency of GNOME
			// Software on AlmaLinux 10 and broke flatpak preinstall at boot.
			remove:   "dnf -y remove --noautoremove",
			excludes: true,
			clean:    "dnf clean all",
			caches:   "/var/cache/dnf /var/cache/libdnf5 /var/lib/dnf/repos /var/log/dnf* /var/log/hawkey.log",
			rhsm:     true,
		}, nil
	case model.PackageManagerDeb:
		return pkgManager{
			// apt installs nothing without indexes, and the transaction removes
			// them, so every transaction refreshes them first.
			update:       "apt-get update",
			install:      "apt-get -y install",
			groupInstall: "apt-get -y install",
			clean:        "apt-get clean && rm -rf /var/lib/apt/lists/*",
			caches:       "/var/cache/apt /var/lib/apt/lists/*",
		}, nil
	}
	return pkgManager{}, fmt.Errorf("package manager %q is not supported", mgr)
}

// groupArgs builds a group transaction's arguments. RPM groups carry group ids and
// per-group exclusions; a deb platform's members are plain package names and deb
// has no exclusion flag, so a deb platform must not declare exclusions.
func groupArgs(mgr pkgManager, groups []plan.PackageGroupInstall) []string {
	if !mgr.excludes {
		var args []string
		for _, g := range groups {
			args = append(args, g.Members...)
		}
		return args
	}
	var ids, excluded []string
	for _, g := range groups {
		ids = append(ids, g.Members...)
		excluded = append(excluded, g.ExcludePackages...)
	}
	sort.Strings(excluded)
	var args []string
	for _, x := range slices.Compact(excluded) {
		args = append(args, "--exclude="+x)
	}
	return append(args, ids...)
}

// removeCommand removes the named packages only when they are installed, then
// proves they are gone. A platform may carry a package on one vendor base and
// not another, so an absent name is a no-op rather than a failed build; a name
// that survives the transaction is a failed build (docs/adr/0022).
func removeCommand(mgr pkgManager, packages []plan.PackageRemove) string {
	names := make([]string, 0, len(packages))
	for _, p := range packages {
		names = append(names, shq(p.Name))
	}
	list := strings.Join(names, " ")
	return fmt.Sprintf(
		`installed=""; for pkg in %s; do rpm -q "$pkg" > /dev/null 2>&1 && installed="$installed $pkg"; done; if [ -n "$installed" ]; then %s $installed || exit 1; fi; for pkg in %s; do if rpm -q "$pkg" > /dev/null 2>&1; then echo "package $pkg is still installed after removal" >&2; exit 1; fi; done`,
		list, mgr.remove, list)
}

// pkgTransaction emits one package transaction. The platform's package manager
// picks the update, the command and the cleanup, and the RPM family's RHSM state
// stays out of the committed layer: state directories are tmpfs mounts, and the
// generated repo file is removed in the same RUN. A pre-existing repo file with
// that name fails the build instead of being removed.
//
// Only the first command is written without a leading "&&": a manager whose
// transaction needs no preamble has nothing to chain from, and "RUN \ && apt-get
// ..." is not valid shell.
func pkgTransaction(w func(string, ...any), mgr pkgManager, pre []string, cmd string, args []string, post ...string) {
	w("RUN")
	if mgr.rhsm {
		for i, d := range rhsmStateDirs {
			sep := " "
			if i > 0 {
				sep = " \\\n    "
			}
			w("%s--mount=type=tmpfs,target=%s", sep, d)
		}
	}
	first := true
	step := func(format string, a ...any) {
		if first {
			w(" \\\n    "+format, a...)
			first = false
			return
		}
		w(" \\\n    && "+format, a...)
	}
	if mgr.rhsm {
		step("if [ -e %s ]; then echo 'refusing to run dnf: %s already exists' >&2; exit 1; fi", rhsmRepoFile, rhsmRepoFile)
	}
	if mgr.update != "" {
		step("%s", mgr.update)
	}
	for _, c := range pre {
		step("%s", c)
	}
	step("%s", cmd)
	for _, a := range args {
		w(" \\\n        %s", shq(a))
	}
	step("%s", mgr.clean)
	for _, c := range post {
		step("%s", c)
	}
	if mgr.rhsm {
		step("rm -f %s", rhsmRepoFile)
	}
	w("\n")
}

// enableRepos enables declared repositories that are disabled after the build.
func enableRepos(repos []plan.RpmRepository) []string {
	var args []string
	for _, r := range repos {
		if !r.Enabled {
			args = append(args, "--enablerepo="+r.ID)
		}
	}
	return args
}

// groupBinaries groups installs that share one download.
func groupBinaries(bins []plan.VerifiedBinaryInstall) [][]plan.VerifiedBinaryInstall {
	idx := map[string]int{}
	var out [][]plan.VerifiedBinaryInstall
	for _, b := range bins {
		k := b.URL + "\x00" + b.SHA256
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			out = append(out, nil)
		}
		out[i] = append(out[i], b)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i][0].Artifact < out[j][0].Artifact })
	return out
}
