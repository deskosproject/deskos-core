package system

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"path"
	"regexp"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
	"github.com/deskosproject/deskos-core/internal/providers/assets"
)

var TrustAnchorGVK = model.GVK{Group: "system.deskos.org", Version: model.V1Alpha1, Kind: "TrustAnchor"}

// DomainTrustAnchors holds keyed CA trust anchors, one per file name.
var DomainTrustAnchors = compose.Domain{ID: "trust-anchor", Label: "trust anchor"}

// TrustAnchorSpec is the public TrustAnchor spec.
type TrustAnchorSpec struct {
	Anchors []AnchorSpec `json:"anchors"`
}

// AnchorSpec is one CA certificate inside the resource root.
type AnchorSpec struct {
	// Name is the file stem of the anchor in the trust store.
	Name string `json:"name"`
	File string `json:"file"`
}

// TrustAnchorObject is the decoded TrustAnchor with its certificate assets read.
type TrustAnchorObject struct {
	Anchors []Anchor
}

// Anchor is one composed trust anchor, keyed by Name. File is the absolute
// host path of the asset and is never serialized, so the keyed value is
// host-independent.
type Anchor struct {
	Name    string `json:"name"`
	Asset   string `json:"asset"`
	SHA256  string `json:"sha256"`
	Subject string `json:"subject"`
	File    string `json:"-"`
}

var anchorNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type trustAnchor struct{}

func (trustAnchor) GVK() model.GVK { return TrustAnchorGVK }
func (trustAnchor) Schema() string { return "system.deskos.org/v1alpha1/trustanchor.json" }

func (trustAnchor) Decode(res *model.Resource) error {
	var s TrustAnchorSpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if len(s.Anchors) == 0 {
		errs.Add(model.Errorf(res, "anchors: at least one anchor is required"))
	}
	obj := &TrustAnchorObject{}
	seen := map[string]bool{}
	for i := range s.Anchors {
		a := &s.Anchors[i]
		if !anchorNameRE.MatchString(a.Name) {
			errs.Add(model.Errorf(res, "anchors[%d].name %q must be a simple file name", i, a.Name))
			continue
		}
		if seen[a.Name] {
			errs.Add(model.Errorf(res, "anchors[%d].name %q is listed twice", i, a.Name))
			continue
		}
		seen[a.Name] = true
		as, err := assets.Read(res, a.File)
		if err != nil {
			errs.Add(model.Errorf(res, "anchors[%d].file: %v", i, err))
			continue
		}
		subject, err := certificateSubject(as.Data)
		if err != nil {
			errs.Add(model.Errorf(res, "anchors[%d] %s: %v", i, as.Path, err))
			continue
		}
		obj.Anchors = append(obj.Anchors, Anchor{Name: a.Name, Asset: as.Path, SHA256: as.SHA256, Subject: subject, File: as.File})
	}
	res.Object = obj
	return errs.Err()
}

// certificateSubject validates that data holds at least one PEM-encoded
// X.509 certificate and returns the subject of the first one for messages.
func certificateSubject(data []byte) (string, error) {
	var subject string
	found := false
	for rest := data; ; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("not a valid X.509 certificate: %v", err)
		}
		if !found {
			subject = cert.Subject.String()
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("must contain at least one PEM CERTIFICATE block")
	}
	return subject, nil
}

func (trustAnchor) Contribute(res *model.Resource, s *compose.Scope) error {
	for _, a := range res.Object.(*TrustAnchorObject).Anchors {
		s.AddKeyed(DomainTrustAnchors, a.Name, a, fmt.Sprintf("%s (%s, sha256 %s)", a.Name, a.Subject, a.SHA256))
	}
	return nil
}

// lowerTrust places each anchor in the platform trust store's anchors
// directory as a .crt file and schedules one trust store update after the
// image files are copied.
func lowerTrust(c *compose.Composition, p *plan.Plan) error {
	anchors := c.Result.Keyed(DomainTrustAnchors)
	if len(anchors) == 0 {
		return nil
	}
	t := c.PlatformSpec.Trust
	var who []model.Provenance
	for _, k := range anchors {
		who = append(who, k.Provenance...)
	}
	if t == nil {
		return fmt.Errorf("%s declares no trust store facts; TrustAnchor settings cannot be applied\n  set by: %s",
			c.Platform.ID(), describe(who))
	}
	for _, k := range anchors {
		a := k.Value.(Anchor)
		p.Artifact.TrustAnchors = append(p.Artifact.TrustAnchors, plan.TrustAnchorInstall{
			Name: a.Name, Path: path.Join(t.AnchorsDir, a.Name+".crt"),
			SHA256: a.SHA256, Asset: a.Asset, AssetFile: a.File, Provenance: k.Provenance,
		})
	}
	p.Artifact.TrustStore = &plan.TrustStoreUpdate{Command: t.UpdateCommand, Provenance: who}
	return nil
}
