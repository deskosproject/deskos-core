package software

import (
	"fmt"
	"path"
	"strings"

	"github.com/deskosproject/deskos-core/internal/compose"
	"github.com/deskosproject/deskos-core/internal/model"
	"github.com/deskosproject/deskos-core/internal/plan"
)

// PreinstallUnit runs the upstream `flatpak preinstall` at startup. The
// target platforms ship the preinstall mechanism but no unit invoking it.
//
// AuthenticatorUnit points OCI remotes at their authenticator once the remote
// exists. A .flatpakrepo file cannot carry one, so flatpak keeps it in the
// remote's runtime configuration and it has to be applied on the machine.
const (
	PreinstallUnit    = "deskos-flatpak-preinstall.service"
	AuthenticatorUnit = "deskos-flatpak-oci-authenticator.service"
)

// IconCachePackage lives on the platform, not here: the package that provides the
// icon cache tool is a platform fact, like the tool's command line.

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
		p.Artifact.PackageGroups = append(p.Artifact.PackageGroups, plan.PackageGroupInstall{
			Name: m.Name, Members: g.AllMembers(), ExcludePackages: g.ExcludePackages, Provenance: m.Provenance,
		})
		if g.Graphical {
			graphical = append(graphical, m.Provenance...)
		}
	}
	if len(graphical) > 0 {
		p.Artifact.DefaultTarget = &plan.DefaultTarget{Target: "graphical.target", Provenance: graphical}
		p.EnableUnit(pl.DisplayManager, graphical...)
	}

	// A negative package fact belongs to the platform, so its provenance is the
	// Platform resource rather than a profile. The backend removes these after
	// the installs and proves they are absent (docs/adr/0022).
	platformProv := []model.Provenance{{Resource: c.Platform.ID(), Source: c.Platform.Source.String()}}
	for _, dir := range pl.RepoConfigDirs {
		p.Artifact.RepoConfigDirs = append(p.Artifact.RepoConfigDirs, dir)
	}
	for _, name := range pl.RemovePackages {
		p.Artifact.RemovePackages = append(p.Artifact.RemovePackages, plan.PackageRemove{
			Name: name, Provenance: platformProv,
		})
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
		// A repository kind belongs to one package manager's world. Asking another
		// platform for it is a gap to report, not a file to copy where nothing reads
		// it (invariant 9).
		if pl.PackageManager != model.PackageManagerRpm {
			errs.Add(fmt.Errorf("an RPM repository requires an RPM platform: %s is %s\n  requested by: %s",
				c.Platform.ID(), pl.PackageManager, describe(k.Provenance)))
			continue
		}
		repo := k.Value.(Repository)
		keys := make([]plan.RepoKey, 0, len(repo.GPGKeys))
		for _, key := range repo.GPGKeys {
			keys = append(keys, plan.RepoKey{Name: key.Name, Content: key.Content, SHA256: key.SHA256})
		}
		p.Artifact.RpmRepositories = append(p.Artifact.RpmRepositories, plan.RpmRepository{
			ID: repo.ID, DisplayName: repo.DisplayName, BaseURL: repo.BaseURL,
			Enabled: repo.Enabled, GPGCheck: repo.GPGCheck, GPGKeys: keys,
			Provenance: k.Provenance,
		})
		if !repo.GPGCheck {
			p.Warnings = append(p.Warnings, fmt.Sprintf("RPM repository %q has GPG checking disabled", repo.ID))
		}
	}

	for _, k := range r.Keyed(DomainAptSources) {
		// Same rule as the repository kind above, from the other side: an APT
		// source is not a file to copy where nothing reads it (invariant 9).
		if pl.PackageManager != model.PackageManagerDeb {
			errs.Add(fmt.Errorf("an APT source requires an APT platform: %s is %s\n  requested by: %s",
				c.Platform.ID(), pl.PackageManager, describe(k.Provenance)))
			continue
		}
		src := k.Value.(AptSource)
		keys := make([]plan.RepoKey, 0, len(src.GPGKeys))
		for _, key := range src.GPGKeys {
			keys = append(keys, plan.RepoKey{Name: key.Name, Content: key.Content, SHA256: key.SHA256})
		}
		p.Artifact.AptRepositories = append(p.Artifact.AptRepositories, plan.AptRepository{
			ID: src.ID, DisplayName: src.DisplayName, BaseURL: src.BaseURL, Suite: src.Suite,
			Components: src.Components, Architectures: src.Architectures,
			GPGKeys: keys, Provenance: k.Provenance,
		})
	}

	for _, k := range r.Keyed(DomainRpmFiles) {
		if pl.PackageManager != model.PackageManagerRpm {
			errs.Add(fmt.Errorf("rpmFiles requires an RPM platform: %s is %s\n  requested by: %s",
				c.Platform.ID(), pl.PackageManager, describe(k.Provenance)))
			continue
		}
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

	for _, k := range r.Keyed(DomainDesktopEntries) {
		e := k.Value.(DesktopEntry)
		p.Artifact.DesktopEntries = append(p.Artifact.DesktopEntries, plan.DesktopEntry{
			ID: e.ID, Path: path.Join(ApplicationsDir, e.ID+".desktop"), Name: e.Name, Comment: e.Comment,
			Exec: e.Exec, Icon: e.ID, IconPath: e.IconPath, Categories: e.Categories,
			StartupWMClass: e.StartupWMClass, Provenance: k.Provenance,
		})
		p.Artifact.Files = append(p.Artifact.Files, plan.FileInstall{
			Path: e.IconPath, Mode: "0644", SHA256: e.IconSHA256, Asset: e.IconAsset, AssetFile: e.IconFile,
			Provenance: k.Provenance,
		})
		// Copying icons leaves the theme's cache stale and no package trigger runs
		// for files the build writes, so the cache is regenerated here. Which
		// package provides the tool and how it is invoked are the platform's facts
		// (ADR 0004): Core knows neither name, and a platform that carries no icon
		// cache tool reports the gap instead of shipping an entry with no icon.
		if pl.IconCache == nil {
			errs.Add(fmt.Errorf("a desktop entry with an icon needs an icon cache: %s declares no iconCache, so the icon would not appear\n  requested by: %s",
				c.Platform.ID(), describe(k.Provenance)))
			continue
		}
		p.Artifact.IconCaches = append(p.Artifact.IconCaches, plan.IconCacheUpdate{
			Dir: HicolorDir, Command: pl.IconCache.Command, Provenance: k.Provenance,
		})
		p.AddPackage(pl.IconCache.Package, k.Provenance...)
	}

	remotes := map[string]Remote{}
	remoteProv := map[string][]model.Provenance{}
	var flatpakProv []model.Provenance
	var authenticatorProv []model.Provenance
	for _, k := range r.Keyed(DomainFlatpakRemotes) {
		rm := k.Value.(Remote)
		remotes[rm.Name] = rm
		remoteProv[rm.Name] = k.Provenance
		flatpakProv = append(flatpakProv, k.Provenance...)
		if rm.Authenticator != "" {
			authenticatorProv = append(authenticatorProv, k.Provenance...)
		}
		p.Provisioning.FlatpakRemotes = append(p.Provisioning.FlatpakRemotes, plan.FlatpakRemote{
			Name: rm.Name, Title: rm.Title, Transport: rm.Transport, URL: rm.URL, Homepage: rm.Homepage,
			Comment: rm.Comment, Description: rm.Description, CollectionID: rm.CollectionID,
			Authenticator: rm.Authenticator, GPGKey: rm.GPGKey, GPGKeySHA256: rm.GPGKeySHA256,
			Provenance: k.Provenance,
		})
	}
	if len(authenticatorProv) > 0 {
		p.Provisioning.FlatpakAuthenticatorUnit = AuthenticatorUnit
		p.EnableUnit(AuthenticatorUnit, authenticatorProv...)
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
	// Retiring a remote is a negative operation, so the composition must not declare
	// the same remote: the two say the opposite about the same key. A `FlatpakSet`
	// always names a declared remote, so this covers it too (docs/adr/0017).
	if wf := c.WorkstationSpec.Flatpak; wf != nil {
		for _, rm := range wf.RemoveRemotes {
			prov := []model.Provenance{{
				Resource: c.Workstation.ID(), Layer: model.LayerWorkstation.String(), Source: c.Workstation.Source.String(),
			}}
			if declared, ok := remoteProv[rm.Name]; ok {
				errs.Add(fmt.Errorf("Flatpak remote %q is retired and declared by the same composition\n  retired by: %s\n  declared by: %s",
					rm.Name, rm.Reason, describe(declared)))
				continue
			}
			p.Provisioning.FlatpakRemovedRemotes = append(p.Provisioning.FlatpakRemovedRemotes,
				plan.FlatpakRemoval{Name: rm.Name, Reason: rm.Reason, Provenance: prov})
		}
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
