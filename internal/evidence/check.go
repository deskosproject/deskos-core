package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/deskosproject/deskos-core/internal/plan"
)

// Booted is what the running deployment says about itself, read from bootc.
type Booted struct {
	IsBootc bool   `json:"-"`
	Image   string `json:"image"`
	Digest  string `json:"digest"`
}

// Probe reads the machine. The real one reads files under the root, asks bootc and
// reads dconf; tests bring their own, so the checks are exercised without a machine.
type Probe interface {
	// ReadFile reads an absolute path from the machine's root.
	ReadFile(p string) ([]byte, error)
	// Booted reports the running deployment, or an error when it cannot be read.
	Booted() (Booted, error)
	// DconfRead reads one dconf key, or returns an error when there is no session to
	// read it from.
	DconfRead(key string) (string, error)
}

// Finding is one compared item: what was compared, and how it came out.
type Finding struct {
	Scope  string // "file", "generated file", "dconf default"
	Item   string
	OK     bool
	Detail string
	// Unchecked is a fact that could not be measured on this machine, with the reason.
	// It is reported, never counted as a difference: a value nobody could read is not
	// a value that drifted.
	Unchecked bool
}

// Scope is what a check looks at, published so the reader knows where the coverage
// ends instead of assuming it is complete.
type Scope struct {
	Checked          []string
	NotChecked       []string
	UncheckedReasons map[string]int
}

// DeclaredScope describes what a check covers. The list of what it leaves out is as
// much part of the answer as the list of what it covers (docs/adr/0018): the plan is
// the image's intent, not a model of everything the machine has become.
func DeclaredScope(p *plan.Plan) Scope {
	a := p.Artifact
	s := Scope{UncheckedReasons: map[string]int{}}
	s.Checked = append(s.Checked,
		fmt.Sprintf("the files the plan installs (%d), and their declared digests", len(a.Files)),
		fmt.Sprintf("the files the plan generates (%d), and their content", len(a.GeneratedFiles)),
	)
	if db := a.Dconf; db != nil {
		s.Checked = append(s.Checked, fmt.Sprintf("the dconf defaults the plan declares (%d), as the running session sees them", len(db.Defaults)))
	}
	s.NotChecked = []string{
		"anything under /etc or /var a user or another tool changed: DeskOS does not own it",
		"packages installed, removed or replaced after the image was built",
		"files that belong to packages rather than to this plan",
		"the runtime state of services, units and timers",
		"whether the running image is the one this plan was rendered for: the plan names its base, not the image built from it",
	}
	return s
}

// Check compares the plan's declared scope with the machine, and returns what it
// found. It reads; it never writes.
func Check(probe Probe, p *plan.Plan) ([]Finding, Scope) {
	scope := DeclaredScope(p)
	var out []Finding

	for _, f := range p.Artifact.Files {
		out = append(out, compareFile(probe, "file", f.Path, f.SHA256))
	}
	for _, f := range p.Artifact.GeneratedFiles {
		sum := sha256.Sum256([]byte(f.Content))
		out = append(out, compareFile(probe, "generated file", f.Path, hex.EncodeToString(sum[:])))
	}
	if db := p.Artifact.Dconf; db != nil {
		for _, d := range db.Defaults {
			out = append(out, compareDconf(probe, d, scope.UncheckedReasons))
		}
	}
	return out, scope
}

func compareFile(probe Probe, kind, path, want string) Finding {
	data, err := probe.ReadFile(path)
	if err != nil {
		return Finding{Scope: kind, Item: path, Detail: fmt.Sprintf("missing (%v)", err)}
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return Finding{Scope: kind, Item: path, Detail: fmt.Sprintf("changed: %s, the plan declares %s", got[:12], want[:12])}
	}
	return Finding{Scope: kind, Item: path, OK: true, Detail: "as the plan declares"}
}

func compareDconf(probe Probe, d plan.DconfDefault, reasons map[string]int) Finding {
	got, err := probe.DconfRead(d.Key)
	f := Finding{Scope: "dconf default", Item: d.Key}
	if err != nil {
		f.Unchecked = true
		f.Detail = "could not be read on this machine (no session to read it from)"
		reasons["dconf defaults could not be read"]++
		return f
	}
	if strings.TrimSpace(got) == strings.TrimSpace(d.Value) {
		f.OK = true
		f.Detail = "as the plan declares"
		return f
	}
	f.Detail = fmt.Sprintf("the session sees %s, the plan declares %s", got, d.Value)
	return f
}

// Summary counts what the check found.
func Summary(findings []Finding) (ok, differ, unchecked int) {
	for _, f := range findings {
		switch {
		case f.Unchecked:
			unchecked++
		case f.OK:
			ok++
		default:
			differ++
		}
	}
	return ok, differ, unchecked
}

// Text writes the findings and the scope in the shape a person reads.
func Text(w io.Writer, findings []Finding, scope Scope) {
	pf := func(format string, args ...any) { fmt.Fprintf(w, format, args...) }
	ok, differ, unchecked := Summary(findings)
	pf("Compared %d item(s) against the plan: %d as declared, %d changed, %d not measurable here.\n",
		len(findings), ok, differ, unchecked)

	for _, f := range findings {
		if f.OK || f.Unchecked {
			continue
		}
		pf("  CHANGED  %s %s\n           %s\n", f.Scope, f.Item, f.Detail)
	}
	for _, f := range findings {
		if !f.Unchecked {
			continue
		}
		pf("  UNCHECKED %s %s\n           %s\n", f.Scope, f.Item, f.Detail)
	}

	pf("\nScope of this check\n")
	for _, c := range scope.Checked {
		pf("  checked:     %s\n", c)
	}
	for _, n := range scope.NotChecked {
		pf("  NOT checked: %s\n", n)
	}
	if len(scope.UncheckedReasons) > 0 {
		var keys []string
		for k := range scope.UncheckedReasons {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			pf("  unmeasured:  %s (%d)\n", k, scope.UncheckedReasons[k])
		}
	}
}
