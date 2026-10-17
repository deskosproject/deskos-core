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
   `gnome.keepsVendorDefaults: true`. DeskOS then writes **no** dconf default and
   **no** lock for it: the platform's desktop is the vendor's. Every GNOME setting a
   profile declared is recorded in the plan's warnings as not applied, naming the
   fact, so the omission is visible rather than silent. A platform with the fact must
   not declare `gnome.dconfDatabase`; a platform without it must.
2. Ubuntu 26.04 declares it. A value that must reach an Ubuntu session takes a
   vendor mechanism (a compiled GSettings override, the plan's
   `gsettingsVendorDefaults`), never the dconf database, because no session reads
   that database there.
3. `windows.buttons` writes exactly the buttons and the side a profile declares. The
   plan adds no app menu token of its own. A flavor that wants GNOME's app menu needs
   a declared field, which this ADR does not add.

## Consequences

- The Ubuntu workstation loses a dconf block that reached nobody, and `dconf update`
  disappears from its Containerfile because there is nothing to compile. No user
  sees a change.
- The session check has nothing to assert on Ubuntu instead of a value it could
  never confirm, so its coverage is honest without a weakening: nothing is promised
  there, so nothing is skipped.
- A GNOME default declared on such a platform is a **warning that names the fact**,
  not a silent drop and not a composition failure. Failing would be wrong: the
  platform is not missing a requirement it was asked to meet, it is a desktop we did
  not touch.
- Anything that must still reach the user on Ubuntu has to be delivered the vendor's
  way, which keeps [ADR 0014](0014-platform-base-ownership.md)'s split of who owns
  what intact: the base is the platform's, and DeskOS does not maintain a desktop it
  cannot deliver.
- Removing the app menu token changes an existing plan
  (`example-devops-rhel10`: `'appmenu:close'` becomes `'close:'`), which was reviewed
  as a golden diff.
- The plan remains the record of what was and was not applied, so a reader does not
  have to know which platform keeps its vendor's defaults.
