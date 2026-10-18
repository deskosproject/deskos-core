package compiler_test

import (
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
)

// workstationWithFlatpak builds a Workstation whose spec carries the given Flatpak
// intent, so these tests do not need a variant of the shared helper.
func workstationWithFlatpak(profiles, flatpak string) string {
	return "apiVersion: core.deskos.org/v1alpha1\nkind: Workstation\nmetadata:\n  name: ws\nspec:\n" +
		"  platformRef: test-platform\n  profiles: [" + profiles + "]\n" + flatpak
}

const flathubRemote = "apiVersion: software.deskos.org/v1alpha1\nkind: FlatpakRemote\nmetadata:\n  name: flathub\nspec:\n  title: Flathub\n  url: https://dl.flathub.org/repo/\n  gpgKeyFile: keys/flathub.gpg\n"

const retireFlathub = "  flatpak:\n    removeRemotes:\n      - {name: flathub, reason: apps come from the organization's own remote}\n"

func renderFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out, err := containerfile.Render(mustPlan(t, "ws", dir))
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, f := range out {
		m[f.Path] = string(f.Data)
	}
	return m
}

// A remote the base ships, retired by the workstation, leaves the image as a file
// removal, and the plan says so; it is not `flatpak uninstall`, and applications
// already installed from it stay on an updated machine (docs/adr/0017).
func TestWorkstationRetiresARemoteTheBaseShips(t *testing.T) {
	dir := fixture{"platform.yaml": platformYAML}.with(fixture{
		"core.yaml": profile("deskos-core", "foundation"),
		"ws.yaml":   workstationWithFlatpak("deskos-core", retireFlathub),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	if len(p.Provisioning.FlatpakRemovedRemotes) != 1 {
		t.Fatalf("retired remotes = %+v, want one", p.Provisioning.FlatpakRemovedRemotes)
	}
	rm := p.Provisioning.FlatpakRemovedRemotes[0]
	if rm.Name != "flathub" || !strings.Contains(rm.Reason, "organization's own remote") {
		t.Errorf("retired remote = %+v", rm)
	}
	containerfile := renderFiles(t, dir)["Containerfile"]
	// The paths are shell-quoted in the Containerfile, so the assertion names the
	// command and each path rather than one exact line.
	for _, want := range []string{
		"rm -f",
		"/usr/share/flatpak/remotes.d/flathub.flatpakrepo",
		"/etc/flatpak/remotes.d/flathub.flatpakrepo",
	} {
		if !strings.Contains(containerfile, want) {
			t.Errorf("Containerfile lacks %q:\n%s", want, containerfile)
		}
	}
	for _, never := range []string{"flatpak uninstall", "--force"} {
		if strings.Contains(containerfile, never) {
			t.Errorf("a retired remote must never %s", never)
		}
	}
}

func TestARemoteCannotBeDeclaredAndRetiredAtOnce(t *testing.T) {
	dir := fixture{"platform.yaml": platformYAML}.with(fixture{
		"keys/flathub.gpg": "\x99\x01\x02",
		"remote.yaml":      "apiVersion: software.deskos.org/v1alpha1\nkind: FlatpakRemote\nmetadata:\n  name: flathub\nspec:\n  title: Flathub\n  url: https://dl.flathub.org/repo/\n  gpgKeyFile: keys/flathub.gpg\n",
		"core.yaml":        profile("deskos-core", "foundation", "FlatpakRemote/flathub"),
		"ws.yaml":          workstationWithFlatpak("deskos-core", retireFlathub),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, `Flatpak remote "flathub" is retired and declared by the same composition`)
}

// A FlatpakSet names its remote as a declared resource, so the declared-and-retired
// rule is what refuses retiring the remote its applications come from.
func TestARemoteAFlatpakSetUsesCannotBeRetired(t *testing.T) {
	dir := fixture{"platform.yaml": platformYAML}.with(fixture{
		"keys/flathub.gpg": "\x99\x01\x02",
		"remote.yaml":      flathubRemote,
		"apps.yaml":        "apiVersion: software.deskos.org/v1alpha1\nkind: FlatpakSet\nmetadata:\n  name: apps\nspec:\n  remote: flathub\n  applications: [{id: org.example.App}]\n",
		"core.yaml":        profile("deskos-core", "foundation", "FlatpakRemote/flathub", "FlatpakSet/apps"),
		"ws.yaml":          workstationWithFlatpak("deskos-core", retireFlathub),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	mustFail(t, err, `Flatpak remote "flathub" is retired and declared by the same composition`)
}

func TestARetiredRemoteNeedsAReason(t *testing.T) {
	dir := fixture{"platform.yaml": platformYAML}.with(fixture{
		"core.yaml": profile("deskos-core", "foundation"),
		"ws.yaml":   workstationWithFlatpak("deskos-core", "  flatpak:\n    removeRemotes:\n      - {name: flathub}\n"),
	}).dir(t)
	_, err := planOf(t, "ws", dir)
	if err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("a retirement without a reason must be rejected, got %v", err)
	}
}
