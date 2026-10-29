# ADR 0017: Extensions a profile may turn on, a Flatpak remote a workstation may retire, and the power button

**Status:** accepted (2026-10-18)

## Context

Three gaps. Extensions are a `Platform` fact (`gnome.extensions`, `gnome.enabledExtensions`) and a profile could
only turn on Dash to Dock; enabling a platform-declared UUID stays inside the managed-baseline boundary, but
configuring one with a free key/value map would not, because an extension's settings can execute commands and
its schema id and dconf path are separate. Retiring a Flatpak remote is a negative operation that collides with
first-boot `flatpak preinstall` and that `flatpak remote-delete` refuses unless forced, and the power button's
action is not expressible.

## Decision

### Extensions a profile turns on

1. A `GnomeProfile` gains `shell.extensions`: a list of `{uuid, enabled}`. `enabled` is a **decision**, required
   in an entry, with no `inherit` value and no default (not naming the UUID inherits the platform's decision);
   naming one UUID twice in the same list is a validation error.
2. The effective `enabled-extensions` list is computed **once** from the platform's `enabledExtensions` with the
   decisions applied, in a stable order; no writer appends to it independently. The plan states the list it
   enables whenever the composition carries any GNOME setting, and a catalogued UUID gets its package.
3. **Enabling is open.** Any UUID may be decided, because an extension the composition brings itself arrives in
   the packages it declares. The session check asks GNOME about each one, so a decision that does not load is a
   red check.
4. **The dock is an extension like any other**: `dock.position`, `dock.behavior`, `dock.iconSize` and
   `dock.showTrash` apply only while its UUID's decision is true, and the `dash-to-dock` catalog entry names that
   UUID and installs the package or pinned artifact.
5. On a platform that declares `gnome.keepsVendorDefaults` (ADR 0016), nothing is written and a decision
   declared there **fails composition** with the platform's reason.

### Configuring an extension

6. **Only what DeskOS models can be configured from the artifact.** Settings are written by a typed adapter in
   the lowerer (the dock's is the one that exists), because a setting can be a value the shell **executes**. A
   profile names a semantic setting, never a key, a path or a command.

### Retiring a Flatpak remote

7. A **workstation** gains `flatpak.removeRemotes[]`, each entry with `name` and a mandatory `reason`, in the
   style of `unavailablePackages`.
8. A remote the composition **declares** (a `FlatpakRemote`) and also retires is a composition error; a
   `FlatpakSet` names its remote as a declared resource, so this covers it as well.
9. A remote the **base image** installs from is not visible to composition; retiring one is the workstation
   author's decision, which the mandatory `reason` makes reviewable.
10. The removal is an **image** operation: the plan records it and the backend removes the remote's configuration
   at build time. It is **not** `flatpak uninstall`: applications already installed stay (their data lives under
   `/var/lib/flatpak`, which an update keeps), DeskOS never passes `--force`, and never removes an application.

### The power button

11. `power.buttonAction` lowers to `org.gnome.settings-daemon.plugins.power power-button-action`, with GNOME's
    values `nothing`, `suspend`, `hibernate` and `interactive` (measured on the built CentOS Stream 10 image;
    schema default `suspend`). It is a scalar like every other option.

## Consequences

- Enabling an extension is a **field on an existing kind**, so the public kind count stays at fourteen; the two
  lists a workstation adds are refusals and decisions, not new software, so the boundary of ADR 0005 holds.
