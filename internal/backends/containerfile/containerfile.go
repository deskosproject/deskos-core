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
	"sort"
	"strconv"
	"strings"

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
	rootfsDir         = "rootfs"
	imagePlanPath     = "/usr/share/deskos/plan.json"
	dconfFileName     = "50-deskos"
	// Numbered above distribution overrides (for example 10_ from centos-logos).
	gsettingsOverridePath = "/usr/share/glib-2.0/schemas/50_deskos.gschema.override"
	kargsPath             = "/usr/lib/bootc/kargs.d/50-deskos.toml"
	dracutConfPath        = "/usr/lib/dracut/dracut.conf.d/50-deskos.conf"
)

// File is one file of the build context.
type File struct {
	Path string
	Mode fs.FileMode
	Data []byte
}

// Render produces the complete build context in canonical order.
func Render(p *plan.Plan) ([]File, error) {
	planJSON, err := p.JSON()
	if err != nil {
		return nil, err
	}
	r := &renderer{p: p}
	r.add(PlanPath, 0o644, planJSON)

	for _, repo := range p.Artifact.RpmRepositories {
		if repo.ID == rhsmRepoID {
			return nil, fmt.Errorf("RPM repository id %q would be written as %s, which RPM builds reserve for subscription-manager", repo.ID, rhsmRepoFile)
		}
		r.add(path.Join(reposDir, "etc/yum.repos.d", repo.ID+".repo"), 0o644, repoFile(repo))
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
		dir := path.Join("/etc/dconf/db", db.Name+".d")
		r.addImage(path.Join(dir, dconfFileName), 0o644, dconfKeyfile(db))
		if len(db.Locks) > 0 {
			r.addImage(path.Join(dir, "locks", dconfFileName), 0o644, dconfLocks(db))
		}
	}
	for _, rm := range p.Provisioning.FlatpakRemotes {
		r.addImage(path.Join("/usr/share/flatpak/remotes.d", rm.Name+".flatpakrepo"), 0o644, flatpakRepoFile(rm))
	}
	for _, app := range p.Provisioning.FlatpakApplications {
		r.addImage(path.Join("/usr/share/flatpak/preinstall.d", app.ID+".preinstall"), 0o644, preinstallFile(app))
	}
	if u := p.Provisioning.FlatpakPreinstallUnit; u != "" {
		r.addImage(path.Join("/usr/lib/systemd/system", u), 0o644, preinstallUnit())
	}
	r.addImage(imagePlanPath, 0o644, planJSON)

	r.add(ContainerfilePath, 0o644, containerfile(p, r.hasRepos, r.hasRootfs))

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
		fmt.Fprintf(&b, "gpgkey=%s\n", strings.Join(r.GPGKeys, " "))
	}
	return []byte(b.String())
}

