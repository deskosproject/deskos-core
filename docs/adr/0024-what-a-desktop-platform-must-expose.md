# ADR 0024: What a desktop platform must expose

**Status:** proposed (2026-10-29)

## Context

DeskOS composes desktop intent into a typed Plan, and the vocabulary is closed: `GnomeProfile` fields lower to known
dconf keys, and there is no raw-key escape (ADR 0005). So "support another desktop" is not a configuration switch: it
is a kind, a lowering, a schema, a check and an audit.

Three things make that worth a decision. The model already assumes a desktop whose configuration is file-based and
layered; the shipped platforms are EL10, where GNOME is the vendor desktop; and reviewers keep asking why not another
desktop, with an answer that has stayed implicit.

The first audit of the GNOME System Administration Guide ([gnome-coverage.md](../gnome-coverage.md)) gives the
numbers: **seventeen** guide items are modelled exactly, three partially, and the refusals are the ones the invariants
force.

## Decision

**A desktop platform must expose its administrator configuration as files with explicit precedence and a lock layer,
and a check must be able to read the resulting state from the booted system.** Concretely, all of:

1. machine-wide configuration in files, with a documented precedence between system and user values;
2. a **lock** layer that wins over the user's own configuration;
3. a greeter surface with its own database, separate from the session's, for branding, the user list and messages;
4. a first-boot setup that can be preconfigured or skipped;
5. an extension or plugin policy the image can decide;
6. everything expressible **offline**, with no service, account or network;
7. state that a check can observe after boot, not only a file that was written.

**GNOME on the shipped platforms meets them today**, with the results in [gnome-coverage.md](../gnome-coverage.md):
the `user < local < site < distro` precedence and locks that beat `local` and `site` (verified on a built image), the
GDM database and greeter keys, the Initial Setup override, the extension catalog with per-UUID decisions, and the
instrumented session check that reads the defaults back.

**Another desktop is added the same way GNOME was**: its own kind, ADR, schema, lowering, its own coverage audit
against its own administrator guide, and checks in the VM harness. It is not a flag, a profile field or a second
`GnomeProfile` variant. A desktop that cannot meet criteria 1, 2 and 7 is not adopted until it can, whatever its
reach.

**Adoption is argued on these criteria and on measured results, never on the reputation of a desktop or its
community.** This repository does not rank desktops, and the documentation does not name competitors to argue against
them.

## Consequences

- `GnomeProfile` remains the only desktop kind. `Theme` and `BootProfile` stay GNOME-adjacent by their own ADRs, not
  by this one.
- A desktop contribution arrives with an audit like GNOME's, a kind and checks. That work is the cost of "why not X",
  and it is visible before anyone starts.
- The audit is a dated record. A new GNOME major release re-runs it; an item that stops being expressible is a defect
  in the model, not a footnote.
- Refusals stay refusals: autostart, custom sessions and kiosk mode are an `Exec` in a directory, and invariant 5 is
  the reason (see the audit).
