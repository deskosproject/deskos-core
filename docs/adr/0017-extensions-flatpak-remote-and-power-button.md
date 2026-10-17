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
   `enabled` is a **decision**, not a value: absent inherits, `true` turns the
   extension on, `false` turns it off. Between layers the highest wins; two different
   decisions for the same UUID **in the same layer** are a conflict, never an
   order-dependent winner (ADR 0003).
2. The effective `enabled-extensions` list is computed **once**, from the platform's
   `enabledExtensions`, applying the decisions, plus Dash to Dock when `dock.enabled`
   is true, in a stable order. No writer appends to it independently.
3. Dash to Dock's UUID is reserved for `dock.enabled`; declaring it in
   `shell.extensions` is a validation error, so `dock.enabled: false` never competes
   with a second writer.
4. A UUID that is neither in the platform's catalog (`gnome.extensions`) nor in its
   `enabledExtensions` **fails composition** with the platform's reason; it is never
   dropped silently. An enabled extension's package is installed from the catalog.
5. The plan states that membership in `enabled-extensions` is the **declared
   default**: whether GNOME Shell actually loaded the extension is a session check,
   not a composition claim.

### Configuring an extension

6. Settings are **named capabilities** an extension's platform entry declares, each
   with its type, allowed values, schema and dconf path. A profile names a capability
   and a value; it never names a dconf key, and the path never comes from the UUID.
   A capability whose value is a command is excluded by construction.
7. v1 ships **no** capability: the field turns extensions on and off, and the shape
   above is the only way a setting may be added later. An extension whose settings are
   already modeled (Dash to Dock, through the `dock.*` keys) keeps them.

### Retiring a Flatpak remote

8. A **workstation** learns one new field, list-shaped and justified by the fact it
   states:
   ```yaml
   spec:
     flatpak:
       removeRemotes:
         - name: flathub
           reason: apps come from the organization's own remote
   ```
   `reason` is mandatory, in the style of `unavailablePackages`.
9. A remote that the composition **declares** (a `FlatpakRemote`) and also retires is
   a composition error: the two say the opposite about the same key.
10. A remote a declared `FlatpakSet` installs from cannot be retired: the image's own
    first-boot install would lose its source, so composition fails with that reason.
11. The removal is an **image** operation: the plan records it and the backend removes
    the remote's configuration at build time. It is **not** `flatpak uninstall`:
    applications already installed from that remote stay on the machine (their data
    lives under `/var/lib/flatpak`, which an update keeps), and the plan says so.
    DeskOS never passes `--force`, and never removes an application.
12. A fresh install has nothing from that remote. An updated machine keeps what it
    already had from it. The plan states both, so nothing is promised about removing
    what is already installed.

### The power button

13. A new semantic key, `power.buttonAction`, lowers to
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
