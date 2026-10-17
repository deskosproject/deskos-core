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
   lock for it: the platform's desktop is the vendor's. A platform with the fact must
   not declare `gnome.dconfDatabase` or `gnome.loginScreen`; a platform without it
   must declare a database. What happens to a declared setting depends on who asked:
   - a **lock** always fails composition, at any layer. A lock is enforcement, and
     Core never locks (AGENTS.md invariant 8), so a lock here can only be an owner's
     requirement that this platform cannot enforce; recording it as a warning would
     let a guarantee that does not exist pass `validate`;
   - a **default** is recorded in the plan's warnings as not applied, naming the fact,
     because Core's own foundation opinion is what an Ubuntu workstation carries and
     the platform's nature is what scopes it. The omission stays visible rather than
     silent.
   - a known limit of that rule: `foundation` is a **layer**, not ownership — an owner
     may author a foundation-layer profile, and the layer alone does not prove the
     opinion is dispensable. Until resources carry an owner, the alternative is to
     keep Core's GNOME settings out of the profile list an Ubuntu workstation selects,
     which changes what `deskos-core` gives every other consumer (including an
     external one that pins releases). That trade is named here rather than settled
     silently.
2. Ubuntu 26.04 declares it. A value that must reach an Ubuntu session takes a
   vendor mechanism (a compiled GSettings override, the plan's
   `gsettingsVendorDefaults`), never the dconf database, because no session reads
   that database there.
3. `windows.buttons` writes exactly the buttons and the side a profile declares. The
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
- A default declared on such a platform is a **warning that names the fact**; a lock
  is an **error at any layer**. Neither is a silent drop, and neither weakens a check:
  the platform is not missing a requirement Core asked of itself, and it cannot be
  made to hold a guarantee it has no way to enforce.
- Anything that must still reach the user on Ubuntu has to be delivered the vendor's
  way, which keeps [ADR 0014](0014-platform-base-ownership.md)'s split of who owns
  what intact: the base is the platform's, and DeskOS does not maintain a desktop it
  cannot deliver.
- Removing the app menu token changes an existing plan
  (`example-devops-rhel10`: `'appmenu:close'` becomes `':close'` — the close button
  stays on the right, where it was), which was reviewed as a golden diff. The side is
  asserted by its own test, because a golden that compares only the produced string
  would not catch a swap of the two sides.
- The plan remains the record of what was and was not applied, so a reader does not
  have to know which platform keeps its vendor's defaults.
