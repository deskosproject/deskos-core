# Software resources

Part of the [resource model](../resources.md): the kinds that add packages, repositories, binaries and Flatpaks.

## `PackageSet`

Packages, RPM groups (named by the Platform) and units to enable:

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: PackageSet
metadata:
  name: deskos-core
spec:
  groups:
    - workstation
```

The foundation names its own metapackage group and no individual packages: the
vendor's base already carries what a desktop needs (measured: naming
`NetworkManager` installed nothing because the base already ships it, and naming it
forced Core to know one
family's spelling of it). A browser, a firewall and an application channel are
platform or organization choices: the RHEL family keeps its browser, its firewall
and its application channel in `PackageSet/rhel-family`, which workstations on those
platforms add through `Profile/rhel-family`. They cannot live in the foundation,
because composition unions sets and offers no removal (ADR 0003), so every platform
would be forced to carry them (ADR 0014).

Naming nothing has a price worth stating: nothing here asserts that a platform's
base carries what a desktop needs, so "the base has NetworkManager" became a fact to
check per base rather than a requirement the model enforces. That check belongs to
whoever validates a base digest, not to the foundation.

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

A set also names the **sources its packages need**, and the reference is typed like
the repository it names: `repositories` names `RpmRepository` resources and
`aptRepositories` names `AptRepository` resources. Either includes that repository
in the image. Neither pins which repository a package is resolved from: that stays
the package manager's own resolution and priority order at build time, on both
managers, and a package the vendor also ships elsewhere can still come from there.
A set that names the other manager's kind is included, and fails once the
platform's manager is known, with that reason.

## `RpmRepository`

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
  gpgKeyFiles:
    - keys/microsoft.asc
```

The signing keys are **local assets**, read into the plan and placed in the
image (`/etc/pki/rpm-gpg/`), so the repository file references them by
`file://` and a dnf transaction never trusts a key fetched from the network.

## `AptRepository`

A **signed APT source**, for a platform whose package manager is apt: its suite
and components, the architectures it serves, and the vendor's key.

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: AptRepository
metadata:
  name: vscode
spec:
  id: vscode
  displayName: Visual Studio Code - Stable
  baseURL: https://packages.microsoft.com/repos/code
  suite: stable
  components: [main]
  architectures: [amd64]
  gpgKeyFiles:
    - keys/microsoft.asc
```

It renders as one **deb822** stanza in
`/etc/apt/sources.list.d/deskos-<id>.sources`, and its key as
`/etc/apt/keyrings/deskos-<sha256>.asc`; both are copied into the image before the
first package transaction, so a group member or a package that comes from a
declared source has the source on disk by then. `Signed-By` names the keyring, and
apt then requires that repository's own metadata to be signed by that key — an
unsigned APT source is therefore not expressible, and the rendering refuses one
with no key rather than writing a source apt would trust against every key it has.

Three of the rules come from apt, not from this project:

- **Suite and components are one decision.** apt tells them apart by the trailing
  slash: a suite that is an exact archive path ends with `/` and takes no
  components (the Kubernetes archive is the real case), and a named suite takes at
  least one.
- **The id is a file name.** apt reads only letters, digits, dots, hyphens and
  underscores in `sources.list.d`, and ignores any other name *silently*; a wider
  id is refused at compile time rather than written where nothing reads it.
- **`architectures` is optional.** Omitted, apt downloads the indexes for its own
  architecture list; named, it fetches only those.

This is the same vendor the `RpmRepository` example above declares, at that
vendor's APT endpoint — a different archive from the RPM one — expressed for the
other package manager: a repository of one manager is not a repository of another
(ADR 0015).

## `BinaryArtifact`

A verified upstream binary, **pinned by version and SHA-256**:

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: BinaryArtifact
metadata:
  name: openshift-client
spec:
  version: 4.22.17
  source:
    url: https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.22.17/openshift-client-linux-amd64-rhel9-4.22.17.tar.gz
    sha256: 87f436029301f4856d3c3df85586bd34aafb807d7870939f1aa4885afdd4834f
  archive: tar.gz
  files:
    - path: oc
      destination: /usr/local/bin/oc
      mode: "0755"
```

A graphical application installed this way can declare a
**`desktopEntry`**, so it appears in the GNOME application menu and can
be added to the dock favorites. The example below is organization
content, not a file of this repository; this Ghostty AppImage is a
community build, not an official Ghostty release:

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: BinaryArtifact
metadata:
  name: ghostty
spec:
  version: 1.3.1
  source:
    url: https://github.com/pkgforge-dev/ghostty-appimage/releases/download/v1.3.1/Ghostty-1.3.1-x86_64.AppImage
    sha256: fde48d2b716afd1978766879bbf1aae30dd305e8ad86a1037a2614a14d82dc28
  archive: none
  files:
    - destination: /usr/local/bin/ghostty
      mode: "0755"
  desktopEntry:
    id: com.mitchellh.ghostty
    name: Ghostty
    comment: Fast, native terminal emulator
    icon: ../assets/ghostty.png
    categories: [System, TerminalEmulator]
    startupWMClass: com.mitchellh.ghostty
```

| Field | Rule |
|---|---|
| `id` | reverse-DNS name (dot-separated elements of `[A-Za-z0-9_-]`, none starting with a digit); the file is `/usr/share/applications/<id>.desktop` and the icon name is `<id>` |
| `name`, `comment` | one line of text, without control characters or surrounding whitespace; `comment` is optional |
| `icon` | asset inside the resource root: a square PNG of a size the hicolor theme defines (16, 22, 24, 32, 36, 48, 64, 72, 96, 128, 192, 256 or 512 pixels), installed in `hicolor/<size>x<size>/apps/`, or an SVG, installed in `hicolor/scalable/apps/` |
| `categories` | registered freedesktop categories known to the platforms' `desktop-file-validate`, at least one of them a main category; `ConsoleOnly` is not accepted |
| `startupWMClass` | optional; matches the application's window class so the dock groups its windows under the launcher |

`Exec` and `TryExec` are **not authored**: they are the destination of
the artifact's only file, without arguments, so `desktopEntry` requires
an artifact with exactly one file. The entry also sets
`Type=Application` and `Terminal=false`. Two artifacts that declare the
same `id` conflict unless their entries are identical.

## `FlatpakRemote` and `FlatpakSet`

A **system** Flatpak remote, and applications preinstalled from it. The
`transport` field selects the repository model and defaults to `ostree`:

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

An **OCI** remote is an index of Flatpak images in a container registry,
which flatpak reads over the `oci+https` transport. The registry and its
images carry the signatures, so an OCI remote has no `gpgKeyFile` and no
`collectionID`, and `url` stays a plain `https://` URL: DeskOS adds the
scheme. A registry that asks for credentials names its authenticator, and
the machine supplies them; the artifact never carries a token.

```yaml
apiVersion: software.deskos.org/v1alpha1
kind: FlatpakRemote
metadata:
  name: katello
spec:
  title: Katello
  transport: oci
  url: https://satellite.example.com
  authenticator: org.flatpak.Authenticator.Oci
```

DeskOS writes `/usr/share/flatpak/remotes.d/<name>.flatpakrepo`. That file
format cannot carry an authenticator, so a remote that names one also gets
a unit that points the remote at it once the remote exists. The credentials
stay on the machine, for example in `/etc/flatpak/oci-auth.json`, and are
not part of the image.

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
