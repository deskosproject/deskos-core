package compose

import (
	"fmt"
	"sort"

	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/providers/core"
	"github.com/deskosproject/deskos-core/internal/registry"
)

// Contributor is implemented by providers of content kinds, the kinds a
// Profile may include.
type Contributor interface {
	Contribute(res *model.Resource, s *Scope) error
}

// Catalog indexes decoded resources by identity.
type Catalog struct {
	byGVK map[model.GVK]map[string]*model.Resource
}

// NewCatalog indexes resources and rejects duplicate GVK/name identities.
func NewCatalog(resources []*model.Resource) (*Catalog, error) {
	c := &Catalog{byGVK: map[model.GVK]map[string]*model.Resource{}}
	var errs model.ErrorList
	for _, r := range resources {
		gvk := r.GVK()
		m := c.byGVK[gvk]
		if m == nil {
			m = map[string]*model.Resource{}
			c.byGVK[gvk] = m
		}
		if prev, dup := m[r.Metadata.Name]; dup {
			errs.Add(fmt.Errorf("duplicate resource %s (%s)\n  first defined at %s\n  defined again at %s", r.ID(), gvk.APIVersion(), prev.Source, r.Source))
			continue
		}
		m[r.Metadata.Name] = r
	}
	return c, errs.Err()
}

// Get returns a resource or nil.
func (c *Catalog) Get(gvk model.GVK, name string) *model.Resource { return c.byGVK[gvk][name] }

// All returns resources of one type sorted by name.
func (c *Catalog) All(gvk model.GVK) []*model.Resource {
	var out []*model.Resource
	for _, r := range c.byGVK[gvk] {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Metadata.Name < out[j].Metadata.Name })
	return out
}

// ProfileUse is a profile selected by the workstation.
type ProfileUse struct {
	Name   string
	Layer  model.Layer
	Source model.Source
}

// Composition is the resolved semantic intent of one workstation.
type Composition struct {
	Workstation     *model.Resource
	WorkstationSpec *core.WorkstationSpec
	Platform        *model.Resource
	PlatformSpec    *core.PlatformSpec
	Profiles        []ProfileUse
	Result          *Result
}

// Compose resolves a workstation's platform and profiles, walks every
// referenced resource and resolves the collected contributions.
func Compose(reg *registry.Registry, cat *Catalog, workstation string) (*Composition, error) {
	ws := cat.Get(core.WorkstationGVK, workstation)
	if ws == nil {
		return nil, fmt.Errorf("Workstation/%s is not defined", workstation)
	}
	wsSpec := ws.Object.(*core.WorkstationSpec)
	comp := &Composition{Workstation: ws, WorkstationSpec: wsSpec}
	var errs model.ErrorList

	comp.Platform = cat.Get(core.PlatformGVK, wsSpec.PlatformRef)
	if comp.Platform == nil {
		errs.Add(fmt.Errorf("%s: %s references Platform/%s, which is not defined", ws.Source, ws.ID(), wsSpec.PlatformRef))
	} else {
		comp.PlatformSpec = comp.Platform.Object.(*core.PlatformSpec)
	}

	names := append([]string(nil), wsSpec.Profiles...)
	sort.Strings(names)
	col := NewCollector()
	for _, name := range names {
		p := cat.Get(core.ProfileGVK, name)
		if p == nil {
			errs.Add(fmt.Errorf("%s: %s references Profile/%s, which is not defined", ws.Source, ws.ID(), name))
			continue
		}
		spec := p.Object.(*core.ProfileSpec)
		comp.Profiles = append(comp.Profiles, ProfileUse{Name: name, Layer: spec.Layer, Source: p.Source})
		w := walker{reg: reg, cat: cat, col: col, profile: p, layer: spec.Layer, visited: map[string]bool{}}
		for _, ref := range spec.Resources {
			errs.Add(w.include(p, ref, ""))
		}
	}
	sort.Slice(comp.Profiles, func(i, j int) bool {
		a, b := comp.Profiles[i], comp.Profiles[j]
		if a.Layer != b.Layer {
			return a.Layer < b.Layer
		}
		return a.Name < b.Name
	})
	if err := errs.Err(); err != nil {
		return nil, err
	}
	res, err := col.Resolve()
	if err != nil {
		return nil, err
	}
	comp.Result = res
	return comp, nil
}

type walker struct {
	reg     *registry.Registry
	cat     *Catalog
	col     *Collector
	profile *model.Resource
	layer   model.Layer
	visited map[string]bool
}

func (w *walker) include(from *model.Resource, ref model.ObjectRef, via string) error {
	gvk, err := w.reg.ResolveRef(ref)
	if err != nil {
		return fmt.Errorf("%s: %s references %s: %v", from.Source, from.ID(), ref, err)
	}
	res := w.cat.Get(gvk, ref.Name)
	if res == nil {
		return fmt.Errorf("%s: %s references %s, which is not defined", from.Source, from.ID(), ref)
	}
	key := gvk.String() + "/" + ref.Name
	if w.visited[key] {
		return nil
	}
	w.visited[key] = true

	prov, _ := w.reg.Lookup(gvk)
	c, ok := prov.(Contributor)
	if !ok {
		return fmt.Errorf("%s: %s cannot include %s: only content resources can be included by a Profile", from.Source, from.ID(), res.ID())
	}
	scope := &Scope{c: w.col, layer: w.layer, provenance: model.Provenance{
		Resource: res.ID(),
		Via:      via,
		Profile:  w.profile.Metadata.Name,
		Layer:    w.layer.String(),
		Source:   res.Source.String(),
	}}
	var errs model.ErrorList
	errs.Add(c.Contribute(res, scope))
	if r, ok := prov.(registry.Referencer); ok {
		for _, next := range r.References(res) {
			errs.Add(w.include(res, next, res.ID()))
		}
	}
	return errs.Err()
}
