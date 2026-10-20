# Platforms

The canonical status of every platform DeskOS compiles onto, and how to get a
machine from each **today**. Every other page links here instead of repeating a
matrix; when a platform changes, this is the one table to edit.

DeskOS compiles the same Core intent onto a platform's own bootc base
([ADR 0004](adr/0004-core-is-platform-independent.md)). A platform whose vendor
publishes no bootc base may be added as an organization-owned base with a named
owner and a declared tier ([ADR 0014](adr/0014-platform-base-ownership.md)) —
Ubuntu 26.04 is that case, and it is built and published in a repository of its own,
[`deskosproject/deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu), on a base
the project owns ([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)). What this
repository publishes goes to one repository per platform
(`quay.io/deskos/deskos-core-centos-stream-10`, `-almalinux-10`), and the two below that are
published carry the digest and the run that verified them in
[architecture.md](architecture.md#validation-status). RHEL is never published, and
Fedora is not yet.

## Status

| Platform | Base (owner) | Base pinned | Composes and renders | Image built | Booted and checked | Published image | System Flatpak |
|---|---|---|---|---|---|---|---|
| CentOS Stream 10 | `quay.io/centos-bootc/centos-bootc` (vendor) | x86_64 manifest, by digest | **validated** (test, CI) | **CI** | **CI**: GNOME; session check | **published** (`quay.io/deskos/deskos-core-centos-stream-10`) | Flatpak preinstall |
| AlmaLinux 10 | `quay.io/almalinuxorg/almalinux-bootc` (vendor, **experimental**) | x86_64 manifest, by digest | **validated** (CI) | **CI** | **CI**: GNOME; session check | **published** (`quay.io/deskos/deskos-core-almalinux-10`) | `preinstall` via `rhel-family`; apps not verified |
| RHEL 10 | `registry.redhat.io/rhel10/rhel-bootc` (vendor, entitled) | x86_64 manifest, by digest | **validated** (test) | built by the organization | built by the organization | **never** | Flatpak preinstall |
| Fedora 45 | `quay.io/fedora/fedora-silverblue` (vendor) | x86_64 manifest, by digest | **validated** (test, CI) | **CI** | boot check in CI, not yet green | none | Flatpak preinstall |

`(vendor)` in the table means the base is the platform's own supplier's: Canonical for
Ubuntu, Red Hat for RHEL, the CentOS and AlmaLinux projects, the Fedora Project. Where it
says the DeskOS project, the base is organization-owned instead, with the named owner and
tier that ADR 0014 requires. The distinction matters beyond the base: the desktop a session
starts with is the vendor's on a platform that declares `gnome.keepsVendorDefaults`, and
DeskOS's own where it does not (ADR 0016).

"Composes and renders" means the workstation compiles to a deterministic plan and
build context. It does **not** mean the image builds or boots — that is the next
two columns. Evidence, per claim, is in
[architecture.md](architecture.md#validation-status) and the run artifacts.

## How to get a machine today

### CentOS Stream 10 — pull the image, then install

The image is published at `quay.io/deskos/deskos-core-centos-stream-10` as `:latest` and
`:<commit>` by the gate's `publish=true` dispatch, with its digest and the run in
[architecture.md](architecture.md#validation-status). Make a QCOW2 (or an
Anaconda ISO) from it with `bootc-image-builder`:
[install.md](install.md#install-a-published-image).

### RHEL 10 — build privately, never publish

An organization with RHEL subscriptions builds its own from the entitled base
and keeps it inside the organization; the RHEL EULA forbids redistributing it.
[install.md](install.md#rhel-10-private-builds-only).

### AlmaLinux 10 — a published image on an experimental base

AlmaLinux is a public, subscription-free EL10 base, so a build needs no
entitlement. CI builds and boots a derived AlmaLinux image
(`example-almalinux-10`) and runs the boot, session and first-boot checks on
it. That exercises the paths the example's profiles need — its package group, the
Plymouth packages, the GNOME facts — not every capability a platform declares:
the example declares no `FlatpakSet` or `TrustAnchor`, and does not test the base
updater. The image is published at `quay.io/deskos/deskos-core-almalinux-10`, and
**the base it derives from is the vendor's experimental one**: that is the caveat to
weigh, not whether the image exists. Making your own is the same shape as CentOS
Stream 10 ([install.md](install.md#install-a-published-image)).

### Ubuntu 26.04 — built and published in its own repository

Ubuntu has no bootc base from its vendor, so the project owns one, and both the base and the
workstation that builds on it live in a repository of their own:
[`deskosproject/deskos-ubuntu`](https://github.com/deskosproject/deskos-ubuntu), on base
`deskosproject/deskos-ubuntu-bootc`. That repository documents how to build it, how to write
a disk with `bootc install to-disk` — there is no Anaconda path on this platform, which is
why it is not in this page's install steps — and what is worth testing
([ADR 0020](adr/0020-ubuntu-lives-in-its-own-repository.md)).

This repository still **composes and renders** a Debian-family workstation, and its tests
say so. It no longer builds, boots or publishes one: the virtual machines, the publish leg
and the platform's resources moved with it.

### Fedora 45 — no published image; build from the vendor's Silverblue base

Fedora is published by the Fedora Project, is subscription-free, and its bootc
base is its own, so this platform needs no entitlement and no organization-owned
base: it follows ADR 0004's official-base rule on
`quay.io/fedora/fedora-silverblue` — Fedora's *desktop* image, not the minimal
`quay.io/fedora/fedora-bootc`, which carries no GNOME, no GDM, no Flatpak and no
Plymouth at all (measured). The measured facts behind the platform —
`workstation-product-environment` as the comps environment dnf5 resolves (347
packages on this pin), Flatpak 1.18.4 with `flatpak preinstall`,
`/etc/pki/ca-trust/source/anchors` with `update-ca-trust`,
`gtk-update-icon-cache`, the bootc update units (present, not enabled), GDM's
`/etc/gdm/custom.conf` — are in
[`resources/platforms/fedora-45.yaml`](../resources/platforms/fedora-45.yaml), one
comment per fact. [install.md](install.md#fedora-45) has the commands.

**The pin is a prerelease compose.** `:45` still resolves to `45.20261020.n.0`
(os-release reads "Silverblue Prerelease"), measured 2026-10-20, and that base
enables Fedora's `updates-testing` repository — Dash to Dock, for one, is published
only there on this compose. The resource and the table say so; when the released
compose appears it is a re-pin, not a rewrite. Fedora 44 is retired rather than
kept alongside ([roadmap.md](roadmap.md)).

The artifact this platform would publish is the shipped **reference workstation**
`deskos-core-fedora-45` ([`resources/workstations/`](../resources/workstations/)):
Core + `Profile/fedora` + the DeskOS look + the Flathub reference application,
under **its own image repository** — `quay.io/deskos/deskos-core-fedora-45` — and
never a shared `:latest` tag, because a Fedora release must not be able to change
under a machine at `bootc upgrade` ([roadmap.md](roadmap.md) says how a second
release would be carried). Nothing is published for Fedora today: publishing is a
manual, human-gated step ([supply-chain.md](supply-chain.md)).

Composition and rendering for this platform are covered by CI: the reference
workstation lives in the embedded Core, so every `deskosctl validate` run in
`ci.yml` composes it, the Fedora golden pins its Containerfile, and
`ci.yml` renders it. `vm-bootcheck.yml` also builds `examples/fedora-45` and
boots the resulting QCOW2 — boot check, instrumented session check (which reads
back every dconf default in the plan) and first-boot check — on the same path as
CentOS Stream 10 and AlmaLinux 10. That is the check that will produce Fedora's
first boot evidence; until it is green, Fedora has none, and this page does not
claim otherwise.

**Build evidence, measured 2026-10-20 on the pinned base.** The reference
workstation `deskos-core-fedora-45` was built locally that day (image
`14cb6852ae33`, 7.6 GB) and ran `bootc container lint` (12 checks passed, 1
skipped, 2 warnings: `nonempty-run-tmp`, `var-tmpfiles`). The build rebuilds the
initramfs for the kernel the image carries (7.2.11-300.fc45) and **fails unless**
`plymouthd`, the DeskOS theme, its watermark and throbber frames and
`Theme=deskos` are inside it, so it carries them. That local image was not
installed; CI's own build is the one that boots.

What is **not** measured matters as much: **no Fedora image is published**, and
no Fedora disk has been installed on hardware — the boot evidence is CI's QEMU
guest, the same class of evidence the other platforms carry.

Tray icons are part of this platform's desktop: `Profile/fedora` enables
`appindicatorsupport@rgcjonas.gmail.com`, and the package is Fedora's
`gnome-shell-extension-appindicator`. Fedora packages it. EL10 does **not** have a
usable package: it is absent from the vendor repositories (measured on CentOS
Stream 10 and AlmaLinux 10 on 2026-10-20: baseos, appstream, extras and CRB), and
EPEL 10's `61-1.el10_2` requires `libappindicator-gtk3`, whose `libdbusmenu`
dependency lives in CRB — enabled on AlmaLinux, disabled on CentOS Stream and
outside RHEL's supported set. So the EL10 catalog names a **pinned upstream
artifact** instead of a package, and the build verifies its sha256, its UUID and
that it supports the image's GNOME Shell before installing it
([ADR 0021](adr/0021-extensions-can-be-delivered-as-pinned-artifacts.md)). The
package name stays in `unavailablePackages`: asking for the RPM still fails at
composition, with the reason, while the profile gets the extension. [BetterTrayIcons](research-notes.md#tray-icons-on-gnome-2026-10-20) is not an
alternative we can ship: it *replaces* AppIndicator (its metadata declares
`appindicatorsupport@rgcjonas.gmail.com` as a conflict), it is GPL-3.0, Wayland
only, and it is distributed through extensions.gnome.org rather than a
distribution repository. It stays a per-user install for anyone who prefers it.

Two properties of this platform are decisions, and they are visible in the table
above rather than hidden:

- **The release is in the resource, the image name and the support window.** A
  new Fedora release is a new platform resource, a re-pinned digest, a
  re-validation and a new image repository; retirements and re-pins are **manual**,
  as every other base pin is ([roadmap.md](roadmap.md)).
- **One Fedora release at a time.** Fedora 44 was retired when 45 was promoted, so
  `resources/platforms/` carries a single Fedora platform and a machine's platform
  is unambiguous. The *prerelease* is a property of the **pin** — `45.20261020.n.0`
  — and not of the name: when the released compose is published, the digest and the
  measurement comments move and the platform keeps its name, its workstation and
  its image repository.
