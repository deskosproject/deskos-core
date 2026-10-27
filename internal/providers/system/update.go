package system

import (
	"fmt"
	"strconv"
	"time"
	_ "time/tzdata"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
)

var UpdatePolicyGVK = model.GVK{Group: "system.deskos.org", Version: model.V1Alpha1, Kind: "UpdatePolicy"}

// DomainUpdate holds layered update settings.
var DomainUpdate = compose.Domain{ID: "update-setting", Label: "update setting"}

// Generated update units; the endpoint interface of ADR 0007.
const (
	ImageUpdateService   = "deskos-image-update.service"
	ImageUpdateTimer     = "deskos-image-update.timer"
	ImageApplyService    = "deskos-image-apply.service"
	ImageApplyTimer      = "deskos-image-apply.timer"
	FlatpakUpdateService = "deskos-flatpak-update.service"
	FlatpakUpdateTimer   = "deskos-flatpak-update.timer"
)

// UpdateRandomizedDelay spreads timer runs across a fleet, as the stock bootc timer does.
const UpdateRandomizedDelay = "2h"

// UpdatePolicySpec is the public UpdatePolicy spec.
type UpdatePolicySpec struct {
	Image   *AutomaticUpdate `json:"image,omitempty"`
	Flatpak *AutomaticUpdate `json:"flatpak,omitempty"`
}

// AutomaticUpdate is the policy of one update area.
type AutomaticUpdate struct {
	Automatic      *bool        `json:"automatic,omitempty"`
	Schedule       string       `json:"schedule,omitempty"`
	RequireACPower *bool        `json:"requireACPower,omitempty"`
	Apply          *ApplyPolicy `json:"apply,omitempty"`
}

// ApplyPolicy is the explicit consent to apply an image and reboot. It is
// valid only for the image area and only inside a requested window or at an
// exact local time.
type ApplyPolicy struct {
	Enabled  bool         `json:"enabled"`
	Days     []string     `json:"days,omitempty"`
	At       string       `json:"at,omitempty"`
	Window   *ApplyWindow `json:"window,omitempty"`
	Timezone string       `json:"timezone,omitempty"`
}

// ApplyWindow is the interval in which an apply may start.
type ApplyWindow struct {
	Start    string `json:"start"`
	Duration string `json:"duration"`
}

var weekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

// updateArea is one updatable part of the workstation.
type updateArea struct {
	name, kind, service, timer string
}

var updateAreas = []updateArea{
	{"image", plan.UpdateImage, ImageUpdateService, ImageUpdateTimer},
	{"flatpak", plan.UpdateFlatpak, FlatpakUpdateService, FlatpakUpdateTimer},
}

func (a updateArea) key(field string) string { return "update." + a.name + "." + field }

type updatePolicy struct{}

func (updatePolicy) GVK() model.GVK { return UpdatePolicyGVK }
func (updatePolicy) Schema() string { return "system.deskos.org/v1alpha1/updatepolicy.json" }

func (updatePolicy) Decode(res *model.Resource) error {
	var s UpdatePolicySpec
	if err := model.DecodeSpec(res, &s); err != nil {
		return err
	}
	var errs model.ErrorList
	if s.Image == nil && s.Flatpak == nil {
		errs.Add(model.Errorf(res, "set at least one of image or flatpak"))
	}
	for name, u := range map[string]*AutomaticUpdate{"image": s.Image, "flatpak": s.Flatpak} {
		if u == nil {
			continue
		}
		if u.Automatic == nil && u.Schedule == "" && u.RequireACPower == nil && u.Apply == nil {
			options := "automatic, schedule or requireACPower"
			if name == "image" {
				options += " or apply"
			}
			errs.Add(model.Errorf(res, "%s: set at least one of %s", name, options))
		}
		if u.Schedule != "" && u.Schedule != "daily" && u.Schedule != "weekly" {
			errs.Add(model.Errorf(res, "%s.schedule must be daily or weekly, got %q", name, u.Schedule))
		}
		if u.Apply != nil {
			if name != "image" {
				errs.Add(model.Errorf(res, "%s.apply is only valid for image updates", name))
			} else {
				for _, msg := range validateApply(u.Apply) {
					errs.Add(model.Errorf(res, "%s", msg))
				}
			}
		}
	}
	res.Object = &s
	return errs.Err()
}

