package compiler_test

import (
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
)

// platformWithBlocklist adds the blocklist directory the trust provider needs
// to the test platform.
var platformWithBlocklist = strings.Replace(platformYAML,
	"trust: {anchorsDir: /etc/pki/ca-trust/source/anchors, updateCommand: update-ca-trust}",
	"trust: {anchorsDir: /etc/pki/ca-trust/source/anchors, blocklistDir: /etc/pki/ca-trust/source/blocklist, updateCommand: update-ca-trust}",
	1)

func trustBlocklistYAML(resource, name, file string) string {
	return "apiVersion: system.deskos.org/v1alpha1\nkind: TrustAnchor\nmetadata:\n  name: " + resource +
		"\nspec:\n  blocklist:\n    - {name: " + name + ", file: " + file + "}\n"
}

// A distrusted certificate is image intent an organization can declare at the
// workstation layer: it leaves the platform's blocklist directory, not its
// anchors directory, and one trust store update follows the copy.
func TestTrustBlocklistPlanAndRender(t *testing.T) {
	cert := testCertPEM(t, "Compromised Root CA")
	dir := fixture{
		"platform.yaml": platformWithBlocklist,
		"ws.yaml":       workstation("block-ws", "ws-layer"),
		// The declaration lives in a workstation-layer profile: distrusting a
		// CA is the workstation's own decision, not the foundation's only to make.
		"profiles/ws-layer.yaml": profile("ws-layer", "workstation", "TrustAnchor/distrust"),
		"system/t.yaml":          trustBlocklistYAML("distrust", "compromised-root", "../assets/compromised.crt"),
		"assets/compromised.crt": cert,
	}.dir(t)

	p := mustPlan(t, "block-ws", dir)
	if len(p.Artifact.TrustBlocklist) != 1 {
		t.Fatalf("%d distrusted certificates, want 1", len(p.Artifact.TrustBlocklist))
	}
	b := p.Artifact.TrustBlocklist[0]
	if want := "/etc/pki/ca-trust/source/blocklist/compromised-root.crt"; b.Path != want {
		t.Errorf("blocklist path = %q, want %q", b.Path, want)
	}
	if b.SHA256 != sha256Hex(cert) {
		t.Errorf("blocklist sha256 = %q", b.SHA256)
	}
	if len(b.Provenance) == 0 || b.Provenance[0].Layer != "workstation" {
		t.Errorf("blocklist provenance = %+v, want the workstation layer", b.Provenance)
	}
	if len(p.Artifact.TrustAnchors) != 0 {
		t.Errorf("distrust produced %d anchors, want none", len(p.Artifact.TrustAnchors))
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
	if got := byPath["rootfs/etc/pki/ca-trust/source/blocklist/compromised-root.crt"]; got != cert {
		t.Errorf("rendered blocklist entry = %q, want the certificate asset", got)
	}
	cf := byPath[containerfile.ContainerfilePath]
	copyAt := strings.Index(cf, "COPY rootfs/ /")
	updateAt := strings.Index(cf, "'update-ca-trust'")
	if copyAt < 0 || updateAt < 0 || updateAt < copyAt {
		t.Fatalf("update-ca-trust must run after the blocklist copy:\n%s", cf)
	}
	guard := "for f in '/etc/pki/ca-trust/source/blocklist/compromised-root.crt'; do"
	if at := strings.Index(cf, guard); at < 0 || at > copyAt {
		t.Errorf("the build must refuse an existing blocklist destination before COPY rootfs:\n%s", cf)
	}
}

// A platform that declares anchors but no blocklist directory cannot express a
// distrust, and composition says which platform gap it is.
func TestTrustBlocklistRequiresPlatformDirectory(t *testing.T) {
	dir := fixture{
		"platform.yaml":     platformYAML,
		"ws.yaml":           workstation("gap-ws", "org"),
		"profiles/org.yaml": profile("org", "organization", "TrustAnchor/distrust"),
		"system/t.yaml":     trustBlocklistYAML("distrust", "compromised-root", "../assets/c.crt"),
		"assets/c.crt":      testCertPEM(t, "Compromised"),
	}.dir(t)
	_, err := planOf(t, "gap-ws", dir)
	mustFail(t, err, "declares no trust store blocklist directory", "test-platform")
}

// One name means one file in the trust store: anchoring and distrusting the
// same certificate is a contradiction even when two resources declare it.
func TestTrustBlocklistCannotAlsoBeAnAnchor(t *testing.T) {
	cert := testCertPEM(t, "Contradictory")
	dir := fixture{
		"platform.yaml":     platformWithBlocklist,
		"ws.yaml":           workstation("both-ws", "org"),
		"profiles/org.yaml": profile("org", "organization", "TrustAnchor/trust-me", "TrustAnchor/distrust-me"),
		"system/a.yaml":     trustAnchorYAML("trust-me", "same-name", "../assets/one.crt"),
		"system/b.yaml":     trustBlocklistYAML("distrust-me", "same-name", "../assets/two.crt"),
		"assets/one.crt":    cert,
		"assets/two.crt":    cert,
	}.dir(t)
	_, err := planOf(t, "both-ws", dir)
	mustFail(t, err, "both a trust anchor and a distrusted certificate", "same-name")
}

func TestTrustAnchorNeedsAnAnchorOrABlocklist(t *testing.T) {
	dir := fixture{
		"platform.yaml":     platformWithBlocklist,
		"ws.yaml":           workstation("empty-ws", "org"),
		"profiles/org.yaml": profile("org", "organization", "TrustAnchor/empty"),
		"system/t.yaml": "apiVersion: system.deskos.org/v1alpha1\nkind: TrustAnchor\n" +
			"metadata:\n  name: empty\nspec: {}\n",
	}.dir(t)
	_, err := planOf(t, "empty-ws", dir)
	mustFail(t, err, "missing property 'anchors'", "missing property 'blocklist'")
}
