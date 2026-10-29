# Software resources

Part of the [resource model](../resources.md): the kinds that add packages, repositories, binaries and Flatpaks.

## `PackageSet`

Packages, RPM groups (named by the Platform, e.g. `groups: [workstation]`) and
units:

| Field | Type | Notes |
|---|---|---|
| `groups`, `packages` | group names; package names | one of `groups`, `packages`, `rpmFiles` |
| `repositories`, `aptRepositories` | `RpmRepository` / `AptRepository` names | includes the repository; neither pins resolution |
| `enableUnits`, `rpmFiles[].url`, `.sha256`, `.gpgKeyFile` | `*.service` etc.; https URL; `sha256:<64 hex>`; relative path | signed RPMs published without a repository |

The foundation names its metapackage group and no packages: the base already
carries what a desktop needs. A browser, firewall and application channel belong to
a platform or organization (`PackageSet/rhel-family`), since composition unions sets
and offers no removal (ADR 0003), so the foundation would impose them everywhere
(ADR 0014). `rpmFiles` needs a non-floating https URL, the SHA-256 and `gpgKeyFile`
in the resource root, or the build fails.

## `RpmRepository`

An **official vendor repository**:

| Field | Type | Notes |
|---|---|---|
| `id`, `displayName`, `baseURL` | `^[A-Za-z0-9][A-Za-z0-9._:-]*$`; one line; https URL | |
| `gpgKeyFiles`, `enabled`, `gpgCheck` | relative paths, local assets; boolean | `gpgCheck` default true and needs keys |

```yaml
spec: {id: code, baseURL: https://packages.microsoft.com/yumrepos/vscode, gpgKeyFiles: [keys/microsoft.asc]}
```

The keys are **local assets** in the image (`/etc/pki/rpm-gpg/`), so the repository
file references them by `file://` and dnf never trusts a fetched key.

## `AptRepository`

A **signed APT source** for a platform whose package manager is apt:

| Field | Type | Notes |
|---|---|---|
| `id`, `displayName`, `baseURL`, `suite` | `^[A-Za-z0-9][A-Za-z0-9._-]*$`; one line; https URL; one line | the id is a file name; an archive-path suite ends with `/` |
| `components`, `architectures`, `gpgKeyFiles` | e.g. `main`; e.g. `amd64`; at least one key | a named suite takes a component; `architectures` optional |

```yaml
spec: {id: vscode, baseURL: https://packages.microsoft.com/repos/code, suite: stable, components: [main], gpgKeyFiles: [keys/microsoft.asc]}
```

It renders as one **deb822** stanza in `/etc/apt/sources.list.d/deskos-<id>.sources`
and its key as `/etc/apt/keyrings/deskos-<sha256>.asc`, both copied before the first
package transaction. `Signed-By` names the keyring, so an unsigned APT source is not
expressible (ADR 0015). Apt rules apply: suite and components are one decision; the
id takes only letters, digits, dots, hyphens and underscores, because apt ignores
other names in `sources.list.d`; with `architectures` omitted, apt uses its own
list.

## `BinaryArtifact`

A verified upstream file, **pinned by version and SHA-256**:

| Field | Type | Notes |
|---|---|---|
| `version`, `source.url`, `source.sha256` | `^[0-9A-Za-z][0-9A-Za-z._+-]*$`; https URL; `sha256:<64 hex>` | |
| `archive` | `none` or `tar.gz` | |
| `files[].path`, `.destination` | path inside the archive; `/usr/local/bin/<name>`, `/usr/local/share/fonts/<name>.<ttf|otf|ttc|txt>` or `/usr/bin/deskos` | |
| `files[].mode` | `0755` or `0555` for an executable, `0644` or `0444` for a font | default: `0755`, or `0644` in the font directory |

A font goes under `/usr/local/share/fonts` because fontconfig reads that directory
and no package owns it, so a family the platform does not package can still ship
pinned by checksum (ADR 0018). The extension is part of the rule: the destination
is a font or the licence text beside it, never any other payload.

```yaml
spec:
  version: 4.22.17
  source: {url: https://mirror.openshift.com/pub/openshift-v4/clients/ocp/4.22.17/openshift-client-linux-amd64-rhel9-4.22.17.tar.gz, sha256: 87f436029301f4856d3c3df85586bd34aafb807d7870939f1aa4885afdd4834f}
  archive: tar.gz
  files: [{path: oc, destination: /usr/local/bin/oc, mode: "0755"}]
```

A graphical application can also declare a **`desktopEntry`** for the menu and dock:

| `desktopEntry` field | Rule |
|---|---|
| `id`, `name`, `comment` | reverse-DNS name (file `/usr/share/applications/<id>.desktop`, icon name `<id>`); one line, without control characters or surrounding whitespace; `comment` optional |
| `icon` | asset inside the resource root: a square PNG of a size the hicolor theme defines (16, 22, 24, 32, 36, 48, 64, 72, 96, 128, 192, 256 or 512 pixels) in `hicolor/<size>x<size>/apps/`, or an SVG in `hicolor/scalable/apps/` |
| `categories`, `startupWMClass` | registered freedesktop categories known to the platforms' `desktop-file-validate`, at least one a main category (`ConsoleOnly` is not accepted); optional window class, so the dock groups windows under the launcher |

`Exec` and `TryExec` are **not authored**: they are the destination of the
artifact's only file, so `desktopEntry` requires exactly one file. Two artifacts
with the same `id` conflict unless their entries are identical.

## `FlatpakRemote` and `FlatpakSet`

A **system** Flatpak remote, and applications preinstalled from it:

| Field | Type |
|---|---|
| `title`, `url`, `transport` | one line; https URL; `ostree` (default) or `oci` |
| `collectionID`, `gpgKeyFile` | collection id; relative path; `ostree` only |
| `authenticator`, `homepage`, `comment`, `description` | authenticator name (`oci`); https URL; one line |

```yaml
spec: {title: Flathub, url: https://dl.flathub.org/repo/, gpgKeyFile: keys/flathub.gpg}
```

An **OCI** remote indexes Flatpak images in a container registry over `oci+https`
(the scheme DeskOS adds to `url`): the registry and images carry the signatures, so
it has no `gpgKeyFile` and no `collectionID`. A registry that asks for credentials
names its authenticator, for example `org.flatpak.Authenticator.Oci`; the machine
supplies them, and the artifact never carries a token. DeskOS writes
`/usr/share/flatpak/remotes.d/<name>.flatpakrepo`, which cannot carry an
authenticator, so a remote that names one also gets a unit that points the remote at
it. The credentials stay on the machine, for example in `/etc/flatpak/oci-auth.json`,
and are not part of the image.

```yaml
spec: {remote: flathub, applications: [{id: io.github.kolunmi.Bazaar, branch: stable}]}
```