// validateApply checks the fields the schema cannot express: the relationship
// between enabled, days, and exactly one of at or window.
func validateApply(a *ApplyPolicy) []string {
	var errs []string
	if !a.Enabled {
		if len(a.Days) > 0 || a.At != "" || a.Window != nil || a.Timezone != "" {
			errs = append(errs, "apply.enabled is false; remove days, at, window and timezone")
		}
		return errs
	}
	if len(a.Days) == 0 {
		errs = append(errs, "apply.days is required when apply.enabled is true")
	}
	seen := map[string]bool{}
	for _, day := range a.Days {
		if !containsString(weekdays, day) {
			errs = append(errs, fmt.Sprintf("apply.days: unknown day %q (expected %s)", day, joinWords(weekdays)))
		}
		if seen[day] {
			errs = append(errs, fmt.Sprintf("apply.days: %q is listed twice", day))
		}
		seen[day] = true
	}
	if (a.At == "") == (a.Window == nil) {
		errs = append(errs, "apply: set exactly one of at or window")
	}
	if a.At != "" {
		if err := validateClock(a.At); err != nil {
			errs = append(errs, "apply.at: "+err.Error())
		}
	}
	if a.Window != nil {
		if err := validateClock(a.Window.Start); err != nil {
			errs = append(errs, "apply.window.start: "+err.Error())
		} else {
			start, _ := time.Parse("15:04", a.Window.Start)
			d, err := time.ParseDuration(a.Window.Duration)
			if err != nil || d <= 0 || d%time.Minute != 0 {
				errs = append(errs, fmt.Sprintf("apply.window.duration: %q is not a positive whole-minute duration", a.Window.Duration))
			} else if start.Hour()*60+start.Minute()+int(d/time.Minute) > 24*60 {
				errs = append(errs, "apply.window: the window must end on the same day")
			}
		}
	}
	if a.Timezone != "" {
		if _, err := time.LoadLocation(a.Timezone); err != nil {
			errs = append(errs, fmt.Sprintf("apply.timezone: unknown time zone %q", a.Timezone))
		}
	}
	return errs
}

