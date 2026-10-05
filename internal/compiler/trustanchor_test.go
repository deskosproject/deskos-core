package compiler_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
)

// testCertPEM returns a self-signed CA certificate as a PEM asset. It never
// writes a private key anywhere.
func testCertPEM(t *testing.T, cn string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Unix(0, 0),
		NotAfter:              time.Unix(1<<31, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// trustAnchorYAML builds a TrustAnchor resource with one anchor.
func trustAnchorYAML(resource, anchor, file string) string {
	return fmt.Sprintf("apiVersion: system.deskos.org/v1alpha1\nkind: TrustAnchor\nmetadata:\n  name: %s\nspec:\n  anchors:\n    - name: %s\n      file: %s\n", resource, anchor, file)
}

func TestTrustAnchorPlanAndRender(t *testing.T) {
	cert := testCertPEM(t, "Example Org Root CA")
	dir := fixture{
		"platform.yaml":       platformYAML,
		"ws.yaml":             workstation("trust-ws", "org"),
		"profiles/org.yaml":   profile("org", "organization", "TrustAnchor/org-root"),
		"system/trust.yaml":   trustAnchorYAML("org-root", "org-root", "../assets/org-root.crt"),
		"assets/org-root.crt": cert,
	}.dir(t)

	p := mustPlan(t, "trust-ws", dir)
	if len(p.Artifact.TrustAnchors) != 1 {
		t.Fatalf("%d trust anchors, want 1", len(p.Artifact.TrustAnchors))
	}
	a := p.Artifact.TrustAnchors[0]
	if a.Path != "/etc/pki/ca-trust/source/anchors/org-root.crt" {
		t.Errorf("anchor path = %q", a.Path)
	}
	if a.SHA256 != sha256Hex(cert) {
		t.Errorf("anchor sha256 = %q", a.SHA256)
	}
	if !strings.HasSuffix(a.Asset, "assets/org-root.crt") {
		t.Errorf("anchor asset = %q", a.Asset)
	}
	if len(a.Provenance) == 0 {
		t.Error("anchor carries no provenance")
	}
	if p.Artifact.TrustStore == nil || p.Artifact.TrustStore.Command != "update-ca-trust" {
		t.Fatalf("trust store update = %+v", p.Artifact.TrustStore)
	}

	files, err := containerfile.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Data)
	}
	if got := byPath["rootfs/etc/pki/ca-trust/source/anchors/org-root.crt"]; got != cert {
		t.Errorf("rendered anchor = %q, want the certificate asset", got)
	}
	cf := byPath[containerfile.ContainerfilePath]
	copyAt := strings.Index(cf, "COPY rootfs/ /")
	if copyAt < 0 {
		t.Fatalf("no rootfs copy:\n%s", cf)
	}
	// The build refuses a destination that a package or the base image already
	// provides, instead of silently replacing it.
	guard := "for f in '/etc/pki/ca-trust/source/anchors/org-root.crt'; do"
	guardAt := strings.Index(cf, guard)
	if guardAt < 0 || guardAt > copyAt || !strings.Contains(cf, `[ ! -e "$f" ] || { echo "$f already exists in the image" >&2; exit 1; }`) {
		t.Errorf("the build must refuse an existing trust anchor destination before COPY rootfs:\n%s", cf)
	}
	updateAt := strings.Index(cf, "'update-ca-trust'")
	lintAt := strings.Index(cf, "bootc container lint")
	if updateAt < 0 || updateAt < copyAt || updateAt > lintAt {
		t.Errorf("update-ca-trust must run after COPY rootfs and before lint:\n%s", cf)
	}
}

func TestTrustAnchorRejectsNonCertificate(t *testing.T) {
	dir := fixture{
		"platform.yaml":     platformYAML,
		"ws.yaml":           workstation("bad-ws", "org"),
		"profiles/org.yaml": profile("org", "organization", "TrustAnchor/bad"),
		"system/t.yaml":     trustAnchorYAML("bad", "bad", "../assets/bad.crt"),
		"assets/bad.crt":    "this is not a certificate\n",
	}.dir(t)
	_, err := planOf(t, "bad-ws", dir)
	mustFail(t, err, "PEM CERTIFICATE")
}

