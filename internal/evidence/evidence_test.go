package evidence_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/evidence"
	"github.com/deskosproject/deskos-core/internal/plan"
)

// The checks are exercised against a machine a test describes, so nothing here needs
// bootc, dconf or a session.
type fake struct {
	files    map[string]string
	booted   evidence.Booted
	err      error
	dconf    map[string]string
	dconfErr error
}

func (f fake) ReadFile(p string) ([]byte, error) {
	if v, ok := f.files[p]; ok {
		return []byte(v), nil
	}
	return nil, os.ErrNotExist
}

func (f fake) Booted() (evidence.Booted, error) { return f.booted, f.err }

func (f fake) DconfRead(key string) (string, error) {
	if f.dconfErr != nil {
		return "", f.dconfErr
	}
	if v, ok := f.dconf[key]; ok {
		return v, nil
	}
	return "", nil
}

func planText(t *testing.T, extra map[string]any) string {
	t.Helper()
	p := map[string]any{
		"workstation": map[string]any{"name": "ws", "source": "w.yaml"},
		"platform":    map[string]any{"name": "test-platform", "family": "test", "release": "1"},
		"profiles":    []any{map[string]any{"name": "deskos-gnome", "layer": "foundation"}},
		"artifact": map[string]any{
			"baseImage":      map[string]any{"image": "example.org/bootc:1", "digest": "sha256:aaaa"},
			"files":          []any{map[string]any{"path": "/usr/share/deskos/logo.svg", "sha256": hash("logo")}},
			"generatedFiles": []any{map[string]any{"path": "/etc/dconf/db/distro.d/50-deskos", "content": "keyfile"}},
			"dconf": map[string]any{"name": "distro", "defaults": []any{
				map[string]any{"key": "/org/gnome/desktop/session/idle-delay", "type": "u", "value": "uint32 600"},
			}},
			"packages":         []any{},
			"scheduledUpdates": []any{map[string]any{"kind": "image", "unit": "bootc-fetch-apply-updates.timer"}},
		},
	}
	for k, v := range extra {
		p[k] = v
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// parsePlan is what the command's loadPlan does with the file, without the file: the
// logic under test takes a plan, and the command's tests cover reading it.
func parsePlan(t *testing.T, text string) *plan.Plan {
	t.Helper()
	var p plan.Plan
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func TestCheckFindsNoDifferenceWhenTheMachineMatchesThePlan(t *testing.T) {
	text := planText(t, nil)
	p := parsePlan(t, text)
	first := p.Artifact.Files[0]
	probe := fake{
		files: map[string]string{
			first.Path:                        "logo",
			p.Artifact.GeneratedFiles[0].Path: p.Artifact.GeneratedFiles[0].Content,
		},
		dconf: map[string]string{"/org/gnome/desktop/session/idle-delay": "uint32 600"},
	}
	findings, _ := evidence.Check(probe, p)
	ok, differ, unchecked := evidence.Summary(findings)
	if differ != 0 {
		t.Errorf("%d difference(s) on a matching machine: %+v", differ, findings)
	}
	if unchecked != 0 {
		t.Errorf("%d unchecked on a machine that answers: %+v", unchecked, findings)
	}
	if ok != len(findings) {
		t.Errorf("%d of %d as declared", ok, len(findings))
	}
}

func TestCheckReportsADifferenceAndNamesIt(t *testing.T) {
	text := planText(t, nil)
	p := parsePlan(t, text)
	probe := fake{
		files: map[string]string{
			p.Artifact.Files[0].Path:          "something else",
			p.Artifact.GeneratedFiles[0].Path: p.Artifact.GeneratedFiles[0].Content,
		},
		dconf: map[string]string{"/org/gnome/desktop/session/idle-delay": "uint32 900"},
	}
	findings, _ := evidence.Check(probe, p)
	_, differ, _ := evidence.Summary(findings)
	if differ != 2 {
		t.Fatalf("differ = %d, want the file and the default: %+v", differ, findings)
	}
	var out bytes.Buffer
	evidence.Text(&out, findings, evidence.DeclaredScope(p))
	if !strings.Contains(out.String(), "CHANGED  file "+p.Artifact.Files[0].Path) {
		t.Errorf("the changed file is not named:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "this user's dconf context reads uint32 900") {
		t.Errorf("the change does not say what it saw:\n%s", out.String())
	}
}

func TestAValueThatCannotBeReadIsReportedAndNotCountedAsADifference(t *testing.T) {
	// A value nobody could read is not a value that drifted: it is coverage with a
	// reason, and the scope says how much of it there was.
	text := planText(t, nil)
	p := parsePlan(t, text)
	probe := fake{
		files: map[string]string{
			p.Artifact.Files[0].Path:          "logo",
			p.Artifact.GeneratedFiles[0].Path: p.Artifact.GeneratedFiles[0].Content,
		},
		dconfErr: errors.New("no session bus"),
	}
	findings, scope := evidence.Check(probe, p)
	ok, differ, unchecked := evidence.Summary(findings)
	if differ != 0 || unchecked != 1 || ok != len(findings)-1 {
		t.Errorf("ok=%d differ=%d unchecked=%d", ok, differ, unchecked)
	}
	var out bytes.Buffer
	evidence.Text(&out, findings, scope)
	for _, want := range []string{"UNCHECKED dconf default", "unmeasured:  dconf defaults could not be read from this user's context (1)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, out.String())
		}
	}
}

func TestCheckSaysItDoesNotReadTheBinariesItInstalls(t *testing.T) {
	// The binaries are files the plan installs, and this check does not read them: it
	// has to say so, or its list of what it covers reads as if it did (docs/adr/0018).
	p := parsePlan(t, planText(t, nil))
	p.Artifact.Binaries = append(p.Artifact.Binaries, plan.VerifiedBinaryInstall{
		Artifact: "deskos", Version: "0.12.0", Archive: "none",
		SHA256: hash("deskos"), Destination: "/usr/bin/deskos", Mode: "0755"})
	joined := strings.Join(evidence.DeclaredScope(p).NotChecked, "\n")
	if !strings.Contains(joined, "the bytes of the files the plan installs through an artifact (1)") {
		t.Errorf("scope does not say the binaries are left out:\n%s", joined)
	}
}

func TestCheckPublishesWhatItLeavesOut(t *testing.T) {
	// The scope is part of the answer: a check that only listed what it looks at would
	// read as if it measured everything (docs/adr/0018).
	text := planText(t, nil)
	p := parsePlan(t, text)
	scope := evidence.DeclaredScope(p)
	joined := strings.Join(append(append([]string{}, scope.Checked...), scope.NotChecked...), "\n")
	for _, want := range []string{
		"the files the plan installs (1)",
		"anything a user or another tool changed under /etc, /var or anywhere else",
		"packages installed, removed or replaced after the image was built",
		"whether the running image is the one this plan was rendered for",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("scope lacks %q:\n%s", want, joined)
		}
	}
}

func TestReportSeparatesWhatIsInstalledFromWhatIsOnlyDeclared(t *testing.T) {
	text := planText(t, nil)
	p := parsePlan(t, text)
	var out bytes.Buffer
	evidence.Report(&out, p, &evidence.Booted{IsBootc: true, Image: "quay.io/example/ws:latest", Digest: "sha256:bbbb"})
	got := out.String()
	for _, want := range []string{
		"Workstation: ws",
		"Base image:  example.org/bootc:1 @ sha256:aaaa  [declared]",
		"booted:  quay.io/example/ws:latest",
		"digest:  sha256:bbbb",
		"plan: installed on this machine",
		"NOT installed on this machine (ADR 0010",
		"the plan schedules image updates for the image itself",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
}

func TestGuardRefusesAMachineThatIsNotABootcDeployment(t *testing.T) {
	conditions := evidence.Guard(fake{})
	if len(conditions) == 0 || !conditions[0].Blocks {
		t.Fatalf("conditions = %+v, want a blocking one", conditions)
	}
	var out bytes.Buffer
	err := evidence.Update(&out, conditions, func(string, ...string) error {
		t.Error("an update must not run when the guard refuses")
		return nil
	}, false, false)
	if err == nil || !strings.Contains(err.Error(), "was not installed that way") {
		t.Fatalf("err = %v", err)
	}
}

func TestGuardDoesNotRefuseForEtcCustomization(t *testing.T) {
	// A machine a user customized is still a machine they may update: /etc is theirs
	// and DeskOS does not reconcile it (AGENTS.md invariant 8).
	text := planText(t, nil)
	p := parsePlan(t, text)
	conditions := append(
		evidence.Guard(fake{booted: evidence.Booted{IsBootc: true, Image: "img", Digest: "sha256:cc"}}),
		evidence.ReportedConditions(p)...)
	for _, c := range conditions {
		if c.Blocks {
			t.Errorf("a reported condition blocks: %+v", c)
		}
	}
	if !strings.Contains(strings.Join(reportLines(conditions), "\n"), "customization of /etc is not touched") {
		t.Errorf("conditions = %+v", conditions)
	}
}

func reportLines(conditions []evidence.Condition) []string {
	var out []string
	for _, c := range conditions {
		out = append(out, c.Name+": "+c.Detail)
	}
	return out
}

func TestUpdateStagesByDefaultAndAppliesOnRequest(t *testing.T) {
	var ran []string
	run := func(name string, args ...string) error {
		ran = append(ran, name+" "+strings.Join(args, " "))
		return nil
	}
	var out bytes.Buffer
	if err := evidence.Update(&out, nil, run, false, false); err != nil {
		t.Fatal(err)
	}
	if err := evidence.Update(&out, nil, run, true, false); err != nil {
		t.Fatal(err)
	}
	want := []string{"bootc upgrade", "bootc upgrade --apply"}
	if len(ran) != 2 || ran[0] != want[0] || ran[1] != want[1] {
		t.Errorf("ran = %v, want %v", ran, want)
	}
	if !strings.Contains(out.String(), "staged. The machine applies it on the next reboot") {
		t.Errorf("staging is not explained: %q", out.String())
	}
}

func TestUpdateDryRunChangesNothing(t *testing.T) {
	var out bytes.Buffer
	if err := evidence.Update(&out, nil, func(string, ...string) error {
		t.Error("a dry run must not run anything")
		return nil
	}, true, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "would run: bootc upgrade --apply") {
		t.Errorf("dry run said %q", out.String())
	}
}
