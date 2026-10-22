# ADR 0022: Removing packages a platform must not carry

**Status:** accepted (2026-10-22)

## Context

Composition is additive: a `PackageSet` installs packages and groups, and a
`Platform` maps a DeskOS group to the platform's own group. `PackageGroup.excludePackages`
prevents one member from being installed by **that group transaction**, but it
cannot remove a package that the base image already carries.

Measured on 2026-10-22:

- Fedora Silverblue 45's base carries `gnome-software-51.0-1.fc45` and
  `gnome-software-rpm-ostree-51.0-1.fc45`. Excluding either from
  `workstation-product-environment` leaves the installed base package in place.
- The CentOS Stream 10, AlmaLinux 10 and RHEL 10 bootc bases do not carry
  `gnome-software` themselves, but their workstation environment may install it.

DeskOS has decided that **Bazaar is its graphical application store**. Every
shipped reference workstation already selects `Profile/deskos-reference-apps`,
which preinstalls the Flathub Flatpak `io.github.kolunmi.Bazaar`. Keeping GNOME
Software as a second store is inconsistent on every platform, not only Fedora.

Bluefin solves the same problem with a build-script exclusion list and
`dnf5 -y remove` after its installs. DeskOS forbids shell hooks and
YAML-to-Containerfile escape hatches (ADR 0005), so the equivalent has to be a
typed resource fact and a typed Plan operation, not a script embedded in a
resource.

The fourteen public kinds do not need to grow: this is a platform fact, not a
new resource kind. A `Profile` cannot express it either, because profiles add
installation intent and composition has no subtraction (ADR 0003).

## Decision

1. **`Platform.spec.removePackages` names RPM packages the platform image must
   not carry.** The list is a platform-owned negative fact. It is optional, and
   its entries are validated package names. It is distinct from a package group's
   `excludePackages`, which only changes what that group installs.

2. **The Plan carries removals as typed operations.** A parsed platform appends
   `artifact.removePackages[]` entries of `{name, provenance}`. The provenance is
   the `Platform` resource. The Plan stays canonical: entries are sorted by name
   and duplicate provenance is merged. Removal adds no package to the declared
   SBOM; the installed SBOM describes what the built image actually contains.

3. **The backend removes after the installs, without force.**
   - Group, package and RPM-file transactions run first, because an exclusion
     that only applied before them could be undone by a later transaction.
   - The build checks each name with `rpm -q`; names that are not installed are
     skipped, not treated as an error. This is required for a package that is
     present on one vendor base and absent on another.
   - Installed names are passed to `dnf -y remove --noautoremove`. The backend
     does not use `--nodeps`, `--allowerasing`, `--force` or an ignore-broken
     transaction: a removal that cannot complete fails the build. It also does
     not autoremove dependencies: the first AlmaLinux 10 run removed `flatpak`
     as an unused dependency of GNOME Software, and the next boot failed because
     the image's Flatpak preinstall unit had no `/usr/bin/flatpak`.
   - After the transaction the build checks `rpm -q` again and fails if a named
     package is still installed. DNF's exit status alone is not the evidence
     that the image is clean.

4. **Removal and explicit installation are a contradiction.** If the composition
   explicitly installs a package that the platform declares it must not carry,
   composition fails and names both decisions. A package group that happens to
   contain the package is not a contradiction: the platform removed it for the
   image after the group transaction.

5. **This is an RPM-platform operation in v1alpha1.** A deb platform that
   declares `removePackages` fails composition. APT removal has different
   semantics and has no tested DeskOS path; silently translating it would be
   worse than reporting the gap.

6. **GNOME Software is removed on every shipped platform.** The CentOS Stream 10,
   AlmaLinux 10, Fedora 45 and RHEL 10 resources declare:
   `removePackages: [gnome-software, gnome-software-rpm-ostree]`. Bazaar remains
   the Flatpak delivered by `Profile/deskos-reference-apps`, not an RPM.

## Consequences

- A platform can state a negative package fact once, and every workstation built
  on it gets the same image-level guarantee. Organizations do not copy a build
  script or a list of shell commands.
- On Fedora, the two GNOME Software packages that the Silverblue base carries are
  removed before the image is committed. On EL10, the same declaration is a no-op
  if the base or group did not install them; the post-removal check still proves
  the result.
- The removal is image-level, not a data migration. It does not uninstall
  applications a user installed from a remote, edit a user's `$HOME`, or change
  the update source. An updated machine receives an image without GNOME Software;
  user and `/var` state are outside this operation.
- Removing base packages can remove dependencies. A desktop that needs a removed
  package fails the build rather than shipping a half-functional image. The
  affected platform's boot and session checks are the evidence that the image is
  still usable; a unit test alone cannot prove a desktop.
- Removal without autoremove can leave dependencies that are no longer needed.
  That is deliberate: a dependency may be explicitly installed by the
  composition (as `flatpak` is) and removing it would break the image. The
  installed SBOM records what remains; cleanup of genuinely unused dependencies
  is a separate, tested concern.
- The declared SBOM remains a list of installs. The installed SBOM is the
  authoritative record of what is present, and it will no longer list the removed
  packages.
- The Plan is still deterministic and model-free. The package names are typed
  input; repository content and the final image are not claimed to be
  reproducible.
- The next time a platform needs a negative package fact — an unwanted base
  service's package, a conflicting vendor store, a package that must be replaced
  by a Flatpak — it uses this field instead of inventing a `postInstall` or a
  platform-specific backend branch.