func dconfKeyfile(db *plan.DconfDatabase) []byte {
	groups := map[string][]string{}
	for _, d := range db.Defaults {
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

func flatpakRepoFile(r plan.FlatpakRemote) []byte {
	var b strings.Builder
	b.WriteString(generatedHeader)
	b.WriteString("[Flatpak Repo]\n")
	b.WriteString("Version=1\n")
	fmt.Fprintf(&b, "Title=%s\n", r.Title)
	fmt.Fprintf(&b, "Url=%s\n", r.URL)
	if r.Homepage != "" {
		fmt.Fprintf(&b, "Homepage=%s\n", r.Homepage)
	}
	if r.Comment != "" {
		fmt.Fprintf(&b, "Comment=%s\n", r.Comment)
	}
	if r.Description != "" {
		fmt.Fprintf(&b, "Description=%s\n", r.Description)
	}
	if r.CollectionID != "" {
		fmt.Fprintf(&b, "DeploySideloadCollectionID=%s\n", r.CollectionID)
	}
	fmt.Fprintf(&b, "GPGKey=%s\n", r.GPGKey)
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
func preinstallUnit() []byte {
	return []byte(generatedHeader + `[Unit]
Description=Install operating system Flatpaks declared in preinstall.d
Documentation=man:flatpak-preinstall(1)
Wants=network-online.target
After=network-online.target multi-user.target

[Service]
Type=exec
ExecStart=/usr/bin/flatpak preinstall --system --assumeyes --noninteractive

[Install]
WantedBy=multi-user.target
`)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// --------------------------------------------------------------- Containerfile

func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'" }

func containerfile(p *plan.Plan, hasRepos, hasRootfs bool) []byte {
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

	if len(a.RpmGroups) > 0 {
		var ids []string
		for _, g := range a.RpmGroups {
			ids = append(ids, g.RpmGroups...)
		}
		w("\n# RPM groups\n")
		rpmTransaction(w, "dnf -y group install", ids)
	}

	if hasRepos {
		w("\n# RPM repositories\nCOPY %s/ /\n", reposDir)
	}

	if len(a.RpmPackages) > 0 {
		var args []string
		for _, r := range a.RpmRepositories {
			if !r.Enabled {
				args = append(args, "--enablerepo="+r.ID)
			}
		}
		for _, pkg := range a.RpmPackages {
			args = append(args, pkg.Name)
		}
		w("\n# RPM packages\n")
		rpmTransaction(w, "dnf -y install", args)
	}

	for _, group := range groupBinaries(a.Binaries) {
		first := group[0]
		w("\n# Binary artifact %s %s (sha256-verified)\n", first.Artifact, first.Version)
		w("RUN set -eu; \\\n    tmp=\"$(mktemp -d)\"; \\\n")
		w("    curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \\\n        -o \"$tmp/download\" %s; \\\n", shq(first.URL))
		w("    echo %s | sha256sum --check --strict --quiet -; \\\n", shq(first.SHA256+"  ")+`"$tmp/download"`)
		for _, bin := range group {
			src := `"$tmp/download"`
			if bin.Archive == "tar.gz" {
				w("    tar -xzf \"$tmp/download\" -C \"$tmp\" --no-same-owner -- %s; \\\n", shq(bin.Member))
				src = `"$tmp"/` + shq(bin.Member)
			}
			w("    install -D -m %s %s %s; \\\n", bin.Mode, src, shq(bin.Destination))
		}
		w("    rm -rf \"$tmp\"\n")
	}

	if hasRootfs {
		w("\n# Image files\nCOPY %s/ /\n", rootfsDir)
	}

	var steps []string
	if len(a.GSettings) > 0 {
		steps = append(steps, "glib-compile-schemas --strict /usr/share/glib-2.0/schemas")
	}
	if a.Dconf != nil {
		steps = append(steps, "dconf update")
	}
	if len(a.SystemdUnits) > 0 {
		units := make([]string, 0, len(a.SystemdUnits))
		for _, u := range a.SystemdUnits {
			units = append(units, shq(u.Unit))
		}
		steps = append(steps, "systemctl enable "+strings.Join(units, " "))
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
		w("    for kdir in /usr/lib/modules/*/; do \\\n")
		w("        [ -e \"${kdir}vmlinuz\" ] || continue; \\\n")
		w("        kver=\"$(basename \"$kdir\")\"; \\\n")
		w("        env DRACUT_NO_XATTR=1 dracut --force \"${kdir}initramfs.img\" \"$kver\"; \\\n")
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
	w("RUN rm -rf /var/cache/dnf /var/cache/libdnf5 /var/lib/dnf/repos /var/log/dnf* /var/log/hawkey.log /tmp/* /var/tmp/* \\\n")
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

// rpmTransaction emits one dnf transaction whose RHSM state stays out of the
// committed layer: state directories are tmpfs mounts, and the generated repo
// file is removed in the same RUN. A pre-existing repo file with that name
// fails the build instead of being removed.
func rpmTransaction(w func(string, ...any), cmd string, args []string) {
	w("RUN")
	for i, d := range rhsmStateDirs {
		sep := " "
		if i > 0 {
			sep = " \\\n    "
		}
		w("%s--mount=type=tmpfs,target=%s", sep, d)
	}
	w(" \\\n    if [ -e %s ]; then echo 'refusing to run dnf: %s already exists' >&2; exit 1; fi", rhsmRepoFile, rhsmRepoFile)
	w(" \\\n    && %s", cmd)
	for _, a := range args {
		w(" \\\n        %s", shq(a))
	}
	w(" \\\n    && dnf clean all \\\n    && rm -f %s\n", rhsmRepoFile)
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
