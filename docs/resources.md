# Resources

DeskOS configuration uses versioned, Kubernetes-inspired resource
envelopes:

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

DeskOS borrows the useful API ideas (Group, Version, Kind, schemas and
composition) without requiring Kubernetes.

The v1alpha1 kinds are `Platform`, `Profile` and `Workstation`
(`core.deskos.org`); `PackageSet`, `RpmRepository`, `BinaryArtifact`,
`FlatpakRemote` and `FlatpakSet` (`software.deskos.org`);
`GnomeProfile` (`desktop.deskos.org`); and `BootProfile`
(`system.deskos.org`). Their JSON Schemas are in
[`schemas/`](../schemas/).

## Examples

Resources live in YAML files under one or more resource roots. DeskOS
ships `./resources`; an organization keeps its own root and references
DeskOS resources by kind and name without copying them. File and directory
names have no meaning, and asset paths are relative to the YAML file. All
examples below are files from this repository
([`resources/`](../resources/) and [`examples/example-org/`](../examples/example-org/)).

A Profile places resources at one semantic layer:

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

Packages, RPM groups (named by the Platform) and units to enable:

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

An official vendor repository:

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

A verified upstream binary, pinned by version and SHA-256:

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

A system Flatpak remote and applications preinstalled from it:

    apiVersion: software.deskos.org/v1alpha1
    kind: FlatpakRemote
    metadata:
      name: flathub
    spec:
      title: Flathub
      url: https://dl.flathub.org/repo/
      collectionID: org.flathub.Stable
      gpgKeyFile: keys/flathub.gpg
    ---
    apiVersion: software.deskos.org/v1alpha1
    kind: FlatpakSet
    metadata:
      name: deskos-reference-apps
    spec:
      remote: flathub
      applications:
        - id: io.github.kolunmi.Bazaar
          branch: stable

GNOME intent; an organization layer overrides the Core defaults it
names:

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

Boot appearance:

    apiVersion: system.deskos.org/v1alpha1
    kind: BootProfile
    metadata:
      name: deskos-core
    spec:
      splash: graphical
      quiet: true
      watermark: ../assets/deskos/deskos-splash-watermark.png

The organization's workstation then composes Core, its baseline and a
role on a platform; profile order is not precedence:

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

`deskosctl plan ./resources ./examples/example-org --workstation
example-devops-centos10` shows the composed result, including which layer
won each setting.

## Composition

A workstation is composed from explicit semantic layers:

    foundation
        <
    organization
        <
    role
        <
    workstation

DeskOS Core provides defaults.

Organizations intentionally override them.

Conflicting settings at the same semantic layer are errors.

File order is never used as an implicit precedence mechanism.

## DeskOS Core

DeskOS Core is the reusable workstation foundation maintained by the
project.

The public reference workstation uses CentOS Stream 10.

Organizations that require RHEL can compile the same DeskOS Core
semantics directly onto the official RHEL 10 bootc base.

The RHEL artifact is not derived from the CentOS image.

## Software model

DeskOS deliberately avoids becoming a universal package manager.

The managed baseline can use the delivery mechanism appropriate to the
software:

    distribution RPM
    official vendor RPM
    verified upstream binary
    system Flatpak
    future managed web applications

User/project environments such as mise, SDKMAN, Homebrew, language
version managers, dotfiles and personal Toolboxes are outside the DeskOS
managed baseline.

## GNOME

DeskOS exposes administrator intent rather than dconf implementation
details.

An administrator should be able to describe things such as:

- window controls;
- wallpapers and branding;
- fonts;
- icon and cursor themes;
- ordered favorites;
- dock behavior;
- idle timeout;
- lock behavior;

without knowing which GNOME schema or dconf key implements them.

DeskOS Core ships reasonable defaults, but organizations can override
those defaults declaratively.
