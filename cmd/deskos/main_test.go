package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The command is exercised with --root, so nothing here needs a bootc machine.
func withPlan(t *testing.T, extra map[string]any) string {
	t.Helper()
	p := map[string]any{
		"workstation": map[string]any{"name": "ws", "source": "w.yaml"},
		"platform":    map[string]any{"name": "test-platform", "family": "test", "release": "1"},
		"artifact": map[string]any{
			"baseImage": map[string]any{"image": "example.org/bootc:1", "digest": "sha256:aaaa"},
			"files":     []any{},
			"packages":  []any{},
		},
	}
	for k, v := range extra {
		p[k] = v
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "usr/share/deskos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestReportReadsThePlanAndSaysWhatIsOnlyDeclared(t *testing.T) {
	root := withPlan(t, nil)
	code, out, errOut := runCLI(t, "report", "--root", root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{
		"Workstation: ws",
		"Base image:  example.org/bootc:1 @ sha256:aaaa  [declared]",
		"plan: installed on this machine",
		"NOT installed on",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
}

func TestReportRefusesAMachineWithoutAPlan(t *testing.T) {
	code, _, errOut := runCLI(t, "report", "--root", t.TempDir())
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "does not carry a plan") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestCheckComparesTheDeclaredScopeAndPublishesItsLimits(t *testing.T) {
	root := withPlan(t, nil)
	code, out, errOut := runCLI(t, "check", "--root", root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{
		"Compared 0 item(s) against the plan",
		"Scope of this check",
		"NOT checked: anything a user or another tool changed under /etc, /var or anywhere else",
		"NOT checked: whether the extensions the plan enables are really loaded",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("check lacks %q:\n%s", want, out)
		}
	}
}

func TestCheckFailsWhenSomethingChanged(t *testing.T) {
	root := withPlan(t, map[string]any{
		"artifact": map[string]any{
			"baseImage": map[string]any{"image": "example.org/bootc:1"},
			"files": []any{map[string]any{
				"path": "/usr/share/deskos/missing.svg", "sha256": strings.Repeat("a", 64),
			}},
			"packages": []any{},
		},
	})
	code, out, _ := runCLI(t, "check", "--root", root)
	if code != 1 {
		t.Fatalf("exit %d, want 1 for a difference", code)
	}
	if !strings.Contains(out, "CHANGED  file /usr/share/deskos/missing.svg") {
		t.Errorf("check does not name the change:\n%s", out)
	}
}

func TestUpdateRefusesWithoutAPlanAndChangesNothing(t *testing.T) {
	// update takes no --root: it acts on the machine it runs on. On a machine without
	// the plan it refuses instead of updating something into something else.
	code, out, errOut := runCLI(t, "update")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stdout %q)", code, out)
	}
	if !strings.Contains(errOut, "does not carry a plan") {
		t.Errorf("stderr = %q", errOut)
	}
	if strings.Contains(out, "running: bootc") {
		t.Errorf("an update ran without a plan: %q", out)
	}
}

func TestUpdateTakesNoRoot(t *testing.T) {
	// A flag that mixed one tree's plan with the host's bootc would be a footgun.
	code, _, _ := runCLI(t, "update", "--root", t.TempDir())
	if code != 2 {
		t.Errorf("exit %d, want 2 for an unknown flag", code)
	}
}

func TestUsageAndVersion(t *testing.T) {
	if code, out, _ := runCLI(t, "version"); code != 0 || !strings.HasPrefix(out, "deskos ") {
		t.Errorf("version: exit %d, %q", code, out)
	}
	if code, _, errOut := runCLI(t, "nonsense"); code != 2 || !strings.Contains(errOut, "unknown command") {
		t.Errorf("unknown command: exit %d, %q", code, errOut)
	}
	if code, out, _ := runCLI(t, "help"); code != 0 || !strings.Contains(out, "deskos report") {
		t.Errorf("help: exit %d, %q", code, out)
	}
}

func TestWindowCommand(t *testing.T) {
	now := time.Now()
	day := strings.ToLower(now.Weekday().String()[:3])
	if code, _, errOut := runCLI(t, "window", "--days", day, "--at", now.Format("15:04"), "--grace", "1h"); code != 0 {
		t.Errorf("exact window at now: exit %d, %s", code, errOut)
	}
	if code, _, _ := runCLI(t, "window", "--days", day, "--at", now.Add(12*time.Hour).Format("15:04"), "--grace", "1m"); code != 1 {
		t.Errorf("exact window far away: exit %d, want 1", code)
	}
	if now.Hour() != 0 { // the window helper deliberately stays inside one calendar day
		if code, _, errOut := runCLI(t, "window", "--days", day, "--start", now.Add(-time.Minute).Format("15:04"), "--duration", "2h"); code != 0 {
			t.Errorf("window around now: exit %d, %s", code, errOut)
		}
	}
	if code, _, _ := runCLI(t, "window", "--days", day, "--start", now.Add(12*time.Hour).Format("15:04"), "--duration", "1h"); code != 1 {
		t.Errorf("window far away: exit %d, want 1", code)
	}
	for _, args := range [][]string{
		{"window"},
		{"window", "--days", "moonday", "--at", "03:00"},
		{"window", "--days", "mon", "--at", "3:00"},
		{"window", "--days", "mon", "--at", "03:00", "--start", "03:00", "--duration", "2h"},
		{"window", "--days", "mon", "--at", "03:00", "--grace", "-1m"},
	} {
		if code, _, _ := runCLI(t, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	utc := time.Now().UTC()
	utcDay := strings.ToLower(utc.Weekday().String()[:3])
	if code, _, errOut := runCLI(t, "window", "--days", utcDay, "--at", utc.Format("15:04"), "--grace", "1h", "--timezone", "UTC"); code != 0 {
		t.Errorf("UTC window at now: exit %d, %s", code, errOut)
	}
}
