# ADR 0017: Extensions a profile may turn on, a Flatpak remote a workstation may retire, and the power button

**Status:** accepted (2026-10-18)

## Context

Three gaps the desktop-defaults line of work found, and the boundary each one has to
respect.

**Extensions.** GNOME extensions are a `Platform` fact (`gnome.extensions`,
`gnome.enabledExtensions`) and a profile could only turn on Dash to Dock, through a
switch of its own. AppIndicator, the most requested item, had nowhere to be declared,
and a workstation could not bring an extension DeskOS had not modeled.
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
   `enabledExtensions` with the decisions applied, in a stable order. No writer
   appends to it independently.
4. The plan **states the list it enables** whenever the composition carries any GNOME
   setting at all, so the artifact says what it decided instead of leaning on whichever
   default the vendor shipped. (A composition with no GNOME intent writes no dconf
   database, and so no list: DeskOS does not touch a desktop nobody configured.) A UUID
   the platform lists in `enabledExtensions` is presumed already shipped and enabled:
   turning it **off** needs no catalog entry, because there is no package to turn away.
   A UUID the platform's catalog declares gets its package installed.
5. **Enabling is open.** Any UUID may be decided, including one the platform's catalog
   does not know: the extension the composition brings itself is installed by the
   packages the composition declares, so a workstation is not limited to the
   extensions DeskOS has modeled. The verification moves to where it belongs: the
   plan carries the extensions the composition turned on, and the **session check
   asks GNOME about each one**. A decision that does not really load is a red check,
   not a silent no-op. This is the same shape as the rest of the model: the
   composition declares, and the session check measures.
6. **The dock is an extension like any other.** There is no separate switch: the dock
   is enabled by deciding about its UUID, and the settings DeskOS models for it
   (`dock.position`, `dock.behavior`, `dock.iconSize`, `dock.showTrash`) apply only
   while that decision is true. The platform's catalog entry for `dash-to-dock` is
   what names the UUID whose decision gates those settings, and what installs the
   package. A higher-layer `false` masks lower-layer dock settings (a warning); dock
   settings with no enabling decision, or at or above the disabling layer, are errors.
7. On a platform that declares `gnome.keepsVendorDefaults` (ADR 0016), this field
   follows the same rule as every other GNOME setting: nothing is written, and a
   decision declared there **fails composition** with the platform's reason. A
   workstation whose platform ships a desktop of its own does not select the profile
   that carries the field, so the new one does not reopen the hole ADR 0016 closed.

### Configuring an extension

8. **Only what DeskOS models can be configured from the artifact.** An extension's
   settings are written by a typed adapter in the lowerer — the dock's is the one that
   exists — because an extension's setting can be a value the shell **executes**, and
   no type system can prove otherwise for an extension nobody modeled. A declaration
   that let a resource name dconf keys and values would be that escape hatch wearing a
   schema, so it is not added: a profile names a semantic setting, never a key, a
   path or a command.
9. An extension DeskOS does not model is **not left without a configuration**: it keeps
   its own defaults, and the user changes them in their session, which is where
   [ADR 0005](0005-managed-baseline-boundary.md) draws the line. What the artifact
   promises is what it declares; the rest belongs to the person at the machine.
10. When an option of an unmodeled extension must be part of the artifact, the way to
   add it is an adapter in the lowerer plus its test — a reviewed, typed change — not
   a new place for a resource to name dconf keys.

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
   a composition error: the two say the opposite about the same key. A `FlatpakSet`
   names its remote as a declared resource, so this covers it as well.
13. A remote the **base image itself** installs from is not visible to composition: the
   base's files are not resources. Retiring one of those is the workstation author's
   decision, which is what the mandatory `reason` exists to make reviewable. A broken
   first-boot install is a risk the session check observes when the unit fails, not a
   guarantee: this repository documents that `flatpak preinstall` can also exit zero
   without network. Closing the gap needs a platform fact listing the remotes a base
   ships; this ADR does not add one.
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
- `enabled-extensions` has **one writer in the plan**, and one switch: the dock and
  every other extension go through the same decision, so two writers cannot disagree
  and a manifest ordering cannot change the output. The plan states the effective
  list, and it names the extensions the composition turned on.
  One writer exists outside the plan: the experimental Shell stylesheet path writes
  `61-deskos-shell`, which repeats the whole list because dconf does not merge arrays
  and sorts after the plan's file, so whoever opts into it has to pass the effective
  list.
- The vocabulary grows by one key (the power button) and by the extension decisions,
  which any layer may take: a workstation is not limited to the extensions DeskOS
  modeled.
- A profile can name an extension, and only a typed adapter can configure one. A
  profile can never introduce a dconf key, a schema path or a command, and an
  extension DeskOS does not model keeps its own defaults for its user to change
  (ADR 0005).
- A retirement cannot silently force and cannot claim to remove applications that
  stay; the reason field makes the intent reviewable. A remote the base itself installs
  from is the one case composition cannot see, and it is named above rather than
  promised away.
- The two lists a workstation adds are refusals and decisions, not new software, so
  the boundary of ADR 0005 holds: DeskOS declares what the machine keeps, not what
  the user installs.
