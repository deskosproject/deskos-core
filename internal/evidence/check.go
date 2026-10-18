package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	"github.com/deskosproject/deskos-core/internal/plan"
)

// Booted is what the running deployment says about itself, read from bootc: what is
// booted, what the machine follows for updates, and what is staged waiting for a
// reboot. It is present-state evidence, as opposed to the plan, which is the declared
// one.
type Booted struct {
	IsBootc      bool
	Image        string
	Digest       string
	Version      string
	Follows      string
	StagedImage  string
	StagedDigest string
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
		s.Checked = append(s.Checked, fmt.Sprintf("the dconf defaults the plan declares (%d), as this user's dconf context reads them", len(db.Defaults)))
	}
	s.NotChecked = []string{
		"anything a user or another tool changed under /etc, /var or anywhere else, " +
			"other than the files this plan owns: those are the plan's, and they are compared above",
		"packages installed, removed or replaced after the image was built",
		"files that belong to packages rather than to this plan",
		"the mode and the ownership of the files this plan installs: only their content is compared",
		"the runtime state of services, units and timers, including whether a declared timer is on",
		"whether the extensions the plan enables are really loaded: that is a session check",
		"the dconf locks the plan declares, and its GSettings vendor defaults",
		"the kernel arguments the plan declares: whether the booted machine carries them",
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
		if errors.Is(err, fs.ErrNotExist) {
			// The plan installs it and it is not there: that is drift, and it is named.
			return Finding{Scope: kind, Item: path, Detail: "missing: the plan installs this file and it is not there"}
		}
		// A file that could not be read is not a file that changed: it is a fact this
		// machine would not give up, and it is reported with the reason.
		return Finding{Scope: kind, Item: path, Unchecked: true,
			Detail: fmt.Sprintf("could not be read (%v)", err)}
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
		f.Detail = "could not be read from this user's dconf context (no session bus, or the key is not in it)"
		reasons["dconf defaults could not be read from this user's context"]++
		return f
	}
	if sameDconfValue(d.Key, d.Type, d.Value, got) {
		f.OK = true
		f.Detail = "as the plan declares"
		return f
	}
	f.Detail = fmt.Sprintf("this user's dconf context reads %s, the plan declares %s", got, d.Value)
	return f
}

// unorderedKeys are the declared defaults whose value is a set: GNOME rewrites the
// whole list and the plan does not fix an order. The rule, and the reason it is a list
// and not a general allowance, live in the session check (tests/vm/sessioncheck.py,
// UNORDERED_DEFAULTS); if one changes, both do.
var unorderedKeys = map[string]bool{
	"/org/gnome/shell/enabled-extensions":                              true,
	"/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings": true,
}

// sameDconfValue compares a value read from the machine with the plan's, the way the
// session check does: whitespace outside a quoted string does not count, a type
// annotation counts only when it is the declared type's own, and the two set-like keys
// compare as sets.
func sameDconfValue(key, typ, want, got string) bool {
	if canonicalGVariant(want, typ) == canonicalGVariant(got, typ) {
		return true
	}
	if !unorderedKeys[key] {
		return false
	}
	w, g := arrayElements(want, typ), arrayElements(got, typ)
	if w == nil || g == nil || len(w) != len(g) {
		return false
	}
	sort.Strings(w)
	sort.Strings(g)
	for i := range w {
		if w[i] != g[i] {
			return false
		}
	}
	return true
}

// canonicalGVariant drops the whitespace that carries no meaning, and a leading type
// annotation only when it is the declared type's own (`@as []` is the `[]` an `as` key
// stores, `@ai []` is not).
func canonicalGVariant(text, typ string) string {
	if strings.HasPrefix(text, "@") {
		annotation, rest, _ := strings.Cut(text, " ")
		if annotation == "@"+typ {
			text = rest
		}
	}
	var out []rune
	quote, escaped := false, false
	for i, c := range text {
		switch {
		case escaped:
			escaped = false
			out = append(out, c)
		case quote && c == '\\':
			escaped = true
			out = append(out, c)
		case c == '\'':
			quote = !quote
			out = append(out, c)
		case !quote && (c == ' ' || c == '\t' || c == '\n'):
			// Only whitespace outside a quoted string, and never at a bracket or a comma.
			if len(out) == 0 || strings.ContainsRune("[({,", out[len(out)-1]) {
				continue
			}
			if next := nextNonSpace(text[i:]); next != 0 && strings.ContainsRune("])},", next) {
				continue
			}
			out = append(out, ' ')
		default:
			out = append(out, c)
		}
	}
	return strings.TrimSpace(string(out))
}

func nextNonSpace(s string) rune {
	for _, c := range s {
		if c != ' ' && c != '\t' && c != '\n' {
			return c
		}
	}
	return 0
}

// arrayElements splits a GVariant text array into its elements, or returns nil when the
// text is not an array.
func arrayElements(text, typ string) []string {
	t := canonicalGVariant(text, typ)
	if !strings.HasPrefix(t, "[") || !strings.HasSuffix(t, "]") {
		return nil
	}
	var out []string
	var cur strings.Builder
	depth := 0
	quote, escaped := false, false
	for _, c := range t[1 : len(t)-1] {
		switch {
		case escaped:
			escaped = false
			cur.WriteRune(c)
		case quote && c == '\\':
			escaped = true
			cur.WriteRune(c)
		case c == '\'':
			quote = !quote
			cur.WriteRune(c)
		case quote:
			cur.WriteRune(c)
		case strings.ContainsRune("[({", c):
			depth++
			cur.WriteRune(c)
		case strings.ContainsRune("])}", c):
			depth--
			cur.WriteRune(c)
		case c == ',' && depth == 0:
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(c)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out
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
