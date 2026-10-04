package compiler_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
)

const (
	rpmURL  = "https://vendor.example.org/client/7.2.1/client_x86_64.rpm"
	rpmURL2 = "https://other.example.org/agent/2.0/agent.x86_64.rpm"
	rpmSHA  = "79b6fc1ffd9fd2e2d136e898aed9c8ed6ab672a83841de4220ca4c14005d76fd"
	rpmSHA2 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	keyA    = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQGNBGXAAAABDADkeyA\n=AAAA\n-----END PGP PUBLIC KEY BLOCK-----\n"
	keyB    = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQGNBGXBBBBBDADkeyB\n=BBBB\n-----END PGP PUBLIC KEY BLOCK-----\n"
)

// rpmFileSet builds a PackageSet whose spec has only rpmFiles; entries are
// "url sha256 gpgKeyFile".
func rpmFileSet(name string, entries ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: software.deskos.org/v1alpha1\nkind: PackageSet\nmetadata:\n  name: %s\nspec:\n  rpmFiles:\n", name)
	for _, e := range entries {
		f := strings.Fields(e)
		fmt.Fprintf(&b, "    - url: %s\n      sha256: %s\n", f[0], f[1])
		if len(f) > 2 {
			fmt.Fprintf(&b, "      gpgKeyFile: %s\n", f[2])
		}
	}
	return b.String()
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestRpmFileValidation(t *testing.T) {
	load := func(doc string) error {
		_, err := newCompiler(t).Load([]string{fixture{"s.yaml": doc, "keys/vendor.asc": keyA}.dir(t)})
		return err
	}
	if err := load(rpmFileSet("vendor", rpmURL+" "+rpmSHA+" keys/vendor.asc")); err != nil {
		t.Fatalf("valid rpmFiles rejected: %v", err)
	}
	for name, tc := range map[string]struct {
		entry string
		want  []string
	}{
		"http url":           {"http://vendor.example.org/7.2.1/c.rpm " + rpmSHA + " keys/vendor.asc", []string{"schema validation failed"}},
		"floating url":       {"https://vendor.example.org/latest/c.rpm " + rpmSHA + " keys/vendor.asc", []string{"rpmFiles[0].url", "floating location"}},
		"main branch url":    {"https://vendor.example.org/Main/c.rpm " + rpmSHA + " keys/vendor.asc", []string{"floating location"}},
		"url with variable":  {"https://vendor.example.org/$releasever/c.rpm " + rpmSHA + " keys/vendor.asc", []string{"unsupported variables"}},
		"uppercase sha256":   {rpmURL + " " + strings.ToUpper(rpmSHA) + " keys/vendor.asc", []string{"schema validation failed"}},
		"short sha256":       {rpmURL + " 79b6fc1f keys/vendor.asc", []string{"schema validation failed"}},
		"missing gpgKeyFile": {rpmURL + " " + rpmSHA, []string{"schema validation failed"}},
		"missing key asset":  {rpmURL + " " + rpmSHA + " keys/missing.asc", []string{"rpmFiles[0].gpgKeyFile"}},
	} {
		t.Run(name, func(t *testing.T) {
			mustFail(t, load(rpmFileSet("vendor", tc.entry)), tc.want...)
		})
	}
	t.Run("duplicate url in one set", func(t *testing.T) {
		mustFail(t, load(rpmFileSet("vendor", rpmURL+" "+rpmSHA+" keys/vendor.asc", rpmURL+" "+rpmSHA+" keys/vendor.asc")), "listed twice")
	})
	t.Run("unknown field", func(t *testing.T) {
		doc := rpmFileSet("vendor", rpmURL+" "+rpmSHA+" keys/vendor.asc") + "      postInstall: rm -rf /\n"
		mustFail(t, load(doc), "schema validation failed")
	})
	t.Run("binary key", func(t *testing.T) {
		dir := fixture{"s.yaml": rpmFileSet("vendor", rpmURL+" "+rpmSHA+" keys/vendor.gpg"), "keys/vendor.gpg": "\x99\x01binary-key"}.dir(t)
		_, err := newCompiler(t).Load([]string{dir})
		mustFail(t, err, "must be an ASCII-armored OpenPGP public key")
	})
	t.Run("gpgKeyFile outside the resource root", func(t *testing.T) {
		dir := fixture{"s.yaml": rpmFileSet("vendor", rpmURL+" "+rpmSHA+" ../outside.asc")}.dir(t)
		if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "outside.asc"), []byte(keyA), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := newCompiler(t).Load([]string{dir})
		mustFail(t, err, "rpmFiles[0].gpgKeyFile", "outside the resource root")
	})
}

