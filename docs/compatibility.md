# Compatibility

What stays stable between releases, what is rejected, and how a break is migrated.
The project is pre-1.0: this policy makes breaks deliberate and documented.

## Surfaces

| Surface | Promise |
|---|---|
| **Resource API** (`v1alpha1` YAML, fields and kinds) | public; unknown fields and kinds are rejected, never ignored; fifteen kinds, and a sixteenth needs a decision |
| **Plan** (`plan.deskos.org/v1alpha1`) | contract between compiler, backends and companion; a companion reads the plans its own release renders, and mixing releases is unsupported |
| **`deskosctl`** | command names, flags and exit codes stable; human text may change; `plan --format json` follows the Plan format and `sbom` follows CycloneDX |
| **`deskos`** | command names and exit codes stable; `report` and `check` text may change; `window` keeps its exit-code contract for systemd |
| **Image reference** | `:latest` moves; `:<commit>` is a naming convention, not immutable; a digest is exact; an installed machine follows the reference it was installed from |
| **Core and organization resources** | organizations add resources in their own root and never redefine Core's; duplicates follow ADR 0003 |

## Versioning and migration

- Releases are semver `vX.Y.Z` tags, and no tag moves.
- **Patch** releases fix defects and change no schema, Plan shape, command, flag or
  exit code.
- **Minor** releases may break pre-1.0 only with a migration note, a reviewed
  golden diff, and a fixture covering the old and new form.
- A schema break bumps `apiVersion`; a Plan-shape change bumps `format` and needs
  the matching companion. The compiler rejects an old input and does not guess a
  migration.
- A deprecation is announced here and in the release notes one minor before
  removal. None has happened yet.
- Nothing promotes itself: a tag, a `stable` alias and a release are human
  decisions.

## Fixture matrix

| Input | Class | Checked by |
|---|---|---|
| Core and the example organization compose and render | compatible | `make check`, `golden_test.go` |
| A higher layer overrides a scalar | compatible | `compose_test.go` (ADR 0003) |
| One keyed definition at several layers deduplicates | compatible | `compose_test.go` |
| An unknown field | rejected | `architecture_test.go` |
| A sixteenth kind | rejected | `architecture_test.go` (exactly fifteen) |
| Two different keyed definitions | rejected | `compose_test.go` |
| Two different scalars in one layer | rejected | `compose_test.go` |
| A renamed or removed field | migratable by edit | the release note gives old and new; the compiler rejects the old form |
| A Plan read by its own release's companion | compatible | the image ships its companion; `deskos report/check` tests |
| A Plan read by another release's companion | unsupported | stated here, no cross-release fixture |
| Invalid input to `deskosctl` | rejected | exit 1 |
| A usage error | rejected | exit 2 |
| `deskos window` outside its window | rejected | exit 1 |
| `deskos window` with bad arguments | rejected | exit 2 |

A golden update is a change under review, not a fix for a red test: run
`go test ./internal/compiler -update`, read the diff, explain it in the pull
request, and let a person merge it.

## Beta and 1.0

By decision, not by calendar.

**Beta** needs an organization outside the maintainer's own use running a pinned
release, two consecutive minors with the matrix green, a migration note for every
break in that window, and the support and security entry points named.

**1.0** needs beta for at least one minor with that external use, no resource or
Plan break for two minors, fixtures for the previous two passing in CI, and a
maintainer decision. Fourteen kinds is the ceiling unless a separate decision
changes it.

## What is not promised

- No LTS or vendor-lifecycle promise ([platforms.md](platforms.md)).
- No cross-release guarantee for the Plan or the companion.
- No bit-for-bit reproducible image: compilation is deterministic, the image build
  is not.
- No promise that a network input stays available; their checks are in
  [supply-chain.md](supply-chain.md#inputs-and-who-moves-them).
