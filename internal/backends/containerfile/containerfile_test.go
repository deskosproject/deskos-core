package containerfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
)

func TestPlymouthThemeNeedsInitramfs(t *testing.T) {
	p := &plan.Plan{Platform: plan.Platform{Name: "test", PackageManager: model.PackageManagerRpm}}
	p.Artifact.PlymouthTheme = &plan.PlymouthTheme{Name: "deskos", Dir: "/usr/share/plymouth/themes/deskos"}
	if _, err := Render(p); err == nil || !strings.Contains(err.Error(), "needs an initramfs regeneration") {
		t.Errorf("Render = %v", err)
	}
}

func TestPlymouthThemeFile(t *testing.T) {
	got := string(plymouthTheme(&plan.PlymouthTheme{Name: "deskos", Dir: "/usr/share/plymouth/themes/deskos"}))
	for _, want := range []string{"\nName=deskos\n", "\nModuleName=two-step\n", "\nImageDir=/usr/share/plymouth/themes/deskos\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("theme file lacks %q", want)
		}
	}
	if strings.Contains(got, "UseFirmwareBackground=true") {
		t.Error("a mode uses the firmware (BGRT) logo")
	}
	for _, mode := range []string{"boot-up", "shutdown", "reboot", "updates", "system-upgrade", "firmware-upgrade", "system-reset"} {
		_, section, ok := strings.Cut(got, "\n["+mode+"]\n")
		if section, _, _ = strings.Cut(section, "\n["); !ok || !strings.Contains(section, "UseFirmwareBackground=false\n") {
			t.Errorf("[%s] does not disable the firmware background", mode)
		}
	}
}

// A repository key's image file name comes from its sha256, never from the
// asset name: a crafted name cannot inject repository directives.
func TestRepoKeyNameIgnoresTheAssetName(t *testing.T) {
	content := "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nAAAA\n-----END PGP PUBLIC KEY BLOCK-----\n"
	sum := sha256.Sum256([]byte(content))
	k := plan.RepoKey{Name: "key\ngpgcheck=0", Content: content, SHA256: hex.EncodeToString(sum[:])}
	if got := repoKeyName(k); got != k.SHA256+".asc" {
		t.Errorf("repoKeyName = %q, want %q", got, k.SHA256+".asc")
	}
	p := &plan.Plan{Platform: plan.Platform{Name: "test", PackageManager: model.PackageManagerRpm}}
	p.Artifact.RpmRepositories = []plan.RpmRepository{{
		ID: "r", DisplayName: "r", BaseURL: "https://example.org/r", Enabled: true, GPGCheck: true,
		GPGKeys: []plan.RepoKey{k},
	}}
	files, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	var repo string
	for _, f := range files {
		if strings.ContainsAny(f.Path, "\n") {
			t.Errorf("a rendered path carries the asset name: %q", f.Path)
		}
		if f.Path == "repos/etc/yum.repos.d/r.repo" {
			repo = string(f.Data)
		}
	}
	if strings.Contains(repo, "gpgcheck=0\n") {
		t.Errorf("the repository file carries an injected directive:\n%s", repo)
	}
}

// Two repositories may share a signing key; its file is written once.
func TestRepoKeysSharedBetweenRepositories(t *testing.T) {
	content := "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nAAAA\n-----END PGP PUBLIC KEY BLOCK-----\n"
	sum := sha256.Sum256([]byte(content))
	k := plan.RepoKey{Name: "shared.asc", Content: content, SHA256: hex.EncodeToString(sum[:])}
	p := &plan.Plan{Platform: plan.Platform{Name: "test", PackageManager: model.PackageManagerRpm}}
	p.Artifact.RpmRepositories = []plan.RpmRepository{
		{ID: "a", DisplayName: "a", BaseURL: "https://example.org/a", Enabled: true, GPGCheck: true, GPGKeys: []plan.RepoKey{k}},
		{ID: "b", DisplayName: "b", BaseURL: "https://example.org/b", Enabled: true, GPGCheck: true, GPGKeys: []plan.RepoKey{k}},
	}
	files, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range files {
		if strings.HasPrefix(f.Path, "repos/etc/pki/rpm-gpg/") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("wrote %d key files, want 1 (shared keys must not duplicate)", n)
	}
}

