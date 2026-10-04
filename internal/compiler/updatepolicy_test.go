package compiler_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/deskosproject/deskos-core/internal/backends/containerfile"
	"github.com/deskosproject/deskos-core/internal/plan"
)

func updatePolicy(name, body string) string {
	return "apiVersion: system.deskos.org/v1alpha1\nkind: UpdatePolicy\nmetadata:\n  name: " + name + "\nspec:\n" + body
}

var updatesPlatformYAML = strings.Replace(platformYAML, "  flatpak: {package: flatpak, preinstall: true}\n",
	"  updates: {imageUpdateUnits: [stock-update.timer, stock-update.service]}\n  flatpak: {package: flatpak, preinstall: true}\n", 1)

var updateFiles = fixture{
	"platform.yaml": updatesPlatformYAML,
	"core.yaml":     profile("core", "foundation", "UpdatePolicy/core"),
	"u-core.yaml": updatePolicy("core", `  image: {automatic: true, schedule: daily, requireACPower: true}
  flatpak: {automatic: true, schedule: daily}
`),
}

func scheduled(p *plan.Plan, kind string) (plan.ScheduledUpdate, bool) {
	for _, u := range p.Artifact.ScheduledUpdates {
		if u.Kind == kind {
			return u, true
		}
	}
	return plan.ScheduledUpdate{}, false
}

func masks(p *plan.Plan) string {
	var out []string
	for _, m := range p.Artifact.SystemdMasks {
		out = append(out, m.Unit)
	}
	return strings.Join(out, " ")
}

func units(p *plan.Plan) string {
	var out []string
	for _, u := range p.Artifact.SystemdUnits {
		out = append(out, u.Unit)
	}
	return strings.Join(out, " ")
}

func TestUpdatePolicyValidation(t *testing.T) {
	c := newCompiler(t)
	for name, tc := range map[string]struct{ body, want string }{
		"invalid schedule": {"  image: {automatic: true, schedule: hourly}\n", "schema validation failed"},
		"calendar text":    {"  image: {automatic: true, schedule: '*-*-* 03:00'}\n", "schema validation failed"},
		"unknown field":    {"  image: {automatic: true, schedule: daily, command: 'bootc upgrade --apply'}\n", "schema validation failed"},
		"unknown area":     {"  firmware: {automatic: true}\n", "schema validation failed"},
		"empty spec":       {"  {}\n", "schema validation failed"},
		"empty area":       {"  image: {}\n", "schema validation failed"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.Load([]string{fixture{"u.yaml": updatePolicy("u", tc.body)}.dir(t)})
			mustFail(t, err, tc.want, "UpdatePolicy/u")
		})
	}
	// The typed decoder enforces the same rules without the schema.
	for name, tc := range map[string]struct{ spec, want string }{
		"invalid schedule": {`{"image":{"automatic":true,"schedule":"hourly"}}`, `image.schedule must be daily or weekly, got "hourly"`},
		"unknown field":    {`{"flatpak":{"automatic":true,"script":"x"}}`, "unknown field"},
		"empty spec":       {`{}`, "set at least one of image or flatpak"},
		"empty area":       {`{"flatpak":{}}`, "flatpak: set at least one of automatic, schedule or requireACPower"},
	} {
		t.Run(name+" typed", func(t *testing.T) {
			res := rawResource("system.deskos.org/v1alpha1", "UpdatePolicy", "x", tc.spec)
			p, err := c.Registry.Lookup(res.GVK())
			if err != nil {
				t.Fatal(err)
			}
			mustFail(t, p.Decode(res), tc.want)
		})
	}
}

