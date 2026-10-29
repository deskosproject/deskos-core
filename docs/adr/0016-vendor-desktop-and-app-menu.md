# ADR 0016: A platform that keeps its vendor's desktop, and no assumed app menu

**Status:** accepted (2026-10-18)

## Context

DeskOS writes its GNOME defaults to one system dconf database (`gnome.dconfDatabase`), which only works if the
platform's session reads it: measured for the EL10 family and Fedora, **assumed** for Ubuntu, which declared
`dconfDatabase: distro`. The 2026-10-18 measurement (`quay.io/deskos/deskos-ubuntu-base:26.04`) found no
`/etc/dconf/profile/user` and no package providing it, so `dconf read` on a key compiled into `distro` returned
nothing, and GDM's `user-db:user` plus `file-db` profile made `loginScreen: true` false. Five GNOME defaults
reached nobody while the plan said the image carried them; `windows.buttons` also added `appmenu:close` unasked.

## Decision

1. A platform whose base ships a desktop of its own declares `gnome.keepsVendorDefaults: true`. DeskOS writes
   **no** dconf default and **no** lock for it, and **any** GNOME setting a profile declares there **fails
   composition** with the reason. That holds at every layer: a lock is enforcement and Core never locks
   (invariant 8), and the setting is a requirement this platform has no way to meet. There is no "best effort"
   case.
2. DeskOS's own GNOME defaults move **out of the `deskos-core` foundation profile** into `deskos-gnome`, which a
   workstation that wants them selects — the same shape as `deskos-look`. Ubuntu does not select it, so an
   Ubuntu workstation declares no GNOME settings and composes with nothing to fail.
3. Ubuntu 26.04 declares the fact. A value that must reach an Ubuntu session takes a vendor mechanism (a
   compiled GSettings override, the plan's `gsettingsVendorDefaults`), never the dconf database, because no
   session reads that database there.
4. `windows.buttons` writes exactly the buttons and the side a profile declares, and the plan adds no app menu
   token of its own. `button-layout` is `<left>:<right>`, so the buttons go after the colon unless the profile
   asks for the left. A flavor that wants GNOME's app menu needs a declared field, which this ADR does not add.

## Consequences

- The Ubuntu workstation loses a dconf block that reached nobody, and `dconf update` disappears from its
  Containerfile because there is nothing to compile. No user sees a change.
- The session check has nothing to assert on Ubuntu instead of a value it could never confirm, so its coverage
  is honest without a weakening.
- **A workstation that wants DeskOS's GNOME defaults now asks for `deskos-gnome`.** This is a deliberate break
  for a consumer that pinned a release and selects `deskos-core` alone: until it adds the profile it stops
  inheriting the session lock, the idle timeout and the software-update policy. The release notes name it.
- Removing the app menu token changes an existing plan (`example-devops-rhel-10`: `'appmenu:close'` becomes
  `':close'`), reviewed as a golden diff; the side is asserted by its own test, because a golden that compares
  only the produced string would not catch a swap of the two sides.
- Anything that must still reach the user on Ubuntu is delivered the vendor's way, which keeps
  [ADR 0014](0014-platform-base-ownership.md)'s split of who owns what intact.
