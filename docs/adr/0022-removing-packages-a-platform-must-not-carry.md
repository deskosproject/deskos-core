# ADR 0022: Removing packages a platform must not carry

**Status:** accepted (2026-10-22)

## Context

Composition is additive, and `PackageGroup.excludePackages` only stops a member from being installed by **that group
transaction**; it cannot remove a package the base already carries. Measured on 2026-10-22: Fedora Silverblue 45
carries `gnome-software-51.0-1.fc45` and `gnome-software-rpm-ostree-51.0-1.fc45`, and excluding either from
`workstation-product-environment` leaves them. **Bazaar is DeskOS's graphical application store**:
`Profile/deskos-reference-apps` preinstalls the Flathub Flatpak `io.github.kolunmi.Bazaar`. DeskOS forbids shell hooks
(ADR 0005) and a `Profile` cannot express a removal (ADR 0003).

## Decision

1. **`Platform.spec.removePackages` names RPM packages the platform image must not carry.** The list is optional, its
   entries are validated package names, and it is distinct from a package group's `excludePackages`.
2. **The Plan carries removals as typed operations**: `artifact.removePackages[]` entries of `{name, provenance}`,
   sorted by name with duplicate provenance merged, where the provenance is the `Platform` resource. Removal adds no
   package to the declared SBOM.
3. **The backend removes after the installs, without force.** Group, package and RPM-file transactions run first; the
   build checks each name with `rpm -q` and skips names that are not installed; installed names go to `dnf -y remove
   --noautoremove`. The backend does not use `--nodeps`, `--allowerasing`, `--force` or an ignore-broken transaction:
   a removal that cannot complete fails the build. It does not autoremove either: the first AlmaLinux 10 run removed
   `flatpak` as an unused dependency of GNOME Software, and the next boot failed because the preinstall unit had no
   `/usr/bin/flatpak`. After the transaction `rpm -q` is checked again, and the build fails if a named package is
   still installed.
4. **Removal and explicit installation are a contradiction.** If the composition installs a package the platform
   declares it must not carry, composition fails and names both decisions; a package group that happens to contain it
   is not a contradiction.
5. **This is an RPM-platform operation in v1alpha1.** A deb platform that declares `removePackages` fails composition,
   because APT removal has different semantics and no tested path.
6. **GNOME Software is removed on every shipped platform.** CentOS Stream 10, AlmaLinux 10, Fedora 45 and RHEL 10
   declare `removePackages: [gnome-software, gnome-software-rpm-ostree]`; Bazaar remains the Flatpak delivered by
   `Profile/deskos-reference-apps`, not an RPM.

## Consequences

- A platform states a negative package fact once, and every workstation built on it gets the same image-level
  guarantee, without a copied build script.
- On Fedora, both GNOME Software packages are removed before the image is committed; on EL10 the same declaration is a
  no-op if the base or group did not install them, and the post-removal check still proves the result.
- The removal is image-level, not a data migration: it does not uninstall applications a user installed from a remote,
  edit a user's `$HOME` or change the update source.
- Removing base packages can remove dependencies. A desktop that needs a removed package fails the build rather than
  shipping half-functional, and the platform's boot and session checks prove it still works.
- Removal without autoremove can leave dependencies no longer needed. That is deliberate: a dependency may be
  explicitly installed by the composition, as `flatpak` is; the installed SBOM records what remains.
- The Plan stays deterministic and model-free; the declared SBOM remains a list of installs, and the installed SBOM is
  the authoritative record.