// The typed decoder enforces the same rules as the schema on its own.
func TestRpmFileTypedValidationWithoutSchema(t *testing.T) {
	c := newCompiler(t)
	for name, tc := range map[string]struct {
		spec string
		want string
	}{
		"http url":           {`{"rpmFiles":[{"url":"http://vendor.example.org/1/c.rpm","sha256":"` + rpmSHA + `","gpgKeyFile":"k.asc"}]}`, "must be an https URL"},
		"invalid sha256":     {`{"rpmFiles":[{"url":"` + rpmURL + `","sha256":"ABC","gpgKeyFile":"k.asc"}]}`, "must be 64 lowercase hex characters"},
		"missing sha256":     {`{"rpmFiles":[{"url":"` + rpmURL + `","gpgKeyFile":"k.asc"}]}`, "sha256 is required"},
		"missing gpgKeyFile": {`{"rpmFiles":[{"url":"` + rpmURL + `","sha256":"` + rpmSHA + `"}]}`, "gpgKeyFile: file is required"},
		"unknown field":      {`{"rpmFiles":[{"url":"` + rpmURL + `","sha256":"` + rpmSHA + `","gpgKeyFile":"k.asc","script":"x"}]}`, "unknown field"},
	} {
		t.Run(name, func(t *testing.T) {
			res := rawResource("software.deskos.org/v1alpha1", "PackageSet", "x", tc.spec)
			p, err := c.Registry.Lookup(res.GVK())
			if err != nil {
				t.Fatal(err)
			}
			mustFail(t, p.Decode(res), tc.want)
		})
	}
}