func validateClock(s string) error {
	if len(s) != 5 || s[2] != ':' {
		return fmt.Errorf("%q is not HH:MM", s)
	}
	if _, err := time.Parse("15:04", s); err != nil {
		return fmt.Errorf("%q is not HH:MM", s)
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func joinWords(list []string) string {
	out := ""
	for i, s := range list {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func (updatePolicy) Contribute(res *model.Resource, sc *compose.Scope) error {
	s := res.Object.(*UpdatePolicySpec)
	for _, a := range updateAreas {
		u := s.Image
		if a.name == "flatpak" {
			u = s.Flatpak
		}
		if u == nil {
			continue
		}
		if u.Automatic != nil {
			sc.SetScalar(DomainUpdate, a.key("automatic"), *u.Automatic, strconv.FormatBool(*u.Automatic))
		}
		if u.Schedule != "" {
			sc.SetScalar(DomainUpdate, a.key("schedule"), u.Schedule, u.Schedule)
		}
		if u.RequireACPower != nil {
			sc.SetScalar(DomainUpdate, a.key("requireACPower"), *u.RequireACPower, strconv.FormatBool(*u.RequireACPower))
		}
		if u.Apply != nil {
			sc.SetScalar(DomainUpdate, a.key("apply.enabled"), u.Apply.Enabled, strconv.FormatBool(u.Apply.Enabled))
			if len(u.Apply.Days) > 0 {
				sc.SetScalar(DomainUpdate, a.key("apply.days"), u.Apply.Days, joinWords(u.Apply.Days))
			}
			if u.Apply.At != "" {
				sc.SetScalar(DomainUpdate, a.key("apply.at"), u.Apply.At, u.Apply.At)
			}
			if u.Apply.Window != nil {
				sc.SetScalar(DomainUpdate, a.key("apply.window.start"), u.Apply.Window.Start, u.Apply.Window.Start)
				sc.SetScalar(DomainUpdate, a.key("apply.window.duration"), u.Apply.Window.Duration, u.Apply.Window.Duration)
			}
			if u.Apply.Timezone != "" {
				sc.SetScalar(DomainUpdate, a.key("apply.timezone"), u.Apply.Timezone, u.Apply.Timezone)
			}
		}
	}
	return nil
}

// lowerUpdates masks the platform's own image updater when image intent is
// set and schedules the generated, non-rebooting update units.
func lowerUpdates(c *compose.Composition, p *plan.Plan) error {
	var errs model.ErrorList
	for _, a := range updateAreas {
		errs.Add(lowerUpdateArea(c, p, a))
	}
	return errs.Err()
}

func lowerUpdateArea(c *compose.Composition, p *plan.Plan, a updateArea) error {
	auto, hasAuto := c.Result.Scalar(DomainUpdate, a.key("automatic"))
	var options []compose.Scalar
	for _, f := range []string{
		"schedule", "requireACPower",
		"apply.enabled", "apply.days", "apply.at",
		"apply.window.start", "apply.window.duration", "apply.timezone",
	} {
		if s, ok := c.Result.Scalar(DomainUpdate, a.key(f)); ok {
			options = append(options, s)
		}
	}
	if !hasAuto {
		var errs model.ErrorList
		for _, o := range options {
			errs.Add(fmt.Errorf("%s requires %s, which no UpdatePolicy sets\n  set by: %s", o.Key, a.key("automatic"), describe(o.Provenance)))
		}
		return errs.Err()
	}
	enabled := auto.Value.(bool)
	if !enabled {
		var errs model.ErrorList
		for _, o := range options {
			if o.Layer < auto.Layer {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s from %s layer is masked: %s is false at %s layer (%s)",
					o.Key, o.Layer, auto.Key, auto.Layer, describeShort(auto.Provenance)))
				continue
			}
			errs.Add(fmt.Errorf("%s is set at %s layer but %s is false at %s layer\n  set by: %s\n  disabled by: %s",
				o.Key, o.Layer, auto.Key, auto.Layer, describe(o.Provenance), describe(auto.Provenance)))
		}
		if err := errs.Err(); err != nil {
			return err
		}
	}

	pl := c.PlatformSpec
	if a.kind == plan.UpdateImage {
		if pl.Updates == nil {
			return fmt.Errorf("%s declares no update facts; %s cannot be applied\n  set by: %s", c.Platform.ID(), auto.Key, describe(auto.Provenance))
		}
		for _, u := range pl.Updates.ImageUpdateUnits {
			p.MaskUnit(u, auto.Provenance...)
		}
	}
	if a.kind == plan.UpdateFlatpak {
		// The on-demand service needs the Flatpak package too, so the package
		// is installed whether or not the timer is scheduled.
		if pl.Flatpak.Package == "" {
			return fmt.Errorf("%s declares no flatpak.package; %s cannot be applied\n  set by: %s", c.Platform.ID(), auto.Key, describe(auto.Provenance))
		}
		p.AddPackage(pl.Flatpak.Package, auto.Provenance...)
	}
	if !enabled {
		// The service is still generated: an administrator can start the same,
		// non-rebooting update on demand with an endpoint command, without
		// re-enabling the timer. Only the timer follows `automatic`.
		p.Artifact.ScheduledUpdates = append(p.Artifact.ScheduledUpdates, plan.ScheduledUpdate{
			Kind: a.kind, Service: a.service,
			Provenance: append([]model.Provenance(nil), auto.Provenance...),
		})
		return nil
	}
	if a.kind == plan.UpdateImage {
		if err := lowerImageApply(c, p, a, true); err != nil {
			return err
		}
	}
	sched, ok := c.Result.Scalar(DomainUpdate, a.key("schedule"))
	if !ok {
		return fmt.Errorf("%s is required when %s is true\n  set by: %s", a.key("schedule"), auto.Key, describe(auto.Provenance))
	}
	prov := append([]model.Provenance(nil), auto.Provenance...)
	prov = append(prov, sched.Provenance...)
	requireAC := false
	if ac, ok := c.Result.Scalar(DomainUpdate, a.key("requireACPower")); ok {
		requireAC = ac.Value.(bool)
		prov = append(prov, ac.Provenance...)
	}
	p.Artifact.ScheduledUpdates = append(p.Artifact.ScheduledUpdates, plan.ScheduledUpdate{
		Kind: a.kind, Service: a.service, Timer: a.timer,
		Schedule: sched.Value.(string), RandomizedDelay: UpdateRandomizedDelay,
		RequireACPower: requireAC, Provenance: prov,
	})
	if a.kind == plan.UpdateImage && requireAC {
		// A staging run skipped on battery waits for the next timer, which can be
		// a day away. When AC comes back, udev starts the same service; its own
		// ConditionACPower still decides, so a battery-only event does nothing.
		rule := fmt.Sprintf("# Generated by deskosctl. Do not edit; change the DeskOS resources instead.\n"+
			"# Start the staging service when AC power comes online.\n"+
			"ACTION==\"change\", SUBSYSTEM==\"power_supply\", ATTR{online}==\"1\", TAG+=\"systemd\", ENV{SYSTEMD_WANTS}+=\"%s\"\n",
			a.service)
		p.Artifact.GeneratedFiles = append(p.Artifact.GeneratedFiles, plan.GeneratedFile{
			Path:    "/usr/lib/udev/rules.d/60-deskos-update-on-ac.rules",
			Mode:    "0644",
			Content: rule,
			// The rule exists because the policy is automatic; the scheduled
			// update already carries the schedule and AC provenance.
			Provenance: append([]model.Provenance(nil), auto.Provenance...),
		})
	}
	p.EnableUnit(a.timer, prov...)
	return nil
}

// lowerImageApply adds the separate apply timer. It is independent of the
// staging timer: the image is still downloaded daily, and this timer is the
// only place where --apply (and therefore a reboot) is allowed.
func lowerImageApply(c *compose.Composition, p *plan.Plan, a updateArea, automatic bool) error {
	apply, hasApply := c.Result.Scalar(DomainUpdate, a.key("apply.enabled"))
	fields := []string{
		a.key("apply.days"), a.key("apply.at"),
		a.key("apply.window.start"), a.key("apply.window.duration"), a.key("apply.timezone"),
	}
	if !hasApply {
		for _, key := range fields {
			if s, ok := c.Result.Scalar(DomainUpdate, key); ok {
				return fmt.Errorf("%s requires %s, which no UpdatePolicy sets\n  set by: %s",
					key, a.key("apply.enabled"), describe(s.Provenance))
			}
		}
		return nil
	}
	if !apply.Value.(bool) {
		for _, key := range fields {
			if s, ok := c.Result.Scalar(DomainUpdate, key); ok {
				if s.Layer < apply.Layer {
					p.Warnings = append(p.Warnings, fmt.Sprintf("%s from %s layer is masked: %s is false at %s layer (%s)",
						key, s.Layer, a.key("apply.enabled"), apply.Layer, describeShort(apply.Provenance)))
					continue
				}
				return fmt.Errorf("%s is set at %s layer but %s is false at %s layer\n  set by: %s\n  disabled by: %s",
					key, s.Layer, a.key("apply.enabled"), apply.Layer, describe(s.Provenance), describe(apply.Provenance))
			}
		}
		return nil
	}
	if !automatic {
		return fmt.Errorf("%s requires %s to be true\n  set by: %s", a.key("apply.enabled"), a.key("automatic"), describe(apply.Provenance))
	}
	days, ok := c.Result.Scalar(DomainUpdate, a.key("apply.days"))
	if !ok {
		return fmt.Errorf("%s is required when %s is true\n  set by: %s", a.key("apply.days"), a.key("apply.enabled"), describe(apply.Provenance))
	}
	at, hasAt := c.Result.Scalar(DomainUpdate, a.key("apply.at"))
	start, hasStart := c.Result.Scalar(DomainUpdate, a.key("apply.window.start"))
	duration, hasDuration := c.Result.Scalar(DomainUpdate, a.key("apply.window.duration"))
	if hasAt == (hasStart || hasDuration) {
		return fmt.Errorf("%s: set exactly one of %s or %s\n  set by: %s",
			a.key("apply.enabled"), a.key("apply.at"), a.key("apply.window.start"), describe(apply.Provenance))
	}
	if hasStart != hasDuration {
		return fmt.Errorf("%s and %s must be set together\n  set by: %s",
			a.key("apply.window.start"), a.key("apply.window.duration"), describe(apply.Provenance))
	}
	prov := append([]model.Provenance(nil), apply.Provenance...)
	prov = append(prov, days.Provenance...)
	if hasAt {
		prov = append(prov, at.Provenance...)
	}
	if hasStart {
		prov = append(prov, start.Provenance...)
		prov = append(prov, duration.Provenance...)
	}
	if tz, ok := c.Result.Scalar(DomainUpdate, a.key("apply.timezone")); ok {
		prov = append(prov, tz.Provenance...)
	}
	requireAC := false
	if ac, ok := c.Result.Scalar(DomainUpdate, a.key("requireACPower")); ok {
		requireAC = ac.Value.(bool)
	}
	u := plan.ScheduledUpdate{
		Kind: plan.UpdateImageApply, Service: ImageApplyService, Timer: ImageApplyTimer,
		Days: days.Value.([]string), RequireACPower: requireAC, Provenance: prov,
	}
	if hasAt {
		u.At = at.Value.(string)
	} else {
		u.Window = &plan.UpdateWindow{Start: start.Value.(string), Duration: duration.Value.(string)}
	}
	if tz, ok := c.Result.Scalar(DomainUpdate, a.key("apply.timezone")); ok {
		u.Timezone = tz.Value.(string)
	}
	p.Artifact.ScheduledUpdates = append(p.Artifact.ScheduledUpdates, u)
	p.EnableUnit(ImageApplyTimer, prov...)
	return nil
}
