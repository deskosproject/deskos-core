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
		// The timer is not scheduled, but the service stays so an endpoint
		// command can run the same, non-rebooting update on demand.
		u, ok := scheduled(p, plan.UpdateImage)
		if !ok || u.Timer != "" || u.Schedule != "" {
			t.Errorf("with automatic false the image service must stay unscheduled: %+v", u)
		}
		if strings.Contains(units(p), "deskos-image-update") {
			t.Error("no update unit is enabled when image updates are off")
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

func TestUpdatePolicyApplyValidation(t *testing.T) {
	c := newCompiler(t)
	for name, body := range map[string]string{
		"exact":  "  image: {automatic: true, schedule: daily, apply: {enabled: true, days: [sunday], at: '03:00', timezone: Europe/Madrid}}\n",
		"window": "  image: {automatic: true, schedule: daily, apply: {enabled: true, days: [sunday, wednesday], window: {start: '03:00', duration: 2h}}}\n",
		"off":    "  image: {automatic: true, schedule: daily, apply: {enabled: false}}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := c.Load([]string{fixture{"u.yaml": updatePolicy("u", body)}.dir(t)}); err != nil {
				t.Fatalf("valid apply policy rejected: %v", err)
			}
		})
	}
	for name, tc := range map[string]struct{ body, want string }{
		"both modes":      {"  image: {automatic: true, schedule: daily, apply: {enabled: true, days: [sunday], at: '03:00', window: {start: '03:00', duration: 2h}}}\n", "apply: set exactly one of at or window"},
		"no days":         {"  image: {automatic: true, schedule: daily, apply: {enabled: true, at: '03:00'}}\n", "apply.days is required"},
		"no mode":         {"  image: {automatic: true, schedule: daily, apply: {enabled: true, days: [sunday]}}\n", "apply: set exactly one of at or window"},
		"bad time":        {"  image: {automatic: true, schedule: daily, apply: {enabled: true, days: [sunday], at: '3:00'}}\n", "schema validation failed"},
		"bad duration":    {"  image: {automatic: true, schedule: daily, apply: {enabled: true, days: [sunday], window: {start: '03:00', duration: 2h30}}}\n", "schema validation failed"},
		"bad timezone":    {"  image: {automatic: true, schedule: daily, apply: {enabled: true, days: [sunday], at: '03:00', timezone: Mars/Olympus}}\n", "apply.timezone: unknown time zone"},
		"unknown day":     {"  image: {automatic: true, schedule: daily, apply: {enabled: true, days: [moonday], at: '03:00'}}\n", "schema validation failed"},
		"off with fields": {"  image: {automatic: true, schedule: daily, apply: {enabled: false, days: [sunday]}}\n", "apply.enabled is false"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.Load([]string{fixture{"u.yaml": updatePolicy("u", tc.body)}.dir(t)})
			mustFail(t, err, tc.want)
		})
	}
}

func TestUpdatePolicyApplyTypedValidation(t *testing.T) {
	c := newCompiler(t)
	for name, tc := range map[string]struct{ spec, want string }{
		"no days": {
			`{"image":{"automatic":true,"schedule":"daily","apply":{"enabled":true,"at":"03:00"}}}`,
			"apply.days is required when apply.enabled is true",
		},
		"both modes": {
			`{"image":{"automatic":true,"schedule":"daily","apply":{"enabled":true,"days":["sunday"],"at":"03:00","window":{"start":"03:00","duration":"2h"}}}}`,
			"apply: set exactly one of at or window",
		},
		"bad timezone": {
			`{"image":{"automatic":true,"schedule":"daily","apply":{"enabled":true,"days":["sunday"],"at":"03:00","timezone":"Mars/Olympus"}}}`,
			"apply.timezone: unknown time zone",
		},
		"flatpak apply": {
			`{"flatpak":{"automatic":true,"schedule":"daily","apply":{"enabled":true,"days":["sunday"],"at":"03:00"}}}`,
			"flatpak.apply is only valid for image updates",
		},
	} {
		t.Run(name, func(t *testing.T) {
			res := rawResource("system.deskos.org/v1alpha1", "UpdatePolicy", "x", tc.spec)
			p, err := c.Registry.Lookup(res.GVK())
			if err != nil {
				t.Fatal(err)
			}
			mustFail(t, p.Decode(res), tc.want)
		})
	}
}