// An APT source renders as a deb822 stanza apt reads itself, next to the keyring
// it names, under the paths apt looks at — and both are on disk before the first
// package transaction, because a group or a package that comes from a declared
// source needs the source to exist by then.
func TestAptSourceRendersDeb822ItsKeyringAndTheCopyOrder(t *testing.T) {
	content := "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nAAAA\n-----END PGP PUBLIC KEY BLOCK-----\n"
	sum := sha256.Sum256([]byte(content))
	sha := hex.EncodeToString(sum[:])
	p := &plan.Plan{Platform: plan.Platform{Name: "ubuntu-26.04", DisplayName: "Ubuntu 26.04 LTS", PackageManager: model.PackageManagerDeb}}
	p.Artifact.BaseImage = plan.BaseImage{Image: "example.org/deskos/ubuntu-bootc:26.04", Ref: "example.org/deskos/ubuntu-bootc:26.04"}
	p.Artifact.PackageGroups = []plan.PackageGroupInstall{{Name: "workstation", Members: []string{"ubuntu-desktop-minimal"}}}
	p.Artifact.AptRepositories = []plan.AptRepository{{
		ID: "vscode", DisplayName: "Visual Studio Code - Stable",
		BaseURL: "https://packages.microsoft.com/repos/code",
		Suite:   "stable", Components: []string{"main"}, Architectures: []string{"amd64"},
		GPGKeys: []plan.RepoKey{{Name: "microsoft.asc", Content: content, SHA256: sha}},
	}}
	files, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	var source, keyring, cf string
	for _, f := range files {
		switch f.Path {
		case "repos/etc/apt/sources.list.d/deskos-vscode.sources":
			source = string(f.Data)
		case "repos/etc/apt/keyrings/deskos-" + sha + ".asc":
			keyring = string(f.Data)
		case "Containerfile":
			cf = string(f.Data)
		}
	}
	if source == "" {
		t.Fatal("no deb822 source was rendered")
	}
	if keyring != content {
		t.Errorf("keyring = %q, want the key's bytes where apt and _apt can read them", keyring)
	}
	want := "Types: deb\n" +
		"URIs: https://packages.microsoft.com/repos/code\n" +
		"Suites: stable\n" +
		"Components: main\n" +
		"Architectures: amd64\n" +
		"Signed-By: /etc/apt/keyrings/deskos-" + sha + ".asc\n" +
		"Description: Visual Studio Code - Stable\n"
	if !strings.Contains(source, want) {
		t.Errorf("source =\n%s\nwant it to contain\n%s", source, want)
	}
	copyAt, groupAt := strings.Index(cf, "COPY repos/ /"), strings.Index(cf, "# Package groups")
	if copyAt < 0 || groupAt < 0 || copyAt > groupAt {
		t.Errorf("the repository files must be copied before the first package transaction:\n%s", cf)
	}
}

// Render never writes a source apt would trust against every key it has, and
// never writes one whose key does not match the bytes the plan recorded.
func TestAptSourceWithoutAKeyOrWithTheWrongOneIsNotRendered(t *testing.T) {
	content := "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nAAAA\n-----END PGP PUBLIC KEY BLOCK-----\n"
	for name, tc := range map[string]struct {
		keys []plan.RepoKey
		want string
	}{
		"no key": {want: "no signing key"},
		"mismatched sha256": {
			keys: []plan.RepoKey{{Name: "k.asc", Content: content, SHA256: strings.Repeat("0", 64)}},
			want: "does not match its sha256",
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := &plan.Plan{Platform: plan.Platform{Name: "ubuntu-26.04", PackageManager: model.PackageManagerDeb}}
			p.Artifact.BaseImage = plan.BaseImage{Image: "example.org/deskos/ubuntu-bootc:26.04", Ref: "example.org/deskos/ubuntu-bootc:26.04"}
			p.Artifact.AptRepositories = []plan.AptRepository{{
				ID: "x", DisplayName: "x", BaseURL: "https://example.org/deb/",
				Suite: "stable", Components: []string{"main"}, GPGKeys: tc.keys,
			}}
			if _, err := Render(p); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Render = %v, want %q", err, tc.want)
			}
		})
	}
}

