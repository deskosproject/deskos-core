// Package evidence is what the runtime binary reads from the machine it runs on: the
// plan the image carries, the state it can compare against that plan, and the
// conditions that decide whether an update may be asked for.
//
// Two promises live here, and they are different on purpose (docs/adr/0018):
//
//   - Report **describes** the artifact: what the machine was built from, as the plan
//     the image carries declares it. It says what is installed and what is only
//     declared elsewhere (the SBOM travels with the release, not with the image).
//   - Check **compares** a declared scope with the present state and enumerates the
//     differences, publishing which paths and states it looks at and which it leaves
//     out. It never claims to measure "all drift": it measures its scope.
//
// Nothing here writes to a user's home, reconciles the machine, or removes anything.
package evidence

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/deskosproject/deskos-core/internal/plan"
)

// PlanPath is where the compiler's plan lives inside the image. The image carries it
// (internal/backends/containerfile), so the plan is installed evidence, unlike the
// SBOM, which is CI evidence (ADR 0010).
const PlanPath = "/usr/share/deskos/plan.json"

// Report writes what the machine was built from. `present` is what could be learned
// about the running deployment right now, or nil when that is unknown.
func Report(w io.Writer, p *plan.Plan, present *Booted) {
	a := p.Artifact
	pf := func(format string, args ...any) { fmt.Fprintf(w, format, args...) }

	pf("Workstation: %s", p.Workstation.Name)
	if p.Workstation.DisplayName != "" {
		pf(" (%s)", p.Workstation.DisplayName)
	}
	pf("\nPlatform:    %s", p.Platform.Name)
	pf("\nBase image:  %s", a.BaseImage.Image) // declared
	if a.BaseImage.Digest != "" {
		pf(" @ %s", a.BaseImage.Digest)
	}
	pf("  [declared]\n")
	pf("Profiles:    %s\n", strings.Join(profileNames(p), ", "))

	pf("Packages:    %d", len(a.Packages))
	if len(a.PackageGroups) > 0 {
		pf(" (in %d package group(s))", len(a.PackageGroups))
	}
	pf("\nRT software: %d RPM repositories, %d apt sources, %d binaries, %d files\n",
		len(a.RpmRepositories), len(a.AptRepositories), len(a.Binaries), len(a.Files))

	if db := a.Dconf; db != nil {
		pf("GNOME:       %d dconf default(s) in the %q database, %d lock(s)\n",
			len(db.Defaults), db.Name, len(db.Locks))
		if len(db.Locks) > 0 {
			pf("Locks:       %d declared\n", len(db.Locks))
			for _, lock := range db.Locks {
				pf("  %s (%s)\n", lock.Key, lock.Setting)
			}
			pf("  declared in the image; whether the running session enforces them is not\n")
			pf("  measured here. The instrumented session check writes each key and expects\n")
			pf("  the write to fail.\n")
		}
	}
	pf("Extensions:  %s\n", enabledExtensions(a.EnabledExtensions))
	if len(a.KernelArguments) > 0 {
		var args []string
		for _, k := range a.KernelArguments {
			args = append(args, k.Arg)
		}
		pf("Boot args:   %s\n", strings.Join(args, " "))
	}
	if kinds := updatingKinds(p); len(kinds) > 0 {
		pf("Updates:     the plan schedules %s updates for the image itself (declared, not that a timer is on)\n",
			strings.Join(kinds, ", "))
	}

	if present != nil {
		pf("\nDeployment  [present, read from bootc]\n")
		if !present.IsBootc {
			pf("  this machine did not boot from a bootc deployment: there is no deployment to read\n")
		} else {
			if present.Image != "" {
				pf("  booted:  %s\n", present.Image)
			}
			if present.Digest != "" {
				pf("  digest:  %s\n", present.Digest)
			}
			if present.Version != "" {
				pf("  version: %s\n", present.Version)
			}
			if present.Follows != "" {
				pf("  updates from: %s\n", present.Follows)
			}
			if present.StagedImage != "" {
				pf("  staged, waiting for a reboot: %s @ %s\n", present.StagedImage, present.StagedDigest)
			} else {
				pf("  nothing staged\n")
			}
		}
	}

	// The distinction Codex asked for, said plainly: the plan is installed, the SBOM
	// is not, and a report may not describe installed content from the declared one.
	pf("\nEvidence\n")
	pf("  plan: installed on this machine (%s), and this report reads it\n", PlanPath)
	pf("  SBOM: built by the pipeline as CI evidence (attached to the workflow run, not\n")
	pf("        shipped with the release) and NOT installed on this machine (ADR 0010\n")
	pf("        stage 4 is planned), so this report does not describe it\n")
}

func profileNames(p *plan.Plan) []string {
	var out []string
	for _, pr := range p.Profiles {
		out = append(out, pr.Name)
	}
	sort.Strings(out)
	return out
}

func enabledExtensions(list []string) string {
	if len(list) == 0 {
		return "none decided by this composition (the platform's own set stands)"
	}
	return strings.Join(list, ", ")
}
