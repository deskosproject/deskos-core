// Package sbom renders a CycloneDX bill of materials from a compiled Plan.
//
// It is a *declared* SBOM: the inputs the Plan selects — base image, RPM
// packages, RPM files, verified binaries, system Flatpaks and trust anchors
// — with the provenance the compiler has. It resolves no package versions,
// reads no host files beyond the plan and scans no bytes.
//
// The authoritative, installed SBOM is the one a scanner (for example
// Syft) produces from the built image. Both carry the same bomFormat, so a
// promotion gate can compare the declared inputs against what the image
// actually contains and against a CVE scan (for example Grype).
package sbom

import (
	"bytes"
	"encoding/json"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/deskosproject/deskos-core/internal/plan"
)

const (
	// BOMFormat and SpecVersion identify the CycloneDX JSON encoding.
	BOMFormat   = "CycloneDX"
	SpecVersion = "1.5"
)

// Document is the subset of CycloneDX 1.5 DeskOS emits. There is no
// serialNumber and no timestamp, so identical plans render identical bytes.
type Document struct {
	BOMFormat   string      `json:"bomFormat"`
	SpecVersion string      `json:"specVersion"`
	Version     int         `json:"version"`
	Metadata    Metadata    `json:"metadata"`
	Components  []Component `json:"components"`
}

// Metadata describes the artifact the components belong to.
type Metadata struct {
	Component Component `json:"component"`
}

// Component is one CycloneDX component.
type Component struct {
	Type       string     `json:"type"`
	Name       string     `json:"name"`
	Version    string     `json:"version,omitempty"`
	Hashes     []Hash     `json:"hashes,omitempty"`
	Properties []Property `json:"properties,omitempty"`
}

// Hash is one component hash.
type Hash struct {
	Alg     string `json:"alg"`
	Content string `json:"content"`
}

// Property is a name/value annotation in the DeskOS namespace.
type Property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Build renders the declared SBOM of a plan, in canonical order.
func Build(p *plan.Plan) ([]byte, error) {
	a := p.Artifact
	d := Document{
		BOMFormat: BOMFormat, SpecVersion: SpecVersion, Version: 1,
		Metadata: Metadata{Component: Component{
			Type: "operating-system", Name: p.Workstation.Name, Version: p.Platform.Release,
			Properties: []Property{
				{Name: "deskos:platform", Value: p.Platform.Name},
				{Name: "deskos:baseImage", Value: a.BaseImage.Ref},
			},
		}},
	}
	add := func(c Component) { d.Components = append(d.Components, c) }

	base := Component{Type: "operating-system", Name: a.BaseImage.Image}
	if a.BaseImage.Digest != "" {
		base.Version = a.BaseImage.Digest
		base.Hashes = []Hash{{Alg: "SHA-256", Content: strings.TrimPrefix(a.BaseImage.Digest, "sha256:")}}
	}
	add(base)

	for _, x := range a.RpmPackages {
		add(Component{Type: "library", Name: x.Name,
			Properties: []Property{{Name: "deskos:source", Value: "rpm"}}})
	}
	for _, x := range a.RpmFiles {
		add(Component{Type: "library", Name: rpmFileName(x.URL),
			Hashes: []Hash{{Alg: "SHA-256", Content: x.SHA256}},
			Properties: []Property{
				{Name: "deskos:source", Value: "rpm-file"},
				{Name: "deskos:url", Value: x.URL},
			}})
	}
	for _, x := range a.Binaries {
		add(Component{Type: "application", Name: x.Artifact, Version: x.Version,
			Hashes: []Hash{{Alg: "SHA-256", Content: x.SHA256}},
			Properties: []Property{
				{Name: "deskos:source", Value: "binary"},
				{Name: "deskos:destination", Value: x.Destination},
			}})
	}
	for _, x := range p.Provisioning.FlatpakApplications {
		add(Component{Type: "application", Name: x.ID, Version: x.Branch,
			Properties: []Property{
				{Name: "deskos:source", Value: "flatpak"},
				{Name: "deskos:remote", Value: x.Remote},
			}})
	}
	for _, x := range a.TrustAnchors {
		add(Component{Type: "data", Name: x.Name,
			Hashes: []Hash{{Alg: "SHA-256", Content: x.SHA256}},
			Properties: []Property{
				{Name: "deskos:source", Value: "trust-anchor"},
				{Name: "deskos:destination", Value: x.Path},
			}})
	}

	sort.Slice(d.Components, func(i, j int) bool {
		x, y := d.Components[i], d.Components[j]
		if x.Type != y.Type {
			return x.Type < y.Type
		}
		if x.Name != y.Name {
			return x.Name < y.Name
		}
		return x.Version < y.Version
	})

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func rpmFileName(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		return path.Base(u.Path)
	}
	return raw
}
