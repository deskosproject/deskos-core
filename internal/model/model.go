// Package model defines the DeskOS resource envelope, identities, semantic
// layers and provenance. It knows nothing about any specific resource kind.
package model

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// API groups and versions of the public DeskOS resource model.
const (
	GroupCore     = "core.deskos.org"
	GroupSoftware = "software.deskos.org"
	GroupDesktop  = "desktop.deskos.org"
	V1Alpha1      = "v1alpha1"
)

// GVK identifies a resource type.
type GVK struct {
	Group   string
	Version string
	Kind    string
}

// APIVersion returns the "group/version" form used in resource envelopes.
func (g GVK) APIVersion() string { return g.Group + "/" + g.Version }

func (g GVK) String() string { return g.APIVersion() + ", Kind " + g.Kind }

// ParseAPIVersion splits "group/version".
func ParseAPIVersion(apiVersion string) (group, version string, err error) {
	g, v, ok := strings.Cut(apiVersion, "/")
	if !ok || g == "" || v == "" || strings.Contains(v, "/") {
		return "", "", fmt.Errorf("invalid apiVersion %q: expected group/version", apiVersion)
	}
	return g, v, nil
}

// Source locates a resource document. Root is the base name of the resource
// root it was loaded from and Path is slash-separated and relative to that
// root, so provenance does not depend on the absolute checkout location.
type Source struct {
	Root string `json:"root"`
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`

	// RootDir and File are absolute host paths used for asset resolution.
	// They are never serialized.
	RootDir string `json:"-"`
	File    string `json:"-"`
}

func (s Source) String() string {
	p := s.Path
	if s.Root != "" {
		p = path.Join(s.Root, s.Path)
	}
	if s.Line > 0 {
		return fmt.Sprintf("%s:%d", p, s.Line)
	}
	return p
}

// Metadata is the resource metadata block.
type Metadata struct {
	Name string `json:"name"`
}

// Resource is a loaded, envelope-checked DeskOS resource document.
type Resource struct {
	APIVersion string
	Kind       string
	Metadata   Metadata
	// Document is the whole document as JSON, used for schema validation.
	Document json.RawMessage
	// Spec is the spec block as JSON, decoded by the owning provider.
	Spec   json.RawMessage
	Source Source
	// SpecLine is the line of the spec block, for more precise messages.
	SpecLine int

	// Object is the typed spec after provider decoding.
	Object any
}

// GVK returns the resource type. The apiVersion was checked at load time.
func (r *Resource) GVK() GVK {
	g, v, _ := ParseAPIVersion(r.APIVersion)
	return GVK{Group: g, Version: v, Kind: r.Kind}
}

// ID is the human identity "Kind/name".
func (r *Resource) ID() string { return r.Kind + "/" + r.Metadata.Name }

// ObjectRef references another resource by kind and name. APIVersion is
// optional when the kind is unambiguous among registered providers.
type ObjectRef struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
}

func (r ObjectRef) String() string { return r.Kind + "/" + r.Name }

var nameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidName reports whether s is a valid resource name (DNS label style).
func ValidName(s string) bool { return nameRE.MatchString(s) }

// Layer is a semantic composition layer. Higher layers may override scalar
// values from lower layers; equal layers must agree.
type Layer int

const (
	LayerFoundation Layer = iota + 1
	LayerOrganization
	LayerRole
	LayerWorkstation
)

var layerNames = map[Layer]string{
	LayerFoundation:   "foundation",
	LayerOrganization: "organization",
	LayerRole:         "role",
	LayerWorkstation:  "workstation",
}

func (l Layer) String() string {
	if s, ok := layerNames[l]; ok {
		return s
	}
	return fmt.Sprintf("layer(%d)", int(l))
}

// ParseLayer parses a layer name.
func ParseLayer(s string) (Layer, error) {
	for l, n := range layerNames {
		if n == s {
			return l, nil
		}
	}
	return 0, fmt.Errorf("unknown layer %q (expected foundation, organization, role or workstation)", s)
}

func (l Layer) MarshalJSON() ([]byte, error) { return json.Marshal(l.String()) }

func (l *Layer) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := ParseLayer(s)
	if err != nil {
		return err
	}
	*l = v
	return nil
}

// Provenance records which resource, included through which profile at which
// layer, contributed a piece of composed intent.
type Provenance struct {
	Resource string `json:"resource"`
	Via      string `json:"via,omitempty"`
	Profile  string `json:"profile,omitempty"`
	Layer    string `json:"layer,omitempty"`
	Source   string `json:"source"`
}

func (p Provenance) key() string {
	return p.Layer + "\x00" + p.Profile + "\x00" + p.Resource + "\x00" + p.Via + "\x00" + p.Source
}

// Less orders provenance deterministically.
func (p Provenance) Less(o Provenance) bool { return p.key() < o.key() }

// Describe renders provenance for human messages.
func (p Provenance) Describe() string {
	var b strings.Builder
	if p.Layer != "" {
		b.WriteString(p.Layer + " ")
	}
	b.WriteString(p.Resource)
	if p.Via != "" {
		b.WriteString(" (via " + p.Via + ")")
	}
	if p.Profile != "" {
		b.WriteString(" in Profile/" + p.Profile)
	}
	return b.String()
}
