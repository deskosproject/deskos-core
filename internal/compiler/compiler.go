// Package compiler wires the DeskOS pipeline:
//
//	resources -> validation -> reference resolution -> composition -> Plan
//
// Nothing here executes commands or accesses the network.
package compiler

import (
	"fmt"

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
			Redistributable:      plSpec.Distribution.Redistributable,
			RequiresSubscription: plSpec.Distribution.RequiresSubscription,
			Source:               pl.Source.String(),
		},
		Artifact: plan.Artifact{
			BaseImage: plan.BaseImage{Image: plSpec.Bootc.Image, Digest: plSpec.Bootc.Digest, Ref: plSpec.BaseImageRef()},
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
	if err := p.Normalize(); err != nil {
		return nil, err
	}
	return p, nil
}
