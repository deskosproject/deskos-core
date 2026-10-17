# ADR 0017: Extensions a profile may turn on, a Flatpak remote a workstation may retire, and the power button

**Status:** proposed (2026-10-18)

## Context

Three gaps the desktop-defaults line of work found, and the boundary each one has to
respect.

**Extensions.** GNOME extensions are a `Platform` fact (`gnome.extensions`,
`gnome.enabledExtensions`) and a profile can only turn on Dash to Dock, through
`dock.enabled`. AppIndicator, the most requested item, has nowhere to be declared.
Enabling one by its platform-declared UUID does not cross the managed-baseline
boundary, but *configuring* one with a free key/value map would: an extension's
settings can execute commands, and GNOME keeps an extension's schema id and its
dconf path separate, with relocatable schemas, so a path derived from the UUID is
not reliable.

**Retiring a Flatpak remote.** Declaring a remote is a keyed definition. Retiring
one is a negative operation over a model of definitions, and it collides with the
image's own first-boot work: `flatpak preinstall` installs a `FlatpakSet`'s apps from
the remotes the image carries, and `flatpak remote-delete` refuses to remove a remote
whose apps are installed unless it is forced.

**The power button.** The hardware power button's action is not expressible. The
session timeout the same decision covers already exists
(`session.idle.blankAfter` → `org.gnome.desktop.session idle-delay`).

## Decision

### Extensions a profile turns on

1. A `GnomeProfile` gains `shell.extensions`: a list of `{uuid, enabled}`.
   `enabled` is a **decision**, not a value, and it is **required** in an entry — there
   is no `inherit` value and no default for it. "Inherit the platform's own decision"
   is expressed by **not naming the UUID at all**. Naming one UUID twice in the same
   list is a validation error: the intent is ambiguous and the model does not guess.
2. Internally each decision is one scalar keyed by its UUID, so the existing rules
   apply unchanged (ADR 0003): the highest layer wins, and the same UUID decided two
   different ways **in the same layer** is a conflict, never an order-dependent
   winner. Two layers may disagree, and the higher one decides.
3. The effective `enabled-extensions` list is computed **once**, from the platform's
   `enabledExtensions`, applying the decisions, plus Dash to Dock when `dock.enabled`
   is true, in a stable order. No writer appends to it independently.
4. **When the key is written:** the plan writes `enabled-extensions` only when the
   composition changes the platform's own list — an extension turned on or off, or the
   dock enabled or disabled. With no decision and no dock, the plan writes nothing and
   the platform's own GNOME default stands. A UUID that the platform lists in
   `enabledExtensions` is presumed already shipped and enabled: turning it **off**
   needs no entry in the extension catalog, because there is no package to install.
   `dock.enabled: false` removes the dock's UUID the same way, whether or not the
   platform offers it as a package (ADR 0008's disable path keeps its semantics).
5. Dash to Dock's UUID is reserved for `dock.enabled`, including when the platform
   enables it by default; declaring it in `shell.extensions` is a validation error, so
   `dock.enabled: false` never competes with a second writer.
6. A UUID that is neither in the platform's catalog (`gnome.extensions`) nor in its
   `enabledExtensions` **fails composition** with the platform's reason; it is never
   dropped silently. An enabled extension's package is installed from the catalog.
7. The plan states that membership in `enabled-extensions` is the **declared
   default**: whether GNOME Shell actually loaded the extension is a session check,
   not a composition claim.
8. On a platform that declares `gnome.keepsVendorDefaults` (ADR 0016), this field
   follows the same rule as every other GNOME setting: nothing is written, a decision
   that comes from the foundation is reported in the plan's warnings as not applied,
   and a lock or a decision a higher layer asked for **fails composition**. The new
   field does not reopen the hole ADR 0016 closed.

### Configuring an extension

9. Settings are **named capabilities** an extension's platform entry declares, each
   with its type, allowed values, schema and dconf path. A profile names a capability
   and a value; it never names a dconf key, and the path never comes from the UUID.
   A capability whose value is a command is excluded by construction.
10. v1 ships **no** capability: the field turns extensions on and off, and the shape
   above is the only way a setting may be added later. An extension whose settings are
   already modeled (Dash to Dock, through the `dock.*` keys) keeps them.

### Retiring a Flatpak remote

11. A **workstation** learns one new field, list-shaped and justified by the fact it
   states:
   ```yaml
   spec:
     flatpak:
       removeRemotes:
         - name: flathub
           reason: apps come from the organization's own remote
   ```
   `reason` is mandatory, in the style of `unavailablePackages`.
12. A remote that the composition **declares** (a `FlatpakRemote`) and also retires is
   a composition error: the two say the opposite about the same key.
13. A remote a declared `FlatpakSet` installs from cannot be retired: the image's own
    first-boot install would lose its source, so composition fails with that reason.
14. The removal is an **image** operation: the plan records it and the backend removes
    the remote's configuration at build time. It is **not** `flatpak uninstall`:
    applications already installed from that remote stay on the machine (their data
    lives under `/var/lib/flatpak`, which an update keeps), and the plan says so.
    DeskOS never passes `--force`, and never removes an application.
15. A fresh install has nothing from that remote. An updated machine keeps what it
    already had from it. The plan states both, so nothing is promised about removing
    what is already installed.

### The power button

16. A new semantic key, `power.buttonAction`, lowers to
    `org.gnome.settings-daemon.plugins.power power-button-action`, with GNOME's own
    values: `nothing`, `suspend`, `hibernate`, `interactive` (measured on the built
    CentOS Stream 10 image; the schema's default is `suspend`). It is a scalar like
    every other option, so the layers, locks and messages need nothing new.

## Consequences

- Enabling an extension is a **field on an existing kind**, so the public kind count
  stays at fourteen (invariant 4).
- `enabled-extensions` has one writer. `dock.enabled` and `shell.extensions` cannot
  disagree, and a manifest ordering cannot change the output.
- The vocabulary grows by one key (the power button) and by the extension decisions,
  which are bounded by the platform's own catalog rather than by the profile.
- A profile can name an extension and, when a platform declares a capability, a
  setting for it. It can never introduce a dconf key, a schema path or a command.
- Retiring a remote cannot break the image's own install, cannot silently force, and
  cannot claim to remove applications that stay; the reason field makes the intent
  reviewable.
- The two lists a workstation adds are refusals and decisions, not new software, so
  the boundary of ADR 0005 holds: DeskOS declares what the machine keeps, not what
  the user installs.
