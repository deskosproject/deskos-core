package system

import (
	"fmt"
	"strconv"

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
	Automatic      *bool  `json:"automatic,omitempty"`
	Schedule       string `json:"schedule,omitempty"`
	RequireACPower *bool  `json:"requireACPower,omitempty"`
}

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
		if u.Automatic == nil && u.Schedule == "" && u.RequireACPower == nil {
			errs.Add(model.Errorf(res, "%s: set at least one of automatic, schedule or requireACPower", name))
		}
		if u.Schedule != "" && u.Schedule != "daily" && u.Schedule != "weekly" {
			errs.Add(model.Errorf(res, "%s.schedule must be daily or weekly, got %q", name, u.Schedule))
		}
	}
	res.Object = &s
	return errs.Err()
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
	for _, f := range []string{"schedule", "requireACPower"} {
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
	p.EnableUnit(a.timer, prov...)
	return nil
}
