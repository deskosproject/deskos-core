// Package registry maps resource types (GVK) to the providers that
// understand them.
//
// A provider decodes and validates one kind, may declare references to other
// resources, and may contribute semantic intent to composition. Providers
// have no host-side effects: they never execute commands or touch the
// network, and asset access is limited to reading files inside the declaring
// resource root.
package registry

import (
	"fmt"
	"sort"

	"github.com/deskosproject/deskos-core/internal/model"
)

// Provider handles one resource kind.
type Provider interface {
	GVK() model.GVK
	// Schema is the path of the kind's JSON Schema inside the schemas tree.
	Schema() string
	// Decode parses the spec into a typed object and validates its
	// semantics. It stores the result in res.Object.
	Decode(res *model.Resource) error
}

// Referencer is implemented by providers whose resources reference others.
type Referencer interface {
	References(res *model.Resource) []model.ObjectRef
}

// Registry is an immutable set of providers keyed by GVK.
type Registry struct {
	byGVK map[model.GVK]Provider
}

// New builds a registry and rejects duplicate registrations.
func New(providers ...Provider) (*Registry, error) {
	r := &Registry{byGVK: map[model.GVK]Provider{}}
	for _, p := range providers {
		if _, dup := r.byGVK[p.GVK()]; dup {
			return nil, fmt.Errorf("duplicate provider for %s", p.GVK())
		}
		r.byGVK[p.GVK()] = p
	}
	return r, nil
}

// Lookup returns the provider for gvk.
func (r *Registry) Lookup(gvk model.GVK) (Provider, error) {
	p, ok := r.byGVK[gvk]
	if !ok {
		return nil, fmt.Errorf("no provider registered for %s", gvk)
	}
	return p, nil
}

// ResolveRef resolves a reference's GVK. Without an apiVersion the kind must
// be served by exactly one provider.
func (r *Registry) ResolveRef(ref model.ObjectRef) (model.GVK, error) {
	if ref.APIVersion != "" {
		g, v, err := model.ParseAPIVersion(ref.APIVersion)
		if err != nil {
			return model.GVK{}, err
		}
		gvk := model.GVK{Group: g, Version: v, Kind: ref.Kind}
		if _, err := r.Lookup(gvk); err != nil {
			return model.GVK{}, err
		}
		return gvk, nil
	}
	var matches []model.GVK
	for gvk := range r.byGVK {
		if gvk.Kind == ref.Kind {
			matches = append(matches, gvk)
		}
	}
	switch len(matches) {
	case 0:
		return model.GVK{}, fmt.Errorf("no provider registered for Kind %s", ref.Kind)
	case 1:
		return matches[0], nil
	}
	return model.GVK{}, fmt.Errorf("Kind %s is ambiguous; set apiVersion on the reference", ref.Kind)
}

// Providers returns all providers ordered by GVK.
func (r *Registry) Providers() []Provider {
	out := make([]Provider, 0, len(r.byGVK))
	for _, p := range r.byGVK {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].GVK().String() < out[j].GVK().String()
	})
	return out
}
