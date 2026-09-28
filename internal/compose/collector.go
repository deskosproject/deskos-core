// Package compose resolves a Workstation into composed semantic intent.
//
// Every contribution belongs to exactly one composition class:
//
//   - set: members are unioned; duplicates collapse (packages, units).
//   - keyed: one definition per key; identical definitions deduplicate and
//     different definitions conflict regardless of layer (repositories,
//     binary destinations, Flatpak remotes and applications).
//   - scalar: layered values; the highest layer wins, and two different
//     values at the same layer conflict (desktop settings).
//
// Profile and file order never influence the result.
package compose

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deskosproject/deskos-core/internal/model"
)

// Domain names a namespace of composed intent. Label is used in messages.
type Domain struct {
	ID    string
	Label string
}

// Member is a composed set member.
type Member struct {
	Name       string
	Provenance []model.Provenance
}

// Keyed is a composed keyed definition.
type Keyed struct {
	Key        string
	Value      any
	Provenance []model.Provenance
}

// Scalar is a composed layered value. Overridden lists lower-layer
// contributions that the winning layer replaced.
type Scalar struct {
	Key        string
	Value      any
	Display    string
	Layer      model.Layer
	Provenance []model.Provenance
	Overridden []Contribution
}

// Contribution is one authored value with its origin.
type Contribution struct {
	Value      any
	Display    string
	Layer      model.Layer
	Provenance model.Provenance
	canonical  string
}

// Collector accumulates contributions during reference resolution.
type Collector struct {
	sets    map[Domain]map[string][]model.Provenance
	keyed   map[Domain]map[string][]Contribution
	scalars map[Domain]map[string][]Contribution
}

// NewCollector returns an empty collector.
func NewCollector() *Collector {
	return &Collector{
		sets:    map[Domain]map[string][]model.Provenance{},
		keyed:   map[Domain]map[string][]Contribution{},
		scalars: map[Domain]map[string][]Contribution{},
	}
}

// Scope is handed to a provider while it contributes one resource included
// through one profile.
type Scope struct {
	c          *Collector
	layer      model.Layer
	provenance model.Provenance
}

// Layer is the semantic layer of the including profile.
func (s *Scope) Layer() model.Layer { return s.layer }

// Provenance describes the resource being contributed.
func (s *Scope) Provenance() model.Provenance { return s.provenance }

// AddMember adds a set member.
func (s *Scope) AddMember(d Domain, name string) {
	m := s.c.sets[d]
	if m == nil {
		m = map[string][]model.Provenance{}
		s.c.sets[d] = m
	}
	m[name] = append(m[name], s.provenance)
}

// AddKeyed adds a keyed definition. display renders the value in messages.
func (s *Scope) AddKeyed(d Domain, key string, value any, display string) {
	m := s.c.keyed[d]
	if m == nil {
		m = map[string][]Contribution{}
		s.c.keyed[d] = m
	}
	m[key] = append(m[key], Contribution{Value: value, Display: display, Layer: s.layer, Provenance: s.provenance, canonical: model.Canonical(value)})
}

// SetScalar adds a layered value. display renders the value as authored.
func (s *Scope) SetScalar(d Domain, key string, value any, display string) {
	m := s.c.scalars[d]
	if m == nil {
		m = map[string][]Contribution{}
		s.c.scalars[d] = m
	}
	m[key] = append(m[key], Contribution{Value: value, Display: display, Layer: s.layer, Provenance: s.provenance, canonical: model.Canonical(value)})
}

// Result is the resolved composition.
type Result struct {
	sets    map[string][]Member
	keyed   map[string][]Keyed
	scalars map[string][]Scalar
}

// Set returns the sorted members of a set domain.
func (r *Result) Set(d Domain) []Member { return r.sets[d.ID] }

// Keyed returns the definitions of a keyed domain sorted by key.
func (r *Result) Keyed(d Domain) []Keyed { return r.keyed[d.ID] }

// Scalars returns the effective values of a scalar domain sorted by key.
func (r *Result) Scalars(d Domain) []Scalar { return r.scalars[d.ID] }

// Scalar returns one effective value.
func (r *Result) Scalar(d Domain, key string) (Scalar, bool) {
	for _, s := range r.scalars[d.ID] {
		if s.Key == key {
			return s, true
		}
	}
	return Scalar{}, false
}