// A tar.gz member is installed only after the extraction proves it is a plain
// file: tar would extract a symlink or hardlink member as such, and install
// would then copy whatever it points at.
func TestBinaryArchiveMemberGuard(t *testing.T) {
	p := &plan.Plan{Platform: plan.Platform{Name: "test", PackageManager: model.PackageManagerRpm}}
	p.Artifact.Binaries = []plan.VerifiedBinaryInstall{{
		Artifact: "tool", Version: "1", URL: "https://example.org/tool.tar.gz",
		SHA256: strings.Repeat("a", 64), Archive: "tar.gz", Member: "tool",
		Destination: "/usr/local/bin/tool", Mode: "0755",
	}}
	files, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	var cf string
	for _, f := range files {
		if f.Path == ContainerfilePath {
			cf = string(f.Data)
		}
	}
	for _, want := range []string{`tar -xzf "$tmp/download"`, `[ ! -L `, `stat -c %h`, "is not a regular file"} {
		if !strings.Contains(cf, want) {
			t.Errorf("the binary extraction lacks %q:\n%s", want, cf)
		}
	}
}

// The guard rejects a symlink member and accepts a regular one; without it,
// install would copy the target of the symlink into the image.
func TestBinaryArchiveGuardRejectsSymlinkMember(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte("TOPSECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(dir, "stage")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stage, "good"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(stage, "evil")); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "a.tar.gz")
	if out, err := exec.Command("tar", "-czf", archive, "-C", stage, "good", "evil").CombinedOutput(); err != nil {
		t.Skipf("tar is unavailable: %v (%s)", err, out)
	}
	for _, m := range []struct {
		name   string
		reject bool
	}{{"evil", true}, {"good", false}} {
		dst := filepath.Join(dir, "out-"+m.name)
		// The guard exactly as the backend renders it.
		script := fmt.Sprintf(`set -eu; tmp="$(mktemp -d)"; tar -xzf %q -C "$tmp" --no-same-owner -- %q; f="$tmp"/%q; [ -f "$f" ] && [ ! -L "$f" ] && [ "$(stat -c %%h "$f")" = 1 ] || { echo 'not a regular file' >&2; exit 1; }; install -D -m 0755 "$f" %q`,
			archive, m.name, m.name, dst)
		if err := exec.Command("sh", "-c", script).Run(); (err != nil) != m.reject {
			t.Errorf("member %q: err=%v, want rejected=%v", m.name, err, m.reject)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "out-evil")); err == nil && bytes.Contains(data, []byte("TOPSECRET")) {
		t.Error("the symlink member leaked the file it points at")
	}
}

// A deb platform renders APT transactions: the same package model, a different
// command, and none of the RPM-only surface (dnf, group flags, RHSM state).
func TestDebPlatformRendersAPT(t *testing.T) {
	p := &plan.Plan{
		Platform: plan.Platform{
			Name: "ubuntu-26.04", DisplayName: "Ubuntu 26.04 LTS",
			PackageManager: model.PackageManagerDeb,
		},
	}
	p.Artifact.BaseImage = plan.BaseImage{
		Image: "example.org/deskos/ubuntu-bootc:26.04",
		Ref:   "example.org/deskos/ubuntu-bootc:26.04",
	}
	p.Artifact.PackageGroups = []plan.PackageGroupInstall{
		{Name: "workstation", Members: []string{"ubuntu-desktop-minimal"}},
	}
	p.Artifact.Packages = []plan.PackageInstall{{Name: "curl"}}

	out, err := Render(p)
	if err != nil {
		t.Fatal(err)
	}
	var cf string
	for _, f := range out {
		if strings.HasSuffix(f.Path, "Containerfile") {
			cf = string(f.Data)
		}
	}
	if cf == "" {
		t.Fatal("no Containerfile rendered")
	}
	for _, want := range []string{
		"apt-get -y install",
		"'ubuntu-desktop-minimal'",
		"'curl'",
		"apt-get clean && rm -rf /var/lib/apt/lists/*",
	} {
		if !strings.Contains(cf, want) {
			t.Errorf("rendered Containerfile lacks %q", want)
		}
	}
	// apt installs nothing without indexes, and each transaction removes them.
	update, install := strings.Index(cf, "apt-get update"), strings.Index(cf, "apt-get -y install")
	if update < 0 || update > install {
		t.Errorf("apt-get update must come before the install in the same RUN\n%s", cf)
	}
	for _, unwanted := range []string{"dnf", "--exclude=", "rhsm", "redhat.repo"} {
		if strings.Contains(cf, unwanted) {
			t.Errorf("a deb platform renders RPM-only surface: %q", unwanted)
		}
	}
}

// A rendered RUN must be valid shell. Rendering does not prove it: a transaction
// whose first command carries a leading "&&" renders fine and only fails when the
// image is built, which is how the deb path shipped an invalid first RUN.
func TestRenderedRunIsValidShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh to check the rendered shell")
	}
	for _, mgr := range []string{model.PackageManagerRpm, model.PackageManagerDeb} {
		p := &plan.Plan{Platform: plan.Platform{Name: "test", PackageManager: mgr}}
		p.Artifact.BaseImage = plan.BaseImage{Image: "example.org/base:1", Ref: "example.org/base:1"}
		p.Artifact.PackageGroups = []plan.PackageGroupInstall{{Name: "workstation", Members: []string{"group-member"}}}
		p.Artifact.Packages = []plan.PackageInstall{{Name: "a-package"}}
		out, err := Render(p)
		if err != nil {
			t.Fatalf("%s: %v", mgr, err)
		}
		for _, f := range out {
			if !strings.HasSuffix(f.Path, "Containerfile") {
				continue
			}
			for _, body := range runBodies(string(f.Data)) {
				cmd := exec.Command("sh", "-n")
				cmd.Stdin = strings.NewReader(body)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Errorf("%s: a RUN is not valid shell: %v\n%s\n--- rendered ---\n%s", mgr, err, out, body)
				}
			}
		}
	}
}

