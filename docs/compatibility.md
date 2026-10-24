# Compatibility

What DeskOS promises to keep working between releases, what it rejects, and how
a change that cannot be kept is migrated. The project is pre-1.0: this policy
makes breaks deliberate and documented, not impossible.

## Surfaces

| Surface | Promise |
|---|---|
| **Resource API** (`v1alpha1` YAML, fields and kinds) | Public. Unknown fields and unknown kinds are rejected, never ignored. Exactly fourteen kinds; a fifteenth needs an explicit decision (ADR 0015 and successors) |
| **Plan** (`plan.deskos.org/v1alpha1`) | Internal contract between the compiler and the backends/companion, not a general API. The companion of a release reads the plans that release renders; mixing releases is unsupported |
| **`deskosctl`** | Command names, flags and exit codes are stable. Human-readable text may change; `plan --format json` follows the Plan format and `sbom` follows CycloneDX |
| **`deskos`** | Command names and exit codes are stable. Human-readable `report`/`check` text may change; `window` keeps its exit-code contract for systemd |
| **Image reference** | `:latest` moves; `:<commit>` is a naming convention, not registry-enforced immutability; a digest is exact. An installed machine follows the reference it was installed from until a person changes it |
| **Core and organization resources** | Organizations add resources in their own root; they never redefine Core's. Duplicate definitions follow ADR 0003 |

## Versioning and migration

- Releases are semver tags `vX.Y.Z`; no tag is moved.
- **Patch** releases fix defects. They do not change resource schemas, Plan
  shape, command names, flags or exit codes.
- **Minor** releases may make a pre-1.0 breaking change only with all of:
  a migration note, a reviewed golden diff, and a test or fixture covering the
  old and new form. Without those, the change waits.
- **Schema-incompatible** changes bump `apiVersion`; **Plan-shape** changes bump
  the `format` string and require the matching companion. The compiler may
  reject an old input with a message, but it does not guess a migration.
- **Deprecation** is announced in this page and the release notes at least one
  minor release before removal; the field keeps working until then. No field
  has been deprecated yet.
- **Automatic promotion does not exist**: a tag, a `latest` alias, or a release
  is a human decision. Nothing promotes a version on a green run.

## Fixture matrix

The classes are the contract; the tests are how it is enforced.

| Input | Class | Evidence |
|---|---|---|
| Core and the example organization compose and render | **compatible** | `make check`; `internal/compiler/golden_test.go` |
| A resource at a higher layer overrides a scalar | **compatible** | `internal/compiler/compose_test.go` (ADR 0003) |
| The same keyed definition at several layers deduplicates | **compatible** | `internal/compiler/compose_test.go` |
| An unknown field | **rejected** | `internal/compiler/architecture_test.go` (`unknown field`) |
| A fifteenth kind | **rejected** | `internal/compiler/architecture_test.go` (exactly 14) |
| Two different definitions of one keyed resource | **rejected** | `internal/compiler/compose_test.go` (`composition conflict`) |
| Two different scalars in one layer | **rejected** | `internal/compiler/compose_test.go` (`same layer conflict`) |
| A renamed or removed field | **migratable by edit** | no automatic migration exists; the release note gives the old→new mapping and the compiler rejects the old form |
| A Plan from release N read by release N's companion | **compatible** | the image ships its companion; `deskos report/check` tests |
| A Plan from release N read by another release's companion | **unsupported** | stated here; no cross-release fixture |
| `deskosctl` invalid input | **rejected** | exit 1 (`cmd/deskosctl/main_test.go`) |
| `deskosctl` usage error | **rejected** | exit 2 (same) |
| `deskos window` outside the allowed time | **rejected** | exit 1 (`cmd/deskos/window_test.go`) |
| `deskos window` bad arguments | **rejected** | exit 2 (same) |

Golden updates are changes under review, not a fix for a red test: run
`go test ./internal/compiler -update`, review the diff, and explain it in the
pull request. An agent never self-merges that diff.

## Promotion to beta and 1.0

Measured, not calendar-based; each gate is a human decision.

**Beta** requires all of:

1. at least one organization outside the maintainer's own use runs a pinned
   release in a documented resource root;
2. two consecutive minor releases with the fixture matrix green and no open
   compatibility defect;
3. a migration note for every breaking change in that window;
4. the support and security entry points named in P1-02/P2-01.

**1.0** requires beta for at least one minor release with that external use,
no resource or Plan break for two consecutive minors, fixtures for the previous
two minors passing in CI, and an explicit maintainer decision. Fourteen kinds is
the ceiling unless a separate decision changes it; promotion does not add kinds.

## Explicit non-promises

- No LTS or vendor-lifecycle promise; see [platforms.md](platforms.md).
- No cross-release guarantee for the Plan or the companion.
- No bit-for-bit reproducible image claim; compilation is deterministic, the
  image build is not yet bit-for-bit reproducible.
- No promise that network inputs stay available; their checks and gaps are in
  [supply-chain.md](supply-chain.md#inputs-and-who-moves-them).
