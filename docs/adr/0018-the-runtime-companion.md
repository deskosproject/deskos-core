# ADR 0018: `deskos`, the runtime companion to `deskosctl`

**Status:** accepted (2026-10-18)

## Context

`deskosctl` builds an artifact before the image exists, with no network, privileges or model. A machine built
from that artifact has the opposite need: to be told what it was built from, to have its drift measured in the
terms the artifact declares, and to be updated. A catalog of recipes that run commands on the machine does not
answer that: recipes that write to `/etc` and to a home do not come back with a `bootc rollback`, and a recipe
catalog is a distribution's job (ADR 0001, ADR 0005). [ADR 0010](0010-artifact-evidence-and-supply-chain.md)
left stage 4 open: the plan travels with the image, the SBOM does not.

## Decision

**A separate binary, `deskos`, for the machine.** It carries no compiler, needs no network, and its three
commands have three different promises.

1. **`deskos report` describes the artifact.** It reads `/usr/share/deskos/plan.json` and says what the machine
   was built from: the workstation, its platform, the base image by digest, the profiles, the packages, the
   GNOME defaults and the extensions the composition enabled, and the committed deployment it runs now. It
   distinguishes what is installed from what is only declared: the SBOM is CI evidence and is not on the
   machine, so a report never describes installed content from the declared one.
2. **`deskos check` compares a declared scope with the present state**: the files the plan installs and
   generates, by digest, and the dconf defaults the plan declares, as the session sees them. It **publishes its
   scope** — what it leaves out (`/etc` and `/var` changes, packages installed later, package-owned files, unit
   runtime state, whether the running image matches the plan). A value it cannot read is reported as unmeasured
   with the reason, never as a difference.
3. **`deskos update` updates the image with a guard.** The guard acts only on what prevents a safe update — this
   is not a bootc deployment, bootc cannot read the deployment — and reports the rest: an image that updates
   itself with its own timer, and what the plan owns under `/etc`. It does not reconcile `/etc` and does not
   block an update for it (invariant 8). It changes the deployment the machine boots from and nothing else, never
   touches a home, and stages by default, applying only when asked.

**What it does not have:** no recipe catalog, nothing that writes to a user's home, no layer crossing, no
reconciliation against the plan, no network of its own beyond `bootc`, and no dependence on a model.

### How it gets onto a machine

`deskos` is a **pinned `BinaryArtifact` in Core's foundation**: every Core image installs `/usr/bin/deskos` from a
published release asset with its checksum. It is the one binary the schema lets a `BinaryArtifact` place in
`/usr/bin` (every other stays under `/usr/local/bin`), because `sudo` searches `/usr/bin` through its
`secure_path` and `sudo deskos update` must work without a full path. The path is fixed by the schema (amended
2026-10-21). v0.12.0 is the first release that publishes the asset: `resources/software/deskos.yaml` names it and
its checksum, `deskos-core` references it, and the binary is part of the declared SBOM.

## Consequences

- The logic lives in `internal/evidence`, which takes a `Probe` and never executes a command, and the machine's
  access lives in `cmd/deskos`, so nothing under `internal/` runs a process.
- The two binaries can be shipped apart: `deskosctl` renders images, `deskos` is what a machine carries. The
  release builds both with their checksums in `SHA256SUMS`.
- `deskos update` acts on the machine it runs on and takes no `--root`, because reading one tree's plan while
  asking the host's bootc to upgrade would mix two machines. Only read-only commands accept a root.
- `check` cannot see drift it does not own and says so on every run; the guard refuses only conditions that make
  an update unsafe, so a machine whose owner changed files under `/etc` is never held back and never reverted.
- The pin pins bytes and nothing more: the checksum says the asset is the one published and reviewed, not that
  it was built from this source tree, and the release signs the `deskosctl` tool image, not this binary.
- ADR 0010 stage 4 stays half done: the reporting command exists; the SBOM still does not travel with the
  artifact.
