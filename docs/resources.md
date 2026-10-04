# Resources

The DeskOS resource model: every kind with a real example from this
repository, how layers compose, and the software and GNOME model behind
them.

## Envelope

DeskOS configuration uses **versioned, Kubernetes-inspired resource
envelopes**. DeskOS borrows the useful API ideas (Group, Version, Kind,
schemas and composition) *without requiring Kubernetes*.

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: acme-developer-centos10
spec:
  platformRef: centos-stream-10
  profiles:
    - deskos-core
    - acme-baseline
    - acme-developer
```

| Group | Kinds |
|---|---|
| `core.deskos.org` | `Platform`, `Profile`, `Workstation` |
| `software.deskos.org` | `PackageSet`, `RpmRepository`, `BinaryArtifact`, `FlatpakRemote`, `FlatpakSet` |
| `desktop.deskos.org` | `GnomeProfile` |
| `system.deskos.org` | `BootProfile` |

All kinds are `v1alpha1`. Their JSON Schemas are in
[`schemas/`](../schemas/).

## Resource roots

Resources live in YAML files under **one or more resource roots**. DeskOS
ships `./resources`; an organization keeps its own root and references
DeskOS resources by kind and name **without copying them**.

- File and directory names have no meaning.
- Asset paths are relative to the YAML file.
- Every example below is a file from this repository
  ([`resources/`](../resources/) and
  [`examples/example-org/`](../examples/example-org/)).

## Resource kinds

### `Workstation`

A concrete build target: **one platform plus profiles**. Profile order is
*not* precedence.

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata:
  name: example-devops-centos10
spec:
  displayName: Example Org DevOps Workstation
  platformRef: centos-stream-10
  profiles:
    - deskos-core
    - example-baseline
    - example-devops
```

`deskosctl plan ./resources ./examples/example-org --workstation
example-devops-centos10` shows the composed result, including **which
layer won each setting**.

### `Profile`

Places resources at **one semantic layer**:

```yaml
apiVersion: core.deskos.org/v1alpha1
kind: Profile
metadata:
  name: example-baseline
spec:
  layer: organization
  description: Example organization baseline for every workstation.
  resources:
    - kind: PackageSet
      name: example-baseline
    - kind: GnomeProfile
      name: example-desktop
```

### `Platform`

Facts about an OS target (bootc base image, RPM groups, GNOME and Flatpak
capabilities, redistribution). DeskOS ships `centos-stream-10` and
`rhel-10` in [`resources/platforms/`](../resources/platforms/); see
[architecture.md](architecture.md#core-organizations-and-roles).

### `PackageSet`

Packages, RPM groups (named by the Platform) and units to enable:

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: PackageSet
metadata:
  name: deskos-core
spec:
  groups:
    - workstation
  packages:
    - bootc
    - NetworkManager
    - firefox
    - flatpak
    - firewalld
    - xdg-utils
  enableUnits:
    - firewalld.service
```

`rpmFiles` installs **signed RPMs that the vendor publishes without a
repository**. Each entry needs an https URL that is not a floating
location, the file's SHA-256 and `gpgKeyFile`, the vendor's ASCII-armored
public key as a file inside the resource root, relative to the YAML file.
The build fails unless the checksum matches and the RPM is signed by that
key. Identical entries from several resources deduplicate; the same URL
with another checksum or key is a conflict at any layer.

An example for Zoom (organization content, not part of DeskOS; the
organization provides `keys/zoom.asc`, here the key from
`https://zoom.us/linux/download/pubkey?version=6-7-5`, fingerprint
`84C3 65D6 CC9A 4886 CA92 6BCC 4F21 9739 9706 AC24`):

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: PackageSet
metadata:
  name: zoom
spec:
  rpmFiles:
    - url: https://zoom.us/client/7.2.1.5760/zoom_x86_64.rpm
      sha256: 79b6fc1ffd9fd2e2d136e898aed9c8ed6ab672a83841de4220ca4c14005d76fd
      gpgKeyFile: keys/zoom.asc
```

### `RpmRepository`

An **official vendor repository**:

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: RpmRepository
metadata:
  name: vscode
spec:
  id: code
  displayName: Visual Studio Code
  baseURL: https://packages.microsoft.com/yumrepos/vscode
  gpgKeys:
    - https://packages.microsoft.com/keys/microsoft.asc
```

