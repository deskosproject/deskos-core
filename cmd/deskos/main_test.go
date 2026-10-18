package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"NOT checked: anything under /etc or /var",
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
	code, _, errOut := runCLI(t, "update", "--root", t.TempDir())
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "does not carry a plan") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestUpdateDryRunSaysWhatItWouldDo(t *testing.T) {
	// Nothing runs on a machine without bootc, so the guard refuses first: the dry run
	// is what shows the command the guard protects.
	root := withPlan(t, nil)
	code, out, _ := runCLI(t, "update", "--root", root, "--dry-run")
	if code != 0 {
		// A refusal is also correct: the test machine is not a bootc deployment. What
		// must never happen is an update running.
		if !strings.Contains(out, "REFUSED") {
			t.Fatalf("exit %d with %q", code, out)
		}
		return
	}
	if !strings.Contains(out, "would run: bootc upgrade") {
		t.Errorf("dry run said:\n%s", out)
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
