package evidence

import (
	"fmt"
	"strings"

	"github.com/deskosproject/deskos-core/internal/plan"
)

// Condition is one thing an update depends on, and what this machine says about it.
type Condition struct {
	Name   string
	Detail string
	// Blocks marks a condition that prevents a safe update. Everything else is
	// reported and left alone.
	Blocks bool
}

// Guard reads the conditions an update depends on.
//
// It is a **guard, not a reconciler** (docs/adr/0018). It acts on what prevents a safe
// update — a machine that is not a bootc deployment, a deployment bootc cannot read —
// and it only *reports* the rest. A machine a user customized is still a machine they
// may update: DeskOS does not reconcile /etc and does not block an update for it
// (AGENTS.md invariant 8, "defaults are not locks"), and an image that updates itself
// with its own timer is not a reason to refuse a person's request.
func Guard(probe Probe) []Condition {
	booted, err := probe.Booted()
	var out []Condition

	if err != nil {
		out = append(out, Condition{
			Name:   "this machine's deployment can be read",
			Detail: fmt.Sprintf("bootc could not read the running deployment (%v); an update would run blind", err),
			Blocks: true,
		})
	} else if !booted.IsBootc {
		out = append(out, Condition{
			Name: "this machine is a bootc deployment",
			Detail: "deskos updates a bootc image; this machine was not installed that way, " +
				"so update it the way it was installed",
			Blocks: true,
		})
	} else {
		out = append(out, Condition{
			Name:   "this machine is a bootc deployment",
			Detail: describeBooted(booted),
		})
	}
	return out
}

// ReportedConditions are the facts a person should see before an update, which do not
// prevent it. They are read from the plan, because they are properties of the image.
func ReportedConditions(p *plan.Plan) []Condition {
	var out []Condition
	if p == nil {
		return out
	}
	if kinds := UpdatingKinds(p); len(kinds) > 0 {
		out = append(out, Condition{
			Name: "this image updates itself",
			Detail: fmt.Sprintf("%s update(s) are scheduled by the image itself (%s), "+
				"so a manual update may find nothing to do or race with its own timer",
				strings.Join(kinds, ", "), strings.Join(UpdatingKinds(p), ", ")),
		})
	}
	etc := etcPaths(p)
	if len(etc) > 0 {
		out = append(out, Condition{
			Name: "customization of /etc is not touched",
			Detail: fmt.Sprintf("DeskOS does not reconcile or block updates for /etc; this plan owns %d "+
				"file(s) there (%s), and whatever a user changed about them stays theirs",
				len(etc), strings.Join(first(etc, 3), ", ")),
		})
	}
	return out
}

func describeBooted(b Booted) string {
	if b.Image == "" {
		return "booted from a bootc deployment"
	}
	if b.Digest == "" {
		return fmt.Sprintf("booted from %s", b.Image)
	}
	return fmt.Sprintf("booted from %s @ %s", b.Image, b.Digest)
}

// UpdatingKinds are the kinds of update the image schedules itself.
func UpdatingKinds(p *plan.Plan) []string {
	var out []string
	for _, u := range p.Artifact.ScheduledUpdates {
		out = append(out, u.Kind)
	}
	return out
}

func etcPaths(p *plan.Plan) []string {
	var out []string
	for _, f := range p.Artifact.Files {
		if strings.HasPrefix(f.Path, "/etc/") {
			out = append(out, f.Path)
		}
	}
	for _, f := range p.Artifact.GeneratedFiles {
		if strings.HasPrefix(f.Path, "/etc/") {
			out = append(out, f.Path)
		}
	}
	return out
}

func first(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return list[:n]
}

// RunnerOutput runs the update and returns what the tool wrote, so the caller
// decides whether a person needs to read it.
type RunnerOutput func(name string, args ...string) ([]byte, error)

// UpdateCommand is what an update runs: bootc upgrading the image this machine
// follows. Scan is not part of it, and neither is anything that writes to a home.
func UpdateCommand(apply bool) (string, []string) {
	args := []string{"upgrade"}
	if apply {
		args = append(args, "--apply")
	}
	return "bootc", args
}

// Blocking returns the conditions that refuse an update. Everything else is
// reported, and printing it is the caller's choice.
func Blocking(conditions []Condition) []Condition {
	var out []Condition
	for _, c := range conditions {
		if c.Blocks {
			out = append(out, c)
		}
	}
	return out
}

// Update runs the update when the guard allows it and returns the tool's own
// output. It writes nothing: a command that stages a deployment has one thing to
// say ("reboot to apply it"), and mixing a summary with the tool's prose produced
// a sentence that contradicted both.
func Update(conditions []Condition, run RunnerOutput, apply bool) ([]byte, error) {
	if blocked := Blocking(conditions); len(blocked) > 0 {
		c := blocked[0]
		return nil, fmt.Errorf("not updating: %s (%s)", c.Name, c.Detail)
	}
	name, args := UpdateCommand(apply)
	if run == nil {
		return nil, fmt.Errorf("no runner")
	}
	out, err := run(name, args...)
	if err != nil {
		return out, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out, nil
}
