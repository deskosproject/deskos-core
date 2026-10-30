// Package compiler wires the DeskOS pipeline:
//
//	resources -> validation -> reference resolution -> composition -> Plan
//
// Nothing here executes commands or accesses the network.
package compiler

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/loader"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
	"github.com/deskosproject/deskos-core/internal/providers/core"
	"github.com/deskosproject/deskos-core/internal/providers/desktop"
	"github.com/deskosproject/deskos-core/internal/providers/software"
	"github.com/deskosproject/deskos-core/internal/providers/system"
	"github.com/deskosproject/deskos-core/internal/registry"
	"github.com/deskosproject/deskos-core/internal/schema"
)

// Compiler holds the provider registry, schemas and lowerers.
type Compiler struct {
	Registry *registry.Registry
	schemas  *schema.Validator
	lowerers []plan.Lowerer
}

// New returns a compiler with the built-in providers.
func New() (*Compiler, error) {
	var providers []registry.Provider
	providers = append(providers, core.Providers()...)
	providers = append(providers, software.Providers()...)
	providers = append(providers, desktop.Providers()...)
	providers = append(providers, system.Providers()...)
	reg, err := registry.New(providers...)
	if err != nil {
		return nil, err
	}
	val, err := schema.New()
	if err != nil {
		return nil, err
	}
	return &Compiler{
		Registry: reg,
		schemas:  val,
		lowerers: []plan.Lowerer{software.Lowerer{}, desktop.Lowerer{}, system.Lowerer{}},
	}, nil
}

// Load reads resource roots and validates every resource: envelope, known
// GVK, JSON Schema, typed decoding and identity uniqueness. The returned
// catalog contains every resource that validated, even when err is non-nil.
func (c *Compiler) Load(roots []string) (*compose.Catalog, error) {
	var errs model.ErrorList
	resources, err := loader.Load(roots)
	errs.Add(err)
	var valid []*model.Resource
	for _, res := range resources {
		p, err := c.Registry.Lookup(res.GVK())
		if err != nil {
			errs.Add(fmt.Errorf("%s: %s: %v", res.Source, res.ID(), err))
			continue
		}
		if err := c.schemas.Validate(res, p.Schema()); err != nil {
			errs.Add(err)
			continue
		}
		if err := p.Decode(res); err != nil {
			errs.Add(err)
			continue
		}
		valid = append(valid, res)
	}
	cat, err := compose.NewCatalog(valid)
	errs.Add(err)
	return cat, errs.Err()
}

// Workstations lists the workstation names in a catalog.
func Workstations(cat *compose.Catalog) []string {
	var out []string
	for _, r := range cat.All(core.WorkstationGVK) {
		out = append(out, r.Metadata.Name)
	}
	return out
}

// Plan composes one workstation and lowers it into a canonical Plan.
func (c *Compiler) Plan(cat *compose.Catalog, workstation string) (*plan.Plan, error) {
	comp, err := compose.Compose(c.Registry, cat, workstation)
	if err != nil {
		return nil, err
	}
	ws, wsSpec := comp.Workstation, comp.WorkstationSpec
	pl, plSpec := comp.Platform, comp.PlatformSpec
	title := wsSpec.DisplayName
	if title == "" {
		title = ws.Metadata.Name
	}
	p := &plan.Plan{
		Format: plan.Format,
		Workstation: plan.Workstation{
			Name: ws.Metadata.Name, DisplayName: wsSpec.DisplayName, Source: ws.Source.String(),
		},
		Platform: plan.Platform{
			Name: pl.Metadata.Name, DisplayName: plSpec.DisplayName, Family: plSpec.Family,
			Release: plSpec.Release, Architectures: plSpec.Architectures,
			PackageManager:       plSpec.PackageManager,
			Redistributable:      plSpec.Distribution.Redistributable,
			RequiresSubscription: plSpec.Distribution.RequiresSubscription,
			Source:               pl.Source.String(),
		},
		Artifact: plan.Artifact{
			BaseImage:       plan.BaseImage{Image: plSpec.Bootc.Image, Digest: plSpec.Bootc.Digest, Ref: plSpec.BaseImageRef()},
			PackageManifest: plSpec.PackageManifest,
			Labels: []plan.Label{
				{Name: "org.opencontainers.image.title", Value: title},
				{Name: "org.deskos.workstation", Value: ws.Metadata.Name},
				{Name: "org.deskos.platform", Value: pl.Metadata.Name},
			},
		},
	}
	for _, pu := range comp.Profiles {
		p.Profiles = append(p.Profiles, plan.Profile{Name: pu.Name, Layer: pu.Layer.String(), Source: pu.Source.String()})
	}
	if !plSpec.Distribution.Redistributable {
		p.Warnings = append(p.Warnings, fmt.Sprintf("artifacts built on %s must not be publicly redistributed", pl.ID()))
	}
	if plSpec.Bootc.Digest == "" {
		p.Warnings = append(p.Warnings, fmt.Sprintf("base image %s is not pinned by digest; it resolves at build time", plSpec.Bootc.Image))
	}
	var errs model.ErrorList
	for _, l := range c.lowerers {
		if err := l.Lower(comp, p); err != nil {
			errs.Add(err)
		}
	}
	if err := errs.Err(); err != nil {
		return nil, err
	}
	if err := applyDownloads(pl, plSpec, p); err != nil {
		return nil, err
	}
	if err := validateRemovePackages(p); err != nil {
		return nil, err
	}
	if err := addImageInfo(p); err != nil {
		return nil, err
	}
	if err := p.Normalize(); err != nil {
		return nil, err
	}
	return p, nil
}

