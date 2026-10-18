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
		pf("Updates:     the image updates itself (%s)\n", strings.Join(kinds, ", "))
	}

	if present != nil {
		pf("\nBooted deployment  [present]\n")
		if !present.IsBootc {
			pf("  this machine is not a bootc deployment: nothing here can be read from one\n")
		} else {
			if present.Image != "" {
				pf("  image:  %s\n", present.Image)
			}
			if present.Digest != "" {
				pf("  digest: %s\n", present.Digest)
			}
		}
	}

	// The distinction Codex asked for, said plainly: the plan is installed, the SBOM
	// is not, and a report may not describe installed content from the declared one.
	pf("\nEvidence\n")
	pf("  plan: installed on this machine (%s), and this report reads it\n", PlanPath)
	pf("  SBOM: declared by the build pipeline and shipped with the release, NOT installed on\n")
	pf("        this machine (ADR 0010 stage 4 is planned), so this report does not describe it\n")
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