func TestUpdatePolicyComposition(t *testing.T) {
	t.Run("core policy", func(t *testing.T) {
		p := mustPlan(t, "ws", updateFiles.with(fixture{"ws.yaml": workstation("ws", "core")}).dir(t))
		img, ok := scheduled(p, plan.UpdateImage)
		if !ok || img.Schedule != "daily" || !img.RequireACPower || img.Timer != "deskos-image-update.timer" || img.RandomizedDelay != "2h" {
			t.Errorf("image update = %+v", img)
		}
		fp, ok := scheduled(p, plan.UpdateFlatpak)
		if !ok || fp.Schedule != "daily" || fp.RequireACPower {
			t.Errorf("flatpak update = %+v", fp)
		}
		if got := masks(p); got != "stock-update.service stock-update.timer" {
			t.Errorf("masks = %q", got)
		}
		if got := units(p); !strings.Contains(got, "deskos-flatpak-update.timer deskos-image-update.timer") {
			t.Errorf("enabled units = %q", got)
		}
	})
	t.Run("higher layer overrides one field", func(t *testing.T) {
		p := mustPlan(t, "ws", updateFiles.with(fixture{
			"ws.yaml":    workstation("ws", "core", "org"),
			"org.yaml":   profile("org", "organization", "UpdatePolicy/org"),
			"u-org.yaml": updatePolicy("org", "  image: {schedule: weekly, requireACPower: false}\n"),
		}).dir(t))
		img, _ := scheduled(p, plan.UpdateImage)
		if img.Schedule != "weekly" || img.RequireACPower {
			t.Errorf("image update = %+v", img)
		}
		var from []string
		for _, pr := range img.Provenance {
			from = append(from, pr.Layer+" "+pr.Resource)
		}
		if got := strings.Join(from, ", "); got != "foundation UpdatePolicy/core, organization UpdatePolicy/org" {
			t.Errorf("provenance = %s", got)
		}
	})
	t.Run("same layer conflict", func(t *testing.T) {
		_, err := planOf(t, "ws", updateFiles.with(fixture{
			"ws.yaml":      workstation("ws", "core", "core2"),
			"core2.yaml":   profile("core2", "foundation", "UpdatePolicy/core2"),
			"u-core2.yaml": updatePolicy("core2", "  image: {schedule: weekly}\n"),
		}).dir(t))
		mustFail(t, err, `composition conflict for update setting "update.image.schedule"`, "UpdatePolicy/core", "UpdatePolicy/core2")
	})
	t.Run("organization turns image updates off", func(t *testing.T) {
		p := mustPlan(t, "ws", updateFiles.with(fixture{
			"ws.yaml":    workstation("ws", "core", "org"),
			"org.yaml":   profile("org", "organization", "UpdatePolicy/org"),
			"u-org.yaml": updatePolicy("org", "  image: {automatic: false}\n"),
		}).dir(t))
		if _, ok := scheduled(p, plan.UpdateImage); ok || strings.Contains(units(p), "deskos-image-update") {
			t.Error("disabled image updates are still scheduled")
		}
		if got := masks(p); got != "stock-update.service stock-update.timer" {
			t.Errorf("the platform updater must stay masked, masks = %q", got)
		}
		want := "update.image.schedule from foundation layer is masked: update.image.automatic is false at organization layer (UpdatePolicy/org)"
		if !strings.Contains(strings.Join(p.Warnings, "\n"), want) {
			t.Errorf("warnings %q lack %q", p.Warnings, want)
		}
	})
	t.Run("option at the disabling layer", func(t *testing.T) {
		_, err := planOf(t, "ws", updateFiles.with(fixture{
			"ws.yaml":    workstation("ws", "core", "org"),
			"org.yaml":   profile("org", "organization", "UpdatePolicy/org"),
			"u-org.yaml": updatePolicy("org", "  flatpak: {automatic: false, schedule: weekly}\n"),
		}).dir(t))
		mustFail(t, err, "update.flatpak.schedule is set at organization layer but update.flatpak.automatic is false at organization layer", "UpdatePolicy/org")
	})
	t.Run("schedule without automatic", func(t *testing.T) {
		_, err := planOf(t, "ws", updateFiles.with(fixture{
			"ws.yaml":    workstation("ws", "org"),
			"org.yaml":   profile("org", "organization", "UpdatePolicy/org"),
			"u-org.yaml": updatePolicy("org", "  image: {schedule: weekly}\n"),
		}).dir(t))
		mustFail(t, err, "update.image.schedule requires update.image.automatic, which no UpdatePolicy sets", "UpdatePolicy/org")
	})
	t.Run("automatic without schedule", func(t *testing.T) {
		_, err := planOf(t, "ws", updateFiles.with(fixture{
			"ws.yaml":    workstation("ws", "org"),
			"org.yaml":   profile("org", "organization", "UpdatePolicy/org"),
			"u-org.yaml": updatePolicy("org", "  flatpak: {automatic: true}\n"),
		}).dir(t))
		mustFail(t, err, "update.flatpak.schedule is required when update.flatpak.automatic is true", "UpdatePolicy/org")
	})
	t.Run("platform without update facts", func(t *testing.T) {
		_, err := planOf(t, "ws", updateFiles.with(fixture{"platform.yaml": platformYAML, "ws.yaml": workstation("ws", "core")}).dir(t))
		mustFail(t, err, "Platform/test-platform declares no update facts", "UpdatePolicy/core")
	})
	t.Run("flatpak policy needs no update facts", func(t *testing.T) {
		p := mustPlan(t, "ws", updateFiles.with(fixture{
			"platform.yaml": platformYAML,
			"u-core.yaml":   updatePolicy("core", "  flatpak: {automatic: true, schedule: weekly}\n"),
			"ws.yaml":       workstation("ws", "core"),
		}).dir(t))
		if len(p.Artifact.SystemdMasks) != 0 || !strings.Contains(strings.Join(packages(p), " "), "flatpak") {
			t.Errorf("masks %q, packages %q", masks(p), packages(p))
		}
	})
}

