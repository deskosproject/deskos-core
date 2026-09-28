// Package textplan renders a Plan for humans.
package textplan

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
)

// Write prints the effective plan.
func Write(out io.Writer, p *plan.Plan) error {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	pf := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }

	pf("WORKSTATION\n")
	if p.Workstation.DisplayName != "" {
		pf("  %s (%s)\n", p.Workstation.Name, p.Workstation.DisplayName)
	} else {
		pf("  %s\n", p.Workstation.Name)
	}

	pf("\nPLATFORM\n  %s (%s)\n", p.Platform.DisplayName, p.Platform.Name)
	bi := p.Artifact.BaseImage
	pin := "not pinned by digest"
	if bi.Digest != "" {
		pin = "pinned " + bi.Digest
	}
	pf("  base image: %s (%s)\n", bi.Image, pin)
	if p.Platform.Redistributable {
		pf("  publication: redistributable\n")
	} else {
		pf("  publication: private, must not be publicly redistributed\n")
	}
	if p.Platform.RequiresSubscription {
		pf("  build host: requires a subscription (factory credential, not part of resources)\n")
	}

	pf("\nPROFILES\n")
	for _, pr := range p.Profiles {
		pf("  %s\t%s\n", pr.Layer+":", pr.Name)
	}

	a := p.Artifact
	pf("\nARTIFACT\n")
	section(w, "RPM repositories", len(a.RpmRepositories))
	for _, r := range a.RpmRepositories {
		state := ""
		if !r.Enabled {
			state = " (disabled after build)"
		}
		if !r.GPGCheck {
			state += " (GPG check disabled)"
		}
		pf("    %s\t%s%s\t%s\n", r.ID, r.BaseURL, state, origins(r.Provenance))
	}
	section(w, "RPM groups", len(a.RpmGroups))
	for _, g := range a.RpmGroups {
		excl := ""
		if len(g.ExcludePackages) > 0 {
			excl = " (without " + strings.Join(g.ExcludePackages, ", ") + ")"
		}
		pf("    %s\t-> %s%s\t%s\n", g.Name, strings.Join(g.RpmGroups, ", "), excl, origins(g.Provenance))
	}
	section(w, "RPM packages", len(a.RpmPackages))
	for _, pkg := range a.RpmPackages {
		pf("    %s\t%s\n", pkg.Name, origins(pkg.Provenance))
	}
	section(w, "Binary artifacts", len(a.Binaries))
	for _, b := range a.Binaries {
		pf("    %s\t%s %s, sha256 %s…\t%s\n", b.Destination, b.Artifact, b.Version, b.SHA256[:12], origins(b.Provenance))
	}
	section(w, "Files", len(a.Files))
	for _, f := range a.Files {
		pf("    %s\tfrom %s\t%s\n", f.Path, f.Asset, origins(f.Provenance))
	}
	if db := a.Dconf; db != nil {
		section(w, "GNOME defaults (dconf system database "+db.Name+")", len(db.Defaults))
		for _, d := range db.Defaults {
			pf("    %s\t%s = %s\t%s\n", d.Setting, d.Key, d.Value, origins(d.Provenance))
		}
		section(w, "GNOME locks", len(db.Locks))
		for _, l := range db.Locks {
			pf("    %s\t%s\t%s\n", l.Setting, l.Key, origins(l.Provenance))
		}
	} else {
		section(w, "GNOME defaults", 0)
	}
	section(w, "GSettings vendor defaults (for sessions outside the DeskOS dconf profile)", len(a.GSettings))
	for _, d := range a.GSettings {
		pf("    %s\t%s %s = %s\t%s\n", d.Setting, d.Schema, d.Key, d.Value, origins(d.Provenance))
	}
	section(w, "Kernel arguments (bootc kargs.d)", len(a.KernelArguments))
	for _, k := range a.KernelArguments {
		pf("    %s\t%s\t%s\n", k.Arg, k.Setting, origins(k.Provenance))
	}
	if ir := a.Initramfs; ir != nil {
		pf("  Initramfs\n    rebuilt with dracut modules: %s\t%s\n", strings.Join(ir.DracutModules, ", "), origins(ir.Provenance))
	}
	if t := a.PlymouthTheme; t != nil {
		pf("  Boot splash theme (default)\n    %s\t%s, frames from %s, watermark %s\t%s\n", t.Name, t.Dir, t.FramesFrom, t.Watermark, origins(t.Provenance))
	}
	section(w, "systemd units enabled", len(a.SystemdUnits))
	for _, u := range a.SystemdUnits {
		pf("    %s\t%s\n", u.Unit, origins(u.Provenance))
	}
	if a.DefaultTarget != nil {
		pf("  Default target\n    %s\n", a.DefaultTarget.Target)
	}

	pr := p.Provisioning
	pf("\nPROVISIONING\n")
	if len(pr.FlatpakRemotes) == 0 && len(pr.FlatpakApplications) == 0 {
		pf("  none\n")
	} else {
		section(w, "Flatpak remotes (system)", len(pr.FlatpakRemotes))
		for _, r := range pr.FlatpakRemotes {
			pf("    %s\t%s\t%s\n", r.Name, r.URL, origins(r.Provenance))
		}
		section(w, "Flatpak applications (preinstalled at startup)", len(pr.FlatpakApplications))
		for _, app := range pr.FlatpakApplications {
			pf("    %s//%s\tfrom %s\t%s\n", app.ID, app.Branch, app.Remote, origins(app.Provenance))
		}
	}

	pf("\nENROLLMENT\n  none\n")

	if len(p.Warnings) > 0 {
		pf("\nWARNINGS\n")
		for _, warn := range p.Warnings {
			pf("  %s\n", warn)
		}
	}
	return w.Flush()
}

func section(w io.Writer, title string, n int) {
	if n == 0 {
		fmt.Fprintf(w, "  %s\n    none\n", title)
		return
	}
	fmt.Fprintf(w, "  %s\n", title)
}

// origins summarizes provenance as "layer Kind/name" entries.
func origins(ps []model.Provenance) string {
	seen := map[string]bool{}
	var out []string
	for _, p := range ps {
		s := p.Layer + " " + p.Resource
		if p.Layer == "" {
			s = p.Resource
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return "[" + strings.Join(out, ", ") + "]"
}