// An absolute anchor path never reaches the asset loader: the schema rejects
// it, so a resource cannot pull a host file into the build context.
func TestTrustAnchorRejectsAbsoluteAssetPath(t *testing.T) {
	dir := fixture{
		"platform.yaml":     platformYAML,
		"ws.yaml":           workstation("abs-ws", "org"),
		"profiles/org.yaml": profile("org", "organization", "TrustAnchor/abs"),
		"system/t.yaml":     trustAnchorYAML("abs", "abs", "/etc/passwd"),
	}.dir(t)
	_, err := planOf(t, "abs-ws", dir)
	mustFail(t, err, "does not match pattern")
}

func TestTrustAnchorRejectsDuplicateAnchorName(t *testing.T) {
	cert := testCertPEM(t, "Dup")
	dir := fixture{
		"platform.yaml":     platformYAML,
		"ws.yaml":           workstation("dup-ws", "org"),
		"profiles/org.yaml": profile("org", "organization", "TrustAnchor/dup"),
		"system/t.yaml": "apiVersion: system.deskos.org/v1alpha1\nkind: TrustAnchor\nmetadata:\n  name: dup\nspec:\n  anchors:\n" +
			"    - {name: shared, file: ../assets/a.crt}\n    - {name: shared, file: ../assets/b.crt}\n",
		"assets/a.crt": cert,
		"assets/b.crt": cert,
	}.dir(t)
	_, err := planOf(t, "dup-ws", dir)
	mustFail(t, err, "listed twice")
}

// Two anchors with the same name but different certificates are a keyed
// conflict at any layer, not a silent override.
func TestTrustAnchorKeyedConflict(t *testing.T) {
	dir := fixture{
		"platform.yaml":     platformYAML,
		"ws.yaml":           workstation("conflict-ws", "org"),
		"profiles/org.yaml": profile("org", "organization", "TrustAnchor/a", "TrustAnchor/b"),
		"system/a.yaml":     trustAnchorYAML("a", "shared", "../assets/a.crt"),
		"system/b.yaml":     trustAnchorYAML("b", "shared", "../assets/b.crt"),
		"assets/a.crt":      testCertPEM(t, "A"),
		"assets/b.crt":      testCertPEM(t, "B"),
	}.dir(t)
	_, err := planOf(t, "conflict-ws", dir)
	mustFail(t, err, "composition conflict for trust anchor", "shared")
}

// An asset path that resolves outside the resource root is rejected even
// when it points at a readable file.
func TestTrustAnchorRejectsEscapingAsset(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	// A real certificate outside the resource root.
	if err := os.WriteFile(filepath.Join(base, "outside.crt"), []byte(testCertPEM(t, "Outside")), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"platform.yaml":     platformYAML,
		"ws.yaml":           workstation("escape-ws", "org"),
		"profiles/org.yaml": profile("org", "organization", "TrustAnchor/escape"),
		"system/t.yaml":     trustAnchorYAML("escape", "escape", "../../outside.crt"),
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := planOf(t, "escape-ws", root)
	mustFail(t, err, "resolves outside the resource root")
}

// A platform without trust facts cannot apply TrustAnchor settings.
func TestTrustAnchorRequiresPlatformFacts(t *testing.T) {
	noTrust := strings.Replace(platformYAML,
		"  trust: {anchorsDir: /etc/pki/ca-trust/source/anchors, updateCommand: update-ca-trust}\n", "", 1)
	cert := testCertPEM(t, "No Trust")
	dir := fixture{
		"platform.yaml":     noTrust,
		"ws.yaml":           workstation("notrust-ws", "org"),
		"profiles/org.yaml": profile("org", "organization", "TrustAnchor/root"),
		"system/t.yaml":     trustAnchorYAML("root", "root", "../assets/root.crt"),
		"assets/root.crt":   cert,
	}.dir(t)
	_, err := planOf(t, "notrust-ws", dir)
	mustFail(t, err, "declares no trust store facts")
}
