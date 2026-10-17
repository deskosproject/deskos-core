# ADR 0016: A platform that keeps its vendor's desktop, and no assumed app menu

**Status:** accepted (2026-10-18)

## Context

DeskOS writes its GNOME defaults to one system dconf database
(`gnome.dconfDatabase`), and that only means anything if the platform's session
reads it. That was measured for the EL10 family and Fedora. It was **assumed** for
Ubuntu, which declared `dconfDatabase: distro` with no measurement behind it.

The measurement (2026-10-18, `quay.io/deskos/deskos-ubuntu-base:26.04`) says the
assumption was false:

- the base ships no `/etc/dconf/profile/user`, and no package provides it — the
  `dpkg` file lists of `dconf-cli`, `dconf-gsettings-backend` and `libdconf1` do
  not carry it, and only `ibus` owns the directory. With the implicit profile,
  `dconf read` on a key compiled into `distro` returns nothing;
- GDM's own profile there is `user-db:user` plus a `file-db`, so the login screen
  does not read it either, and `loginScreen: true` was false for the same reason.

So the five GNOME defaults DeskOS declared for the Ubuntu workstation reached
**nobody**, while the plan said the image carried them. That is the error class this
project keeps paying for: something declared that does not apply.

The same class appeared in a smaller place. Lowering `windows.buttons` always
produced `appmenu:<buttons>` (or `<buttons>:appmenu`), adding a token of GNOME's own
default (`appmenu:close`, measured on CentOS Stream 10) that no profile declared.

## Decision

1. A platform whose base ships a desktop of its own declares
   `gnome.keepsVendorDefaults: true`. DeskOS writes **no** dconf default and **no**
   lock for it, and **any** GNOME setting a profile declares there **fails
   composition** with the reason. That holds at every layer: a lock is enforcement and
   Core never locks (AGENTS.md invariant 8), and the setting is a requirement this
   platform has no way to meet. There is no "best effort" case, because DeskOS does not
   drop a declared value and call the composition good.
2. DeskOS's own GNOME defaults move **out of the `deskos-core` foundation profile**
   into `deskos-gnome`, which a workstation that wants them selects. This is the same
   shape as `deskos-look`: Core ships the opinion, the workstation asks for it. Ubuntu
   does not select it, so an Ubuntu workstation declares no GNOME settings at all and
   composes with nothing to fail — which is what makes rule 1 workable without a
   layer-versus-ownership guess.
3. Ubuntu 26.04 declares the fact. A value that must reach an Ubuntu session takes a
   vendor mechanism (a compiled GSettings override, the plan's
   `gsettingsVendorDefaults`), never the dconf database, because no session reads
   that database there.
4. `windows.buttons` writes exactly the buttons and the side a profile declares. The
   plan adds no app menu token of its own. GNOME's `button-layout` is
   `<left>:<right>` — the colon separates the window's left corner from its right
   corner — so the buttons go after the colon unless the profile asks for the left.
   A flavor that wants GNOME's app menu needs a declared field, which this ADR does
   not add.

## Consequences

- The Ubuntu workstation loses a dconf block that reached nobody, and `dconf update`
  disappears from its Containerfile because there is nothing to compile. No user
  sees a change.
- The session check has nothing to assert on Ubuntu instead of a value it could
  never confirm, so its coverage is honest without a weakening: nothing is promised
  there, so nothing is skipped.
- **A workstation that wants DeskOS's GNOME defaults now asks for `deskos-gnome`.**
  This is a deliberate break for a consumer that pinned a release and selects
  `deskos-core` alone: a flavor like DayTwo's RHEL workstations keeps its packages and
  its update policy, and stops inheriting the session lock, the idle timeout and the
  software-update policy until it adds the profile. The release notes name it, and the
  profile is additive, so the change is one line per workstation.
- Anything that must still reach the user on Ubuntu has to be delivered the vendor's
  way, which keeps [ADR 0014](0014-platform-base-ownership.md)'s split of who owns
  what intact: the base is the platform's, and DeskOS does not maintain a desktop it
  cannot deliver.
- Removing the app menu token changes an existing plan
  (`example-devops-rhel10`: `'appmenu:close'` becomes `':close'` — the close button
  stays on the right, where it was), which was reviewed as a golden diff. The side is
  asserted by its own test, because a golden that compares only the produced string
  would not catch a swap of the two sides.
- The plan remains the record of what was applied, so a reader does not have to know
  which platform keeps its vendor's defaults.