func TestRpmFilesKeyed(t *testing.T) {
	ws := base.with(fixture{
		"ws.yaml":       workstation("ws", "org", "role"),
		"org.yaml":      profile("org", "organization", "PackageSet/org-vendor"),
		"role.yaml":     profile("role", "role", "PackageSet/role-vendor"),
		"org/keys.asc":  keyA,
		"role/keys.asc": keyA,
		"role/b.asc":    keyB,
	})
	t.Run("identical definitions deduplicate", func(t *testing.T) {
		dir := ws.with(fixture{
			"org/s.yaml":  rpmFileSet("org-vendor", rpmURL+" "+rpmSHA+" keys.asc"),
			"role/s.yaml": rpmFileSet("role-vendor", rpmURL+" "+rpmSHA+" keys.asc"),
		}).dir(t)
		p := mustPlan(t, "ws", dir)
		if n := len(p.Artifact.RpmFiles); n != 1 {
			t.Fatalf("got %d RPM files, want 1", n)
		}
		f := p.Artifact.RpmFiles[0]
		if f.URL != rpmURL || f.SHA256 != rpmSHA || f.GPGKey != keyA || f.GPGKeySHA256 != sha(keyA) {
			t.Errorf("RPM file = %+v", f)
		}
		if n := len(f.Provenance); n != 2 {
			t.Errorf("deduplicated RPM file should keep both sources, got %d", n)
		}
	})
	t.Run("same url with another sha256 conflicts", func(t *testing.T) {
		dir := ws.with(fixture{
			"org/s.yaml":  rpmFileSet("org-vendor", rpmURL+" "+rpmSHA+" keys.asc"),
			"role/s.yaml": rpmFileSet("role-vendor", rpmURL+" "+rpmSHA2+" keys.asc"),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, `composition conflict for RPM file "`+rpmURL+`"`,
			"PackageSet/org-vendor", "PackageSet/role-vendor", "org/s.yaml", "role/s.yaml", rpmSHA, rpmSHA2)
	})
	t.Run("same url with another key conflicts", func(t *testing.T) {
		dir := ws.with(fixture{
			"org/s.yaml":  rpmFileSet("org-vendor", rpmURL+" "+rpmSHA+" keys.asc"),
			"role/s.yaml": rpmFileSet("role-vendor", rpmURL+" "+rpmSHA+" b.asc"),
		}).dir(t)
		_, err := planOf(t, "ws", dir)
		mustFail(t, err, `composition conflict for RPM file "`+rpmURL+`"`, sha(keyA), sha(keyB))
	})
}

func TestRpmFilesRender(t *testing.T) {
	files := base.with(fixture{
		"a.yaml":       profile("a", "organization", "PackageSet/vendor-b", "PackageSet/tools"),
		"b.yaml":       profile("b", "role", "PackageSet/vendor-a"),
		"tools.yaml":   packageSet("tools", "git"),
		"va.yaml":      rpmFileSet("vendor-a", rpmURL+" "+rpmSHA+" keys/a.asc"),
		"vb.yaml":      rpmFileSet("vendor-b", rpmURL2+" "+rpmSHA2+" keys/b.asc"),
		"keys/a.asc":   keyA,
		"keys/b.asc":   keyB,
		"ws.yaml":      workstation("ws", "a", "b"),
		"swapped.yaml": workstation("swapped", "b", "a"),
	})
	dir := files.dir(t)
	p := mustPlan(t, "ws", dir)
	if n := len(p.Artifact.RpmFiles); n != 2 || p.Artifact.RpmFiles[0].URL != rpmURL2 || p.Artifact.RpmFiles[1].URL != rpmURL {
		t.Fatalf("RPM files are not sorted by URL: %+v", p.Artifact.RpmFiles)
	}

	render := func(ws string) map[string][]byte {
		out, err := containerfile.Render(mustPlan(t, ws, dir))
		if err != nil {
			t.Fatal(err)
		}
		m := map[string][]byte{}
		for _, f := range out {
			m[f.Path] = f.Data
		}
		return m
	}
	a, b := render("ws"), render("ws")
	for k, v := range a {
		if !bytes.Equal(v, b[k]) {
			t.Errorf("%s differs between renders", k)
		}
	}
	cf := string(a[containerfile.ContainerfilePath])
	block := rpmFilesRun(t, cf)
	if swapped := rpmFilesRun(t, string(render("swapped")[containerfile.ContainerfilePath])); swapped != block {
		t.Errorf("RPM file RUN depends on profile order:\n%s\n---\n%s", block, swapped)
	}

	for _, k := range []string{keyA, keyB} {
		if got := string(a["rpm-keys/"+sha(k)+".asc"]); got != k {
			t.Errorf("build context key rpm-keys/%s.asc = %q", sha(k), got)
		}
	}
	checkRHSMRuns(t, "ws", p, cf)

	pkgs := strings.Index(cf, "# RPM packages\n")
	if pkgs < 0 || strings.Index(cf, "# RPM files") < pkgs {
		t.Error("RPM files are not installed after the package transaction")
	}
	if rootfs := strings.Index(cf, "# Image files"); rootfs >= 0 && rootfs < strings.Index(cf, "# RPM files") {
		t.Error("RPM files are installed after the image files")
	}
	for _, f := range []struct{ url, sum, key string }{{rpmURL, rpmSHA, keyA}, {rpmURL2, rpmSHA2, keyB}} {
		rpm := "/tmp/deskos-rpm-files/" + f.sum + ".rpm"
		rpmkeys := "rpmkeys --dbpath '/tmp/deskos-rpm-files/keyring-" + f.sum + "' --define '_keyring rpmdb'"
		for _, need := range []string{
			"-o '" + rpm + "' '" + f.url + "'",
			"echo '" + f.sum + "  " + rpm + "' | sha256sum --check --strict --quiet -",
			rpmkeys + " --import '/usr/share/deskos/rpm-keys/" + sha(f.key) + ".asc'",
			rpmkeys + " --define '_pkgverify_level signature' --checksig '" + rpm + "'",
			"\n        '" + rpm + "'",
		} {
			if !strings.Contains(block, need) {
				t.Errorf("RPM file RUN lacks %q:\n%s", need, block)
			}
		}
		if strings.Index(block, "--checksig '"+rpm+"'") > strings.Index(block, "dnf -y install") {
			t.Errorf("%s is installed before its signature is checked", f.url)
		}
	}
	for _, need := range []string{
		"# RPM files (sha256- and signature-verified)\nCOPY rpm-keys/ /usr/share/deskos/rpm-keys/\nRUN ",
		"curl --fail --silent --show-error --location --proto '=https' --tlsv1.2",
		"&& rm -rf /tmp/deskos-rpm-files \\\n",
	} {
		if !strings.Contains(block, need) {
			t.Errorf("RPM file RUN lacks %q:\n%s", need, block)
		}
	}
}

// rpmFilesRun returns the RPM file block of a Containerfile.
func rpmFilesRun(t *testing.T, cf string) string {
	t.Helper()
	i := strings.Index(cf, "# RPM files")
	if i < 0 {
		t.Fatal("Containerfile has no RPM file RUN")
	}
	block, _, _ := strings.Cut(cf[i:], "\n\n")
	return block
}
