package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestValidate(t *testing.T) {
	code, out, errOut := runCLI("validate", "../../resources", "../../examples/example-org")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "example-devops-rhel10") || !strings.Contains(out, "deskos-core-centos10") {
		t.Errorf("unexpected output: %s", out)
	}
	// An organization overlay alone cannot resolve DeskOS Core.
	code, _, errOut = runCLI("validate", "../../examples/example-org")
	if code != exitInvalid || !strings.Contains(errOut, "Profile/deskos-core, which is not defined") {
		t.Errorf("exit %d: %s", code, errOut)
	}
}

func TestPlanFlagsAfterRoots(t *testing.T) {
	code, out, errOut := runCLI("plan", "../../resources", "../../examples/example-org", "--workstation", "example-devops-rhel10")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{"WORKSTATION", "PLATFORM", "registry.redhat.io/rhel10/rhel-bootc", "PROFILES", "organization:", "ARTIFACT", "/usr/local/bin/oc", "PROVISIONING", "ENROLLMENT"} {
		if !strings.Contains(out, want) {
			t.Errorf("plan output lacks %q", want)
		}
	}
	code, out, _ = runCLI("plan", "--format", "json", "--workstation", "deskos-core-centos10", "../../resources")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(out), &p); err != nil || p["format"] != "plan.deskos.org/v1alpha1" {
		t.Fatalf("invalid JSON plan: %v", err)
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{{}, {"frobnicate"}, {"plan", "../../resources"}, {"render", "../../resources", "--workstation", "x"}, {"plan", "--format", "yaml", "--workstation", "x", "."}} {
		if code, _, _ := runCLI(args...); code != exitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, exitUsage)
		}
	}
}

func TestRender(t *testing.T) {
	out := filepath.Join(t.TempDir(), "ctx")
	code, _, errOut := runCLI("render", "../../resources", "--workstation", "deskos-core-centos10", "--backend", "containerfile", "--output", out)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, f := range []string{"Containerfile", "plan.json", "generated-manifest.json", "rootfs/etc/dconf/db/distro.d/50-deskos"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	code, _, errOut = runCLI("render", "../../resources", "--workstation", "deskos-core-centos10", "--backend", "other", "--output", out)
	if code != exitUsage {
		t.Errorf("unknown backend: exit %d: %s", code, errOut)
	}
}

func TestValidatePrintsMaskedDockWarnings(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"platform.yaml": `apiVersion: core.deskos.org/v1alpha1
kind: Platform
metadata: {name: p}
spec:
  displayName: P
  family: test
  release: "1"
  architectures: [x86_64]
  bootc: {image: example.org/p/bootc:1, digest: "sha256:` + strings.Repeat("0", 64) + `"}
  distribution: {redistributable: true, requiresSubscription: false}
  gnome:
    dconfDatabase: distro
    enabledExtensions: [background-logo@fedorahosted.org]
    extensions:
      - {name: dash-to-dock, package: gnome-shell-extension-dash-to-dock, uuid: dash-to-dock@micxgx.gmail.com}
  flatpak: {preinstall: false}
`,
		"ws.yaml":    "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata: {name: ws}\nspec: {platformRef: p, profiles: [core, org]}\n",
		"core.yaml":  "apiVersion: core.deskos.org/v1alpha1\nkind: Profile\nmetadata: {name: core}\nspec:\n  layer: foundation\n  resources: [{kind: GnomeProfile, name: lower}]\n",
		"org.yaml":   "apiVersion: core.deskos.org/v1alpha1\nkind: Profile\nmetadata: {name: org}\nspec:\n  layer: organization\n  resources: [{kind: GnomeProfile, name: off}]\n",
		"lower.yaml": "apiVersion: desktop.deskos.org/v1alpha1\nkind: GnomeProfile\nmetadata: {name: lower}\nspec:\n  defaults:\n    dock: {enabled: true, position: left}\n",
		"off.yaml":   "apiVersion: desktop.deskos.org/v1alpha1\nkind: GnomeProfile\nmetadata: {name: off}\nspec:\n  defaults:\n    dock: {enabled: false}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errOut := runCLI("validate", dir)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	want := "warning: Workstation/ws: dock.position from foundation layer is masked: dock.enabled is false at organization layer (GnomeProfile/off)\n"
	if errOut != want {
		t.Errorf("stderr = %q, want %q", errOut, want)
	}
	if !strings.Contains(out, "ok: 1 workstation(s)") {
		t.Errorf("stdout = %q", out)
	}

	_, _, errOut = runCLI("validate", "../../resources", "../../examples/example-org")
	if strings.Contains(errOut, "masked") {
		t.Errorf("current resources report masked dock options:\n%s", errOut)
	}
}

func TestVersion(t *testing.T) {
	code, out, _ := runCLI("version")
	if code != exitOK || !strings.HasPrefix(out, "deskosctl dev (commit ") {
		t.Errorf("exit %d: %q", code, out)
	}
	defer func(v, c string) { version, commit = v, c }(version, commit)
	version, commit = "v1.2.3", "0123456789abcdef"
	if _, out, _ = runCLI("version"); out != "deskosctl v1.2.3 (commit 0123456789abcdef)\n" {
		t.Errorf("release build output %q", out)
	}
	if code, _, _ = runCLI("version", "extra"); code != exitUsage {
		t.Errorf("version with an argument: exit %d", code)
	}
}
