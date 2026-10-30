# The desktop as a package list, not a group

Question: can a DeskOS platform install the GNOME desktop from an explicit package
list instead of `workstation-product-environment`, so that a change in the
distribution's `comps.xml` cannot add a package we did not ask for?

## What the group actually does today

Measured on `quay.io/centos-bootc/centos-bootc:stream10` (2026-10-30), installing
the environment group the way the build does:

| | Packages |
|---|---|
| Base image | 488 |
| After `dnf group install workstation-product-environment` | 1333 |
| **Added** | **845** |

The environment is eleven groups. Their direct members add up to **297**; the other
548 packages are dependencies. The groups and their direct member counts:

| Group | Members |
|---|---|
| Core | 65 |
| Workstation product core | 75 |
| Standard | 75 |
| GNOME | 56 |
| Multimedia | 11 |
| Printing Client | 10 |
| Guest Desktop Agents | 4 |
| Internet Browser | 1 |
| Common NetworkManager submodules, Fonts, Hardware Support | 0 (they pull categories) |

What arrives with it, beyond a session, and is worth naming:

- **18 desktop applications**: `baobab`, `firefox` (the RPM, next to the Flatpak the
  profiles declare), `gnome-calculator`, `gnome-characters`, `gnome-clocks`,
  `gnome-color-manager`, `gnome-disk-utility`, `gnome-font-viewer`,
  `gnome-initial-setup`, `gnome-remote-desktop`, `gnome-software`,
  `gnome-system-monitor`, `gnome-text-editor`, `gnome-user-docs`, `loupe`,
  `gnome-bluetooth`, `gnome-browser-connector`, `background-logo`.
- **16 input-method packages**, including the Chinese, Japanese and Korean engines
  (`ibus-anthy`, `ibus-hangul`, `ibus-libpinyin`, `ibus-libzhuyin`, `ibus-m17n`).
- **14 tools** a workstation rarely needs: `telnet`, `lrzsz`, `mtr`, `nmap-ncat`,
  `traceroute`, `mcelog`, `sos`, `psacct`, `quota`, `dos2unix`, `tree`, `bc`,
  `words`, and `setroubleshoot`, which the platform already excludes.

Some of those are deliberate (the input methods serve people who type CJK), some
are already handled (`gnome-software` is removed by ADR 0022), and some are simply
what a distribution thinks a workstation is.

## What the prior art really does

- **Bluefin LTS (CentOS Stream 10)** runs `dnf group install` for GNOME, with
  `exclude=` lines in COPR repositories and explicit upgrades before the group. It
  does not maintain a package list.
- **Bluefin Classic (Fedora)** starts `FROM ghcr.io/ublue-os/silverblue-main` and
  adds and removes packages. The desktop is inherited from Silverblue, not chosen
  in Bluefin's repository.

So the "package by package" discipline lives in the base image they inherit, not in
a manifest anyone maintains. That is the honest comparison, and it matters: 845
names is not a list a person keeps correct.

## The three options

**A. Keep the group, add exclusions.** What exists today: `rpmGroups` plus
`excludePackages`, which the build removes after the transaction and proves absent
(ADR 0022). Cheapest, and it lets dnf resolve the dependencies, but it is blind to a
comps change that adds a package nobody asked for.

**B. An explicit list.** Replace `rpmGroups` with `packages` on the platform: the
297 direct members, minus what we do not want, plus what we add. dnf still resolves
dependencies, so the installed set stays close to 845, but nothing arrives that the
list does not name. The cost is maintenance: a per-platform list (EL10 now, Fedora
45 later), a missing package is a broken session that only a boot finds, and a comps
change that *adds a required* package (a new GNOME dependency) would be missed
because our list is frozen.

**C. The group plus a committed manifest.** Install the group as today, then compare
the resolved set with a manifest committed in the repository and **fail the build
when they differ**, printing the diff. The manifest is reviewed like a golden: a
comps change becomes a pull request a person reads, not a silent install. It gives
the property we want without hand-maintaining 297 names, and it works the same on a
Fedora base that already carries the desktop.

## Recommendation

**C first, with A's exclusions kept.** It answers the actual worry ("a comps change
installs something we did not choose") with a review gate instead of a list nobody
can maintain, and it costs one backend step and one committed file.

**B is worth doing for the applications, not for the session.** The 18 desktop apps
and the 16 input methods are a decision a person can make and keep; the 297 session
packages are not. A trimmed list of applications, installed explicitly on top of the
group, is small enough to maintain and is where the "not more than enough" instinct
belongs.

## What it changes in the model

- A `packages` list on the platform, or a `manifest` beside `rpmGroups`; both are
  model decisions and need an ADR before a field exists.
- The manifest has to travel in the Plan and the SBOM, so `deskos report` can say
  what the image was expected to carry, not only what it carries.
- The build step is small: after the transaction, `rpm -qa` into a sorted list,
  compare with the manifest, and fail with the difference. The same list is what the
  SBOM already records, which makes the two impossible to drift apart silently.

## How it is proved

- Boot and session checks on the example workstation, which already assert the
  session, the extensions, the Flatpaks and the defaults.
- A `deskos check` run in the booted image, which compares declared and installed.
- A deliberate comps change: add a package to a local repository's comps, rebuild,
  and watch the manifest step fail with the diff. That is the test that proves the
  property, and it is the one nobody runs today.

## Fedora 45

`quay.io/fedora/fedora-silverblue:45` already carries the desktop: GNOME 51
(`gnome-shell`, `gdm`, `mutter`, `gnome-session`, `nautilus`,
`gnome-control-center`, `firefox`), 1509 packages, `bootc` 1.16.13 and ostree.
`fedora-bootc:45` has 540 packages and no `gnome-shell`.

So on Fedora the desktop is a **base image**, not an install: the list is
Silverblue's, curated by Fedora, and DeskOS adds and removes on top. The manifest in
option C still applies, and it is what makes "Silverblue changed under us" a pull
request instead of a surprise. Starting from `fedora-bootc` and installing GNOME
explicitly is possible, and it would mean maintaining a second list for no gain over
the image Fedora already tests.