### `BinaryArtifact`

A verified upstream binary, **pinned by version and SHA-256**:

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: BinaryArtifact
metadata:
  name: openshift-client
spec:
  version: 4.22.14
  source:
    url: https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.22.14/openshift-client-linux-amd64-rhel9-4.22.14.tar.gz
    sha256: 73d4204fe2d028a5fb3b05f71da174915442c445d3b417321c635bf17d099f6b
  archive: tar.gz
  files:
    - path: oc
      destination: /usr/local/bin/oc
      mode: "0755"
```

### `FlatpakRemote` and `FlatpakSet`

A **system** Flatpak remote, and applications preinstalled from it:

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: FlatpakRemote
metadata:
  name: flathub
spec:
  title: Flathub
  url: https://dl.flathub.org/repo/
  collectionID: org.flathub.Stable
  gpgKeyFile: keys/flathub.gpg
```

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: FlatpakSet
metadata:
  name: deskos-reference-apps
spec:
  remote: flathub
  applications:
    - id: io.github.kolunmi.Bazaar
      branch: stable
```

### `GnomeProfile`

**GNOME intent** in administrator vocabulary. An organization layer
overrides the Core defaults it names:

```yaml
apiVersion: desktop.deskos.org/v1alpha1
kind: GnomeProfile
metadata:
  name: example-desktop
spec:
  defaults:
    windows:
      buttons: [close]
    appearance:
      wallpaper:
        light: ../assets/example-org.svg
      loginLogo: ../assets/example-org-login-logo.svg
    session:
      idle:
        blankAfter: 5m
      lock:
        enabled: true
        delay: 0s
    dock:
      enabled: true
      position: bottom
      behavior: intellihide
      iconSize: 40
      showTrash: false
```

### `BootProfile`

**Boot appearance**: graphical splash, quiet boot and the splash
watermark.

```yaml
apiVersion: system.deskos.org/v1alpha1
kind: BootProfile
metadata:
  name: deskos-core
spec:
  splash: graphical
  quiet: true
  watermark: ../assets/deskos/deskos-splash-watermark.png
```

## Composition

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="images/deskos-composition-dark.svg">
    <img alt="DeskOS composition: layers foundation, organization, role and workstation with explicit precedence; set, keyed and scalar composition classes; example where the organization's 5m screen blank overrides Core's 10m; the Plan keeps the provenance of every value" src="images/deskos-composition-light.svg" width="900">
  </picture>
</p>

A workstation is composed from **explicit semantic layers**:
`foundation` < `organization` < `role` < `workstation`.

- **DeskOS Core provides defaults**; organizations *intentionally*
  override them.
- **Conflicting settings at the same layer are errors.**
- **File order is never precedence.**

The full rules (set, keyed and scalar classes, provenance) are in
[architecture.md](architecture.md#composition).

## DeskOS Core

**DeskOS Core** is the reusable workstation foundation maintained by the
project.

- The public reference workstation uses **CentOS Stream 10**.
- Organizations that require **RHEL** compile the same Core semantics
  directly onto the official RHEL 10 bootc base.
- The RHEL artifact is **not derived** from the CentOS image.

## Software model

DeskOS deliberately **avoids becoming a universal package manager**. The
managed baseline uses the delivery mechanism appropriate to the
software, in this order of preference:

1. distribution RPM;
2. official vendor RPM, from its repository or, when the vendor publishes
   none, as a signed RPM file;
3. verified upstream binary;
4. system Flatpak;
5. future managed web applications.

User and project environments (mise, SDKMAN, Homebrew, language version
managers, dotfiles, personal Toolboxes) are **outside the DeskOS managed
baseline**.

## GNOME

DeskOS exposes **administrator intent**, not dconf implementation details.
An administrator describes things such as:

- window controls;
- wallpapers and branding;
- fonts;
- icon and cursor themes;
- ordered favorites;
- dock behavior;
- idle timeout;
- lock behavior;

*without knowing* which GNOME schema or dconf key implements them. DeskOS
Core ships reasonable defaults, and organizations override them
declaratively.
