package sbom

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/plan"
)

func testPlan() *plan.Plan {
	digest := "sha256:" + strings.Repeat("a", 64)
	p := &plan.Plan{
		Workstation: plan.Workstation{Name: "example-ws"},
		Platform:    plan.Platform{Name: "rhel-10", Release: "10"},
	}
	p.Artifact.BaseImage = plan.BaseImage{Image: "registry.example/rhel-bootc:10", Digest: digest, Ref: "registry.example/rhel-bootc:10@" + digest}
	p.Artifact.RpmPackages = []plan.RpmInstall{{Name: "git"}, {Name: "bootc"}}
	p.Artifact.RpmFiles = []plan.RpmFileInstall{{URL: "https://example.org/x/zoom_x86_64.rpm", SHA256: strings.Repeat("b", 64)}}
	p.Artifact.Binaries = []plan.VerifiedBinaryInstall{{Artifact: "oc", Version: "4.22.15", Destination: "/usr/local/bin/oc", SHA256: strings.Repeat("c", 64)}}
	p.Provisioning.FlatpakApplications = []plan.FlatpakPreinstall{{ID: "org.gnome.Firmware", Branch: "stable", Remote: "flathub"}}
	p.Artifact.TrustAnchors = []plan.TrustAnchorInstall{{Name: "root", Path: "/etc/pki/ca-trust/source/anchors/root.crt", SHA256: strings.Repeat("d", 64)}}
	return p
}

func TestBuildIsDeterministicAndSorted(t *testing.T) {
	a, err := Build(testPlan())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(testPlan())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("identical plans rendered different SBOMs")
	}
	if bytes.Contains(a, []byte("serialNumber")) || bytes.Contains(a, []byte("timestamp")) {
		t.Error("the SBOM must not carry a serialNumber or timestamp")
	}
	var d Document
	if err := json.Unmarshal(a, &d); err != nil {
		t.Fatal(err)
	}
	if d.BOMFormat != "CycloneDX" || d.SpecVersion != "1.5" {
		t.Errorf("format = %s %s", d.BOMFormat, d.SpecVersion)
	}
	if d.Metadata.Component.Name != "example-ws" {
		t.Errorf("artifact name = %q", d.Metadata.Component.Name)
	}
	// Every input kind is represented: base image, one application binary,
	// one Flatpak, one trust anchor, one RPM file, two RPM packages.
	want := map[string]string{
		"registry.example/rhel-bootc:10": "operating-system",
		"oc":                             "application",
		"org.gnome.Firmware":             "application",
		"root":                           "data",
		"zoom_x86_64.rpm":                "library",
		"git":                            "library",
		"bootc":                          "library",
	}
	got := map[string]string{}
	for _, c := range d.Components {
		got[c.Name] = c.Type
	}
	for name, typ := range want {
		if got[name] != typ {
			t.Errorf("component %q = %q, want %q", name, got[name], typ)
		}
	}
	for i := 1; i < len(d.Components); i++ {
		x, y := d.Components[i-1], d.Components[i]
		if x.Type > y.Type || (x.Type == y.Type && x.Name > y.Name) {
			t.Errorf("components are not sorted: %s/%s before %s/%s", x.Type, x.Name, y.Type, y.Name)
		}
	}
}