// runBodies returns the shell of every RUN instruction: the Dockerfile flags are
// dropped and backslash continuations are joined into one script.
func runBodies(cf string) []string {
	var bodies []string
	lines := strings.Split(cf, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "RUN") {
			continue
		}
		body := strings.TrimPrefix(lines[i], "RUN")
		for strings.HasSuffix(strings.TrimRight(body, " \t"), "\\") && i+1 < len(lines) {
			i++
			body += "\n" + lines[i]
		}
		bodies = append(bodies, dropRunFlags(body))
	}
	return bodies
}

// dropRunFlags removes the leading --mount=... flags of a RUN: they are Dockerfile
// syntax, not shell.
func dropRunFlags(body string) string {
	body = strings.TrimLeft(body, " \t\n\\")
	for strings.HasPrefix(body, "--mount=") {
		end := strings.IndexAny(body, " \t\n")
		if end < 0 {
			return ""
		}
		body = strings.TrimLeft(body[end:], " \t\n\\")
	}
	return body
}

// A platform that names a package manager DeskOS does not know fails to render,
// instead of falling back to another manager's commands. Adding a manager (Arch's
// pacman, for example) means adding it to the table and to the Platform schema.
func TestUnknownPackageManagerFails(t *testing.T) {
	p := &plan.Plan{
		Platform: plan.Platform{Name: "arch", DisplayName: "Arch", PackageManager: "pacman"},
	}
	p.Artifact.BaseImage = plan.BaseImage{
		Image: "example.org/arch/bootc:latest",
		Ref:   "example.org/arch/bootc:latest",
	}
	_, err := Render(p)
	if err == nil || !strings.Contains(err.Error(), `package manager "pacman" is not supported`) {
		t.Errorf("Render = %v, want an unsupported package manager error", err)
	}
}
