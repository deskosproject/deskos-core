# Core resources

Part of the [resource model](../resources.md): the kinds that define what is built and where.

## `Workstation`

A concrete build target: **one platform plus profiles**. Profile order is
*not* precedence. It compiles into a workstation **image**; `Workstation`
is the composition kind — the machine you author — not a distribution edition.

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
example-devops-centos-stream-10` shows the composed result, including **which
layer won each setting**.

A workstation can also **retire a Flatpak remote the base ships**, so a machine
whose owner does not want that application source configured does not have it:

```yaml
spec:
  flatpak:
    removeRemotes:
      - name: flathub
        reason: apps come from the organization's own remote
```

The reason is mandatory. A remote the composition also declares is a contradiction
and fails; a `FlatpakSet` names its remote as a declared resource, so it is covered.
The removal is image-level: it is **not** `flatpak uninstall`, applications already
installed from that remote stay on an updated machine, and DeskOS never forces
([ADR 0017](../adr/0017-extensions-flatpak-remote-and-power-button.md)).

## `Profile`

Places resources at **one semantic layer**:

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

Facts about an OS target: its bootc base image, **its package manager** (`rpm` or
`deb`), its package-group mapping, GNOME, boot, image update and Flatpak
capabilities, **whether its base can fetch a verified artifact**, and its
redistribution constraints. DeskOS ships `centos-stream-10`,
`rhel-10` in
[`resources/platforms/`](../../resources/platforms/); the canonical status of each,
and how to install it, is [platforms.md](../platforms.md). A platform that is only
being *evaluated* is not shipped: it carries its own `Platform` resource inside
an example root instead.

A package-group mapping names the platform's own members: `rpmGroups` for an RPM
platform, `members` (its metapackage names) for a deb one. A platform declares one
or the other, according to its package manager. `rpmFiles`, `RpmRepository` and
`AptRepository` are each one manager's: a platform whose manager is not that one
fails at compile time when any of them is requested, instead of rendering nonsense
or copying files where nothing reads them.

**A package or tool name belongs to a platform, never to Core.** When Core needs a
program the platform may spell differently, the platform declares it: `flatpak.package`
for the application channel, `trust.updateCommand` for the trust store, and
`iconCache.package` with `iconCache.command` for the tool that refreshes an icon
theme cache. Core carries no name of its own and no table translating one platform's
name into another's — a table has to be maintained per platform pair, and it gets
worse with every platform added. A platform that declares nothing where a name is
needed reports the gap at compile time (invariant 9): a desktop entry with an icon
on a platform with no `iconCache` fails composition, rather than shipping an icon
that never appears.

The same rule covers the one capability a build cannot proceed without: a
`BinaryArtifact` or an `RpmFile` has to be *fetched*, so `downloads.baseFetches`
says whether the base can fetch, and `downloads.package` names the fetcher on a
base that cannot. A plan that downloads from a platform that declares nothing
fails composition with the platform and the resource named, and a base without a
fetcher gets that package added to the plan, carrying the provenance of the
download that needed it ([ADR 0019](../adr/0019-a-platform-says-whether-its-base-can-fetch.md)).
What the backend fetches *with* stays Core's own choice: the platform states that a
base can satisfy it, not a command line to run.

A platform can also declare **packages the image must not carry**:

```yaml
spec:
  removePackages: [gnome-software, gnome-software-rpm-ostree]
```

This is different from a package group's `excludePackages`: an exclusion only
prevents that group transaction from installing a member, and cannot remove a
package the base already carries. The build removes `removePackages` after every
install transaction, skips names that are not installed, does not autoremove
dependencies, and fails if a named package is still present. A composition that
explicitly installs one of these packages is a contradiction and fails. The
operation is RPM-only in v1alpha1
([ADR 0022](../adr/0022-removing-packages-a-platform-must-not-carry.md)).
