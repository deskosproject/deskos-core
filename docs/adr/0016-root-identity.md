# ADR 0016: A root declares its name, instead of inheriting the directory's

**Status:** accepted (2026-10-17)

## Context

A resource root is a directory of documents that a workstation composes. When
`deskosctl` loads one, it records where each resource came from: `Source.Root` is
the **base name of the directory** the root was read from, and that path travels
into the provenance of the Plan (`internal/loader/loader.go:93`,
`internal/model/model.go:52-58`). The Plan is serialized and installed inside the
image as `/usr/share/deskos/plan.json`, which ADR 0010 makes part of the
artifact's evidence.

So an image depends on a filesystem detail. A root with identical content, cloned
as `daytwo-resources` or as `acme-workstations`, produces a different Plan and
therefore a different image — while the repository promises that identical
semantic inputs keep producing identical plans and build contexts (`AGENTS.md`,
"Invariants"; `CLAUDE.md`, Determinism). CI cannot see it: it renders the same
paths twice on the same runner (`ci.yml`, "render twice and compare").

Two things hid it. The embedded Core materializes into `<tmp>/resources`, so its
name is always `resources`, and the paths CI uses are fixed. The leak shows up
exactly where it matters most — an organization's own resource tree, cloned
anywhere under any name.

## Decision

1. **A root declares its name in a file.** `.deskos-root`, at the root's top
   level, holds one line: the name. It is not a resource document, it is not a new
   kind, and the loader does not parse it as one.
2. **A root that declares nothing is `root`** — the literal, never the directory
   name. Existing invocations keep working; the cost is that an undeclared root is
   anonymous in provenance, which is a statement about it rather than a guess.
3. **The shipped Core and the shipped examples declare theirs** (`resources`,
   `baseline-and-role`, …), so the plans that exist today do not change: the
   goldens stay byte-identical and the fix is proved by their absence from the
   diff.
4. **A test renders one root under two directory names** and requires identical
   output. That is the check CI could not perform before.

## Consequences

- Provenance stops depending on where a tree happens to live. An organization
  names its root, and that name is part of its intent, not an accident of the
  checkout.
- A root that declares nothing is `root` in every plan. If two undeclared roots
  contribute, provenance cannot tell them apart: ordering still falls back to the
  path, so output stays deterministic, but the evidence is weaker. Naming them is
  the fix, and the file is right there.
- The ordering tiebreak (`Source.Root`, then `Path`) now uses a declared name. Two
  roots declaring the same name are ordered by path — deterministic, and a sign
  that they should be named differently.
- Nothing else in the Plan changes: no new field, no new kind, no new flag.