// addImageInfo writes the small, image-owned identity summary. It deliberately
// omits build-host paths, a build commit and a plan hash: the first two are not
// image content, and the plan hash would be self-referential. The full Plan
// remains the authoritative record.
func addImageInfo(p *plan.Plan) error {
	profiles := append([]plan.Profile(nil), p.Profiles...)
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Layer != profiles[j].Layer {
			return profiles[i].Layer < profiles[j].Layer
		}
		return profiles[i].Name < profiles[j].Name
	})
	type profileInfo struct {
		Name  string `json:"name"`
		Layer string `json:"layer"`
	}
	profileList := make([]profileInfo, 0, len(profiles))
	for _, profile := range profiles {
		profileList = append(profileList, profileInfo{Name: profile.Name, Layer: profile.Layer})
	}
	info := struct {
		Schema          string        `json:"schema"`
		ImageName       string        `json:"image-name"`
		ImageTitle      string        `json:"image-title,omitempty"`
		Platform        string        `json:"platform"`
		PlatformTitle   string        `json:"platform-title,omitempty"`
		PlatformFamily  string        `json:"platform-family"`
		PlatformRelease string        `json:"platform-release"`
		Architectures   []string      `json:"architectures"`
		BaseImage       string        `json:"base-image"`
		BaseImageDigest string        `json:"base-image-digest,omitempty"`
		PlanFormat      string        `json:"plan-format"`
		Profiles        []profileInfo `json:"profiles"`
	}{
		Schema:          "deskos.image-info/v1",
		ImageName:       p.Workstation.Name,
		ImageTitle:      p.Workstation.DisplayName,
		Platform:        p.Platform.Name,
		PlatformTitle:   p.Platform.DisplayName,
		PlatformFamily:  p.Platform.Family,
		PlatformRelease: p.Platform.Release,
		Architectures:   append([]string(nil), p.Platform.Architectures...),
		BaseImage:       p.Artifact.BaseImage.Ref,
		BaseImageDigest: p.Artifact.BaseImage.Digest,
		PlanFormat:      p.Format,
		Profiles:        profileList,
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("image-info: %w", err)
	}
	p.Artifact.GeneratedFiles = append(p.Artifact.GeneratedFiles, plan.GeneratedFile{
		Path: plan.ImageInfoPath, Mode: "0644", Content: string(data) + "\n",
		Provenance: []model.Provenance{{
			Resource: "Workstation/" + p.Workstation.Name,
			Layer:    model.LayerWorkstation.String(),
			Source:   p.Workstation.Source,
		}},
	})
	return nil
}

// validateRemovePackages rejects contradictory intent after every lowerer has
// spoken: a package the platform removes cannot also be installed by the
// composition. A group member that the platform removes is not this conflict;
// the group is an install and the platform's removal runs afterwards
// (docs/adr/0022).
func validateRemovePackages(p *plan.Plan) error {
	for _, rm := range p.Artifact.RemovePackages {
		for _, pkg := range p.Artifact.Packages {
			if pkg.Name == rm.Name {
				installedBy := "the composition"
				if len(pkg.Provenance) > 0 {
					installedBy = pkg.Provenance[0].Describe()
				}
				return fmt.Errorf("package %q is removed by Platform/%s and installed by the composition\n  installed by: %s",
					rm.Name, p.Platform.Name, installedBy)
			}
		}
	}
	return nil
}

// applyDownloads is what makes downloading a platform's fact instead of an
// assumption of the backend. A plan that downloads anything -- a binary artifact or
// a package file -- needs a base that can fetch, and a platform that leaves the
// question unanswered fails composition; a platform whose base cannot fetch gets the
// fetcher package added to the plan, with the provenance of the download that needed
// it. The build would otherwise discover the gap as `curl: not found`, minutes in
// (invariant 9, docs/adr/0019).
func applyDownloads(pl *model.Resource, plSpec *core.PlatformSpec, p *plan.Plan) error {
	var prov []model.Provenance
	what := ""
	switch {
	case len(p.Artifact.Binaries) > 0:
		what = p.Artifact.Binaries[0].Artifact
		for _, b := range p.Artifact.Binaries {
			prov = append(prov, b.Provenance...)
		}
	case len(p.Artifact.RpmFiles) > 0:
		what = p.Artifact.RpmFiles[0].URL
		for _, f := range p.Artifact.RpmFiles {
			prov = append(prov, f.Provenance...)
		}
	case len(p.Artifact.Extensions) > 0:
		what = p.Artifact.Extensions[0].URL
		for _, e := range p.Artifact.Extensions {
			prov = append(prov, e.Provenance...)
		}
	default:
		return nil
	}
	requested := ""
	if len(prov) > 0 {
		requested = "\n  requested by: " + prov[0].Describe()
		if prov[0].Source != "" {
			requested += " (" + prov[0].Source + ")"
		}
	}
	d := plSpec.Downloads
	if d == nil {
		return fmt.Errorf("%s does not say whether its base can fetch a verified artifact, and this plan downloads %s%s",
			pl.ID(), what, requested)
	}
	if d.BaseFetches {
		return nil
	}
	p.AddPackage(d.Package, prov...)
	return nil
}