// The generated units only stage the image or update Flatpaks: the content
// is fixed, and nothing that reboots or applies is ever rendered.
func TestUpdatePolicyUnits(t *testing.T) {
	files, err := containerfile.Render(mustPlan(t, "deskos-core-centos10", resourcesRoot))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"deskos-image-update.service", "deskos-image-update.timer", "deskos-flatpak-update.service", "deskos-flatpak-update.timer"} {
		golden(t, name, fileData(t, files, "rootfs/usr/lib/systemd/system/"+name))
	}
	cf := string(fileData(t, files, containerfile.ContainerfilePath))
	if !strings.Contains(cf, "systemctl mask 'bootc-fetch-apply-updates.service' 'bootc-fetch-apply-updates.timer'") {
		t.Error("the Containerfile does not mask the platform's rebooting updater")
	}

	dir := updateFiles.with(fixture{"ws.yaml": workstation("ws", "core")}).dir(t)
	a, err := containerfile.Render(mustPlan(t, "ws", dir))
	if err != nil {
		t.Fatal(err)
	}
	b, err := containerfile.Render(mustPlan(t, "ws", dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatal("render is not deterministic")
	}
	for i := range a {
		if a[i].Path != b[i].Path || !bytes.Equal(a[i].Data, b[i].Data) {
			t.Fatalf("render is not deterministic at %s", a[i].Path)
		}
	}
	if svc := string(fileData(t, a, "rootfs/usr/lib/systemd/system/deskos-flatpak-update.service")); strings.Contains(svc, "ConditionACPower") {
		t.Errorf("flatpak update without requireACPower has an AC condition:\n%s", svc)
	}

	rhel, err := containerfile.Render(mustPlan(t, "example-devops-rhel10", resourcesRoot, exampleRoot))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range append(append(files, a...), rhel...) {
		for _, bad := range []string{"--apply", "--soft-reboot", "--download-only", "systemctl reboot", "shutdown -r"} {
			if bytes.Contains(f.Data, []byte(bad)) {
				t.Errorf("%s contains %q", f.Path, bad)
			}
		}
	}
}
