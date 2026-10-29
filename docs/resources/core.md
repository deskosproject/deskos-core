# Core resources

Part of the [resource model](../resources.md): the kinds that define what is built and where.

## `Workstation`

A concrete build target: one platform plus profiles. Profile order is not
precedence. It compiles into a workstation image, not a distribution edition.

| Field | Type | Required |
|---|---|---|
| `displayName`, `platformRef` | one-line string; resource name of one `Platform` | `platformRef` yes |
| `profiles` | names, at least one, unique; order is not precedence | yes |
| `flatpak.removeRemotes[]` | `{name, reason}`: a remote the base ships; the reason is one line | yes |

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: example-devops-centos-stream-10
spec:
  displayName: Example DevOps Workstation
  platformRef: centos-stream-10
  profiles:
    - deskos-core
    - deskos-gnome
    - example-organization
    - example-devops
```

`deskosctl plan ./examples/organization-and-role --workstation
example-devops-centos-stream-10` shows which layer won each setting.

A workstation can retire a Flatpak remote the base ships:
`removeRemotes: [{name: flathub, reason: apps come from the organization's own
remote}]`. Declaring the same remote in the composition is a contradiction and
fails; a `FlatpakSet` names its remote as a declared resource, so it is covered.
The removal is image-level, not `flatpak uninstall`: applications already installed
from that remote stay on an updated machine, and DeskOS never forces
([ADR 0017](../adr/0017-extensions-flatpak-remote-and-power-button.md)).

## `Profile`

Places resources at one semantic layer:

| Field | Type | Required |
|---|---|---|
| `layer`, `description`, `resources` | `foundation`, `organization`, `role`, `workstation`; one line; list of `{kind, name}` references, never copies | `layer`, `resources` yes |

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Profile
metadata:
  name: example-organization
spec:
  layer: organization
  description: Example organization baseline for every workstation.
  resources:
    - kind: PackageSet
      name: example-organization
    - kind: GnomeProfile
      name: example-desktop
```

## `Platform`

Facts about an OS target: bootc base image, package manager (`rpm` or `deb`),
package-group mapping, GNOME, boot, image update and Flatpak capabilities, whether
the base can fetch a verified artifact, and redistribution constraints. DeskOS
ships `centos-stream-10` and `rhel-10` in
[`resources/platforms/`](../../resources/platforms/), with status in
[platforms.md](../platforms.md). A platform still being evaluated carries its own
`Platform` resource in an example root.

| Field | Type | Required |
|---|---|---|
| `displayName`, `owner`, `tier` | one-line string; `vendor` or `organization` (`tier` needs `owner`) | `displayName` yes |
| `family`, `release`, `architectures`, `packageManager` | `^[a-z][a-z0-9-]*$`; `^[0-9A-Za-z][0-9A-Za-z.-]*$`; `[x86_64]`; `rpm` or `deb` | yes |
| `bootc.image`, `bootc.digest`, `distribution.redistributable`, `.requiresSubscription` | string; `sha256:<64 hex>`; boolean | image, `distribution` yes |
| `packageGroups[].name`, `.rpmGroups` / `.members` | group name; string list | yes; one of the two |
| `packageGroups[].excludePackages`, `.graphical` | package names; boolean | no |
| `displayManager`, `removePackages`, `unavailablePackages[]` | `*.service`; package names; `{name, reason}` | no |
| `gnome.dconfDatabase` / `gnome.keepsVendorDefaults` | name; boolean | one of the two |
| `gnome.loginScreen`, `gnome.loginScreenDatabase`, `gnome.appFolders`, `gnome.enabledExtensions`, `gnome.extensions[]`, `gnome.appLauncher`, `gnome.terminal` | boolean; name; boolean; string list; `{name, uuid, package}` or `{name, uuid, source{version, url, sha256}}`; `{package, command}`; `{package, dconfPath, fontKey, useSystemFontKey}` | `appLauncher`, `terminal` yes |
| `boot.splashKernelArgument`, `.quietKernelArgument`, `.plymouthPackages`, `.plymouthDracutModule`, `.spinnerFrames`, `updates.imageUpdateUnits`, `flatpak.package`, `flatpak.preinstall` | kernel arguments; package names; module; `{package, imageDir}`; `*.service`, `*.timer`; package name; boolean | arguments, plymouth, `updates`, `preinstall` yes |
| `trust.anchorsDir`, `.blocklistDir`, `.updateCommand`, `iconCache.package`, `.command`, `downloads.baseFetches`, `.package` | `/etc/...`; command; package name; command; boolean; package name | anchors, command, `iconCache`, `baseFetches` yes |

A package-group mapping names the platform's own members: `rpmGroups` for an RPM
platform, `members` for a deb one. `rpmFiles`, `RpmRepository` and `AptRepository`
each belong to one manager, and a platform with the other one fails at compile time
when either is requested.

A package or tool name belongs to a platform, never to Core: `flatpak.package`,
`trust.updateCommand`, `iconCache.package` with `iconCache.command`. A platform
that declares nothing where a name is needed reports the gap at compile time
(invariant 9).

A `BinaryArtifact`, or a package in `PackageSet.rpmFiles`, has to be fetched. `downloads.baseFetches` says
whether the base can fetch, and `downloads.package` names the fetcher where it
cannot; without the fact, composition fails naming the platform and the resource
([ADR 0019](../adr/0019-a-platform-says-whether-its-base-can-fetch.md)).

`gnome.keepsVendorDefaults: true` replaces `gnome.dconfDatabase` on a base with a
desktop of its own: DeskOS writes no GNOME default or lock there, and a profile
setting any GNOME value fails composition. `gnome.loginScreen: true` also needs
`gnome.loginScreenDatabase`, the database GDM reads (`gdm` on the EL family and
Fedora); the greeter reads its own dconf profile, so a key written beside the
desktop defaults never shows (#246).

`removePackages` differs from a group's `excludePackages`: an exclusion only stops
that transaction from installing a member, and cannot remove a package the base
carries. The build removes `removePackages` after every install transaction, skips
names that are not installed, does not autoremove dependencies, and fails if a
named package remains. RPM-only in v1alpha1
([ADR 0022](../adr/0022-removing-packages-a-platform-must-not-carry.md)).