func TestUpdatePolicyApplyLowering(t *testing.T) {
	dir := updateFiles.with(fixture{
		"ws.yaml":    workstation("ws", "core", "org"),
		"org.yaml":   profile("org", "organization", "UpdatePolicy/org"),
		"u-org.yaml": updatePolicy("org", "  image: {apply: {enabled: true, days: [sunday], at: '03:00', timezone: Europe/Madrid}}\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	u, ok := scheduled(p, plan.UpdateImageApply)
	if !ok {
		t.Fatal("image apply was not scheduled")
	}
	if u.Service != "deskos-image-apply.service" || u.Timer != "deskos-image-apply.timer" ||
		u.At != "03:00" || len(u.Days) != 1 || u.Days[0] != "sunday" || u.Timezone != "Europe/Madrid" {
		t.Errorf("apply update = %+v", u)
	}
	if got := units(p); !strings.Contains(got, "deskos-image-apply.timer") {
		t.Errorf("enabled units = %q", got)
	}

	files, err := containerfile.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	svc := string(fileData(t, files, "rootfs/usr/lib/systemd/system/deskos-image-apply.service"))
	for _, want := range []string{
		"ExecCondition=/usr/bin/deskos window --days sun --at 03:00 --grace 5m --timezone Europe/Madrid",
		"ExecStart=/usr/bin/bootc upgrade --apply --quiet",
	} {
		if !strings.Contains(svc, want) {
			t.Errorf("apply service lacks %q:\n%s", want, svc)
		}
	}
	timer := string(fileData(t, files, "rootfs/usr/lib/systemd/system/deskos-image-apply.timer"))
	for _, want := range []string{
		"OnCalendar=Sun *-*-* 03:00:00 Europe/Madrid",
		"AccuracySec=1min",
		"Persistent=false",
	} {
		if !strings.Contains(timer, want) {
			t.Errorf("apply timer lacks %q:\n%s", want, timer)
		}
	}
	if strings.Contains(timer, "RandomizedDelaySec") {
		t.Errorf("exact apply timer has a randomized delay:\n%s", timer)
	}
}

func TestUpdatePolicyApplyWindowLowering(t *testing.T) {
	dir := updateFiles.with(fixture{
		"ws.yaml":    workstation("ws", "core", "org"),
		"org.yaml":   profile("org", "organization", "UpdatePolicy/org"),
		"u-org.yaml": updatePolicy("org", "  image: {apply: {enabled: true, days: [sunday, wednesday], window: {start: '03:00', duration: 2h}}}\n"),
	}).dir(t)
	p := mustPlan(t, "ws", dir)
	u, _ := scheduled(p, plan.UpdateImageApply)
	if u.Window == nil || u.Window.Start != "03:00" || u.Window.Duration != "2h" {
		t.Fatalf("apply window = %+v", u)
	}
	files, err := containerfile.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	svc := string(fileData(t, files, "rootfs/usr/lib/systemd/system/deskos-image-apply.service"))
	if !strings.Contains(svc, "ExecCondition=/usr/bin/deskos window --days wed,sun --start 03:00 --duration 2h") {
		t.Errorf("apply service has the wrong window command:\n%s", svc)
	}
	if strings.Contains(svc, "/run/ostree-booted") {
		t.Errorf("apply service requires the ostree-only boot marker:\n%s", svc)
	}
	timer := string(fileData(t, files, "rootfs/usr/lib/systemd/system/deskos-image-apply.timer"))
	for _, want := range []string{
		"OnCalendar=Wed,Sun *-*-* 03:00:00",
		"RandomizedDelaySec=2h",
		"Persistent=false",
	} {
		if !strings.Contains(timer, want) {
			t.Errorf("apply timer lacks %q:\n%s", want, timer)
		}
	}
}

func TestUpdatePolicyApplyNeedsAutomatic(t *testing.T) {
	_, err := planOf(t, "ws", updateFiles.with(fixture{
		"ws.yaml":    workstation("ws", "org"),
		"org.yaml":   profile("org", "organization", "UpdatePolicy/org"),
		"u-org.yaml": updatePolicy("org", "  image: {apply: {enabled: true, days: [sunday], at: '03:00'}}\n"),
	}).dir(t))
	mustFail(t, err, "update.image.apply.enabled requires update.image.automatic, which no UpdatePolicy sets", "UpdatePolicy/org")
}

// The generated units only stage the image or update Flatpaks: the content
// is fixed, and nothing that reboots or applies is ever rendered.
func TestUpdatePolicyUnits(t *testing.T) {
	files, err := containerfile.Render(mustPlan(t, "deskos-core-centos-stream-10", resourcesRoot))
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

	rhel, err := containerfile.Render(mustPlan(t, "example-devops-rhel-10", resourcesRoot, exampleRoot))
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
