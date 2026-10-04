package software

import (
	"fmt"
	"strings"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
)

// PreinstallUnit runs the upstream `flatpak preinstall` at startup. The
// target platforms ship the preinstall mechanism but no unit invoking it.
const PreinstallUnit = "deskos-flatpak-preinstall.service"

// Lowerer translates composed software intent into Plan IR.
type Lowerer struct{}

func (Lowerer) Name() string { return "software" }

func (Lowerer) Lower(c *compose.Composition, p *plan.Plan) error {
	pl := c.PlatformSpec
	r := c.Result
	var errs model.ErrorList

	var graphical []model.Provenance
	for _, m := range r.Set(DomainPackageGroups) {
		g, ok := pl.PackageGroup(m.Name)
		if !ok {
			var known []string
			for _, pg := range pl.PackageGroups {
				known = append(known, pg.Name)
			}
			errs.Add(fmt.Errorf("package group %q is not defined by %s (defined: %s)\n  requested by: %s",
				m.Name, c.Platform.ID(), strings.Join(known, ", "), describe(m.Provenance)))
			continue
		}
		p.Artifact.RpmGroups = append(p.Artifact.RpmGroups, plan.RpmGroupInstall{
			Name: m.Name, RpmGroups: g.RpmGroups, ExcludePackages: g.ExcludePackages, Provenance: m.Provenance,
		})
		if g.Graphical {
			graphical = append(graphical, m.Provenance...)
		}
	}
	if len(graphical) > 0 {
		p.Artifact.DefaultTarget = &plan.DefaultTarget{Target: "graphical.target", Provenance: graphical}
		p.EnableUnit(pl.DisplayManager, graphical...)
	}

	for _, m := range r.Set(DomainPackages) {
		if reason, ok := pl.Unavailable(m.Name); ok {
			errs.Add(fmt.Errorf("package %q is not available on %s: %s\n  requested by: %s",
				m.Name, c.Platform.ID(), reason, describe(m.Provenance)))
			continue
		}
		p.AddPackage(m.Name, m.Provenance...)
	}
	for _, m := range r.Set(DomainUnits) {
		p.EnableUnit(m.Name, m.Provenance...)
	}

	for _, k := range r.Keyed(DomainRepositories) {
		repo := k.Value.(Repository)
		p.Artifact.RpmRepositories = append(p.Artifact.RpmRepositories, plan.RpmRepository{
			ID: repo.ID, DisplayName: repo.DisplayName, BaseURL: repo.BaseURL,
			Enabled: repo.Enabled, GPGCheck: repo.GPGCheck, GPGKeys: repo.GPGKeys,
			Provenance: k.Provenance,
		})
		if !repo.GPGCheck {
			p.Warnings = append(p.Warnings, fmt.Sprintf("RPM repository %q has GPG checking disabled", repo.ID))
		}
	}

	for _, k := range r.Keyed(DomainRpmFiles) {
		f := k.Value.(RpmFile)
		p.Artifact.RpmFiles = append(p.Artifact.RpmFiles, plan.RpmFileInstall{
			URL: f.URL, SHA256: f.SHA256, GPGKey: f.GPGKey, GPGKeySHA256: f.GPGKeySHA256,
			Provenance: k.Provenance,
		})
	}

	for _, k := range r.Keyed(DomainBinaries) {
		b := k.Value.(BinaryInstall)
		p.Artifact.Binaries = append(p.Artifact.Binaries, plan.VerifiedBinaryInstall{
			Artifact: b.Artifact, Version: b.Version, URL: b.URL, SHA256: b.SHA256,
			Archive: b.Archive, Member: b.Member, Destination: b.Destination, Mode: b.Mode,
			Provenance: k.Provenance,
		})
	}

	remotes := map[string]Remote{}
	var flatpakProv []model.Provenance
	for _, k := range r.Keyed(DomainFlatpakRemotes) {
		rm := k.Value.(Remote)
		remotes[rm.Name] = rm
		flatpakProv = append(flatpakProv, k.Provenance...)
		p.Provisioning.FlatpakRemotes = append(p.Provisioning.FlatpakRemotes, plan.FlatpakRemote{
			Name: rm.Name, Title: rm.Title, URL: rm.URL, Homepage: rm.Homepage, Comment: rm.Comment,
			Description: rm.Description, CollectionID: rm.CollectionID,
			GPGKey: rm.GPGKey, GPGKeySHA256: rm.GPGKeySHA256, Provenance: k.Provenance,
		})
	}
	apps := r.Keyed(DomainFlatpakApps)
	if len(apps) > 0 && !pl.Flatpak.Preinstall {
		errs.Add(fmt.Errorf("%s does not support Flatpak preinstallation; system Flatpak applications cannot be compiled for it\n  requested by: %s",
			c.Platform.ID(), describe(apps[0].Provenance)))
	}
	for _, k := range apps {
		a := k.Value.(App)
		flatpakProv = append(flatpakProv, k.Provenance...)
		p.Provisioning.FlatpakApplications = append(p.Provisioning.FlatpakApplications, plan.FlatpakPreinstall{
			ID: a.ID, Branch: a.Branch, Remote: a.Remote, CollectionID: remotes[a.Remote].CollectionID,
			Provenance: k.Provenance,
		})
	}
	if len(flatpakProv) > 0 && pl.Flatpak.Package != "" {
		p.AddPackage(pl.Flatpak.Package, flatpakProv...)
	}
	if len(apps) > 0 && pl.Flatpak.Preinstall {
		p.Provisioning.FlatpakPreinstallUnit = PreinstallUnit
		var appProv []model.Provenance
		for _, k := range apps {
			appProv = append(appProv, k.Provenance...)
		}
		p.EnableUnit(PreinstallUnit, appProv...)
	}
	return errs.Err()
}

func describe(ps []model.Provenance) string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Describe()+" ("+p.Source+")")
	}
	return strings.Join(out, "; ")
}