// Resolve applies the composition rules and reports every conflict.
func (c *Collector) Resolve() (*Result, error) {
	r := &Result{sets: map[string][]Member{}, keyed: map[string][]Keyed{}, scalars: map[string][]Scalar{}}
	var errs model.ErrorList

	for d, m := range c.sets {
		for _, name := range sortedKeys(m) {
			r.sets[d.ID] = append(r.sets[d.ID], Member{Name: name, Provenance: uniqProvenance(m[name])})
		}
	}

	for d, m := range c.keyed {
		for _, key := range sortedKeys(m) {
			contribs := m[key]
			if distinct(contribs) > 1 {
				errs.Add(keyedConflict(d, key, contribs))
				continue
			}
			provs := make([]model.Provenance, 0, len(contribs))
			for _, ct := range contribs {
				provs = append(provs, ct.Provenance)
			}
			r.keyed[d.ID] = append(r.keyed[d.ID], Keyed{Key: key, Value: contribs[0].Value, Provenance: uniqProvenance(provs)})
		}
	}

	for d, m := range c.scalars {
		for _, key := range sortedKeys(m) {
			s, err := resolveScalar(d, key, m[key])
			if err != nil {
				errs.Add(err)
				continue
			}
			r.scalars[d.ID] = append(r.scalars[d.ID], s)
		}
	}
	return r, errs.Err()
}

func resolveScalar(d Domain, key string, contribs []Contribution) (Scalar, error) {
	byLayer := map[model.Layer][]Contribution{}
	for _, ct := range contribs {
		byLayer[ct.Layer] = append(byLayer[ct.Layer], ct)
	}
	var layers []model.Layer
	for l := range byLayer {
		layers = append(layers, l)
	}
	sort.Slice(layers, func(i, j int) bool { return layers[i] < layers[j] })

	var errs model.ErrorList
	for _, l := range layers {
		if distinct(byLayer[l]) > 1 {
			errs.Add(scalarConflict(d, key, l, byLayer[l]))
		}
	}
	if err := errs.Err(); err != nil {
		return Scalar{}, err
	}
	top := layers[len(layers)-1]
	win := byLayer[top]
	s := Scalar{Key: key, Value: win[0].Value, Display: win[0].Display, Layer: top}
	for _, ct := range win {
		s.Provenance = append(s.Provenance, ct.Provenance)
	}
	s.Provenance = uniqProvenance(s.Provenance)
	for _, l := range layers[:len(layers)-1] {
		s.Overridden = append(s.Overridden, uniqContribs(byLayer[l])...)
	}
	return s, nil
}

func distinct(cs []Contribution) int {
	seen := map[string]bool{}
	for _, c := range cs {
		seen[c.canonical] = true
	}
	return len(seen)
}

func uniqContribs(cs []Contribution) []Contribution {
	sort.Slice(cs, func(i, j int) bool { return cs[i].Provenance.Less(cs[j].Provenance) })
	var out []Contribution
	for i, c := range cs {
		if i > 0 && c.Provenance == cs[i-1].Provenance && c.canonical == cs[i-1].canonical {
			continue
		}
		out = append(out, c)
	}
	return out
}

func uniqProvenance(ps []model.Provenance) []model.Provenance {
	sort.Slice(ps, func(i, j int) bool { return ps[i].Less(ps[j]) })
	var out []model.Provenance
	for i, p := range ps {
		if i > 0 && p == ps[i-1] {
			continue
		}
		out = append(out, p)
	}
	return out
}

func scalarConflict(d Domain, key string, layer model.Layer, cs []Contribution) error {
	var b strings.Builder
	fmt.Fprintf(&b, "composition conflict for %s %q\n", d.Label, key)
	writeContribs(&b, uniqContribs(cs))
	fmt.Fprintf(&b, "\n  these contributions are at the same semantic layer (%s); a higher layer may\n  override a lower one, but contributions at one layer must agree", layer)
	return fmt.Errorf("%s", b.String())
}

func keyedConflict(d Domain, key string, cs []Contribution) error {
	var b strings.Builder
	fmt.Fprintf(&b, "composition conflict for %s %q\n", d.Label, key)
	writeContribs(&b, uniqContribs(cs))
	b.WriteString("\n  identical definitions are deduplicated; different definitions for the\n  same key are an error at any layer")
	return fmt.Errorf("%s", b.String())
}

func writeContribs(b *strings.Builder, cs []Contribution) {
	for _, c := range cs {
		fmt.Fprintf(b, "\n  %s\n    value: %s\n    source: %s\n", c.Provenance.Describe(), c.Display, c.Provenance.Source)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
