# ADR 0018: `deskos`, the runtime companion to `deskosctl`

**Status:** accepted (2026-10-18)

## Context

`deskosctl` builds an artifact: it reads resource roots, composes a typed Plan and
renders a deterministic build context. It runs with no network, no privileges and no
model, and everything it touches it touches before the image exists.

A machine that was built from one of those artifacts has the opposite need: to be told
what it was built from, to have its drift measured in the terms the artifact declares,
and to be updated. Nothing in the repository did that, and the temptation to solve it
the way `ujust` does — a catalog of recipes that run commands on the machine — is
exactly what DeskOS declines: recipes that write to `/etc` and to a home do not come
back with a `bootc rollback`, and a recipe catalog is a distribution's job, not an
artifact factory's (ADR 0001, ADR 0005).

[ADR 0010](0010-artifact-evidence-and-supply-chain.md) left stage 4 open: "evidence
travels with the artifact: ship the SBOM beside `plan.json`; an endpoint command
reports it". The plan travels today; the SBOM does not.

## Decision

**A separate binary, `deskos`, for the machine.** It carries no compiler, needs no
network, and its three commands have three promises, which are not the same promise
three ways.

1. **`deskos report` describes the artifact.** It reads the plan the image carries
   (`/usr/share/deskos/plan.json`) and says what the machine was built from: the
   workstation, its platform, the base image by digest, the profiles, the packages, the
   GNOME defaults and the extensions the composition enabled, and the committed
   deployment it is running now. It **distinguishes what is installed from what is only
   declared**: the plan is installed evidence, the SBOM is CI evidence shipped with the
   release and is not on the machine, so a report never describes installed content
   from the declared one. That is stage 4's command, with the SBOM's journey still
   pending.
2. **`deskos check` compares a declared scope with the present state** and enumerates
   the differences: the files the plan installs and generates, by digest, and the dconf
   defaults the plan declares, as the session sees them. It **publishes its scope**:
   what it looks at and what it leaves out (`/etc` and `/var` changes, packages
   installed later, package-owned files, the runtime state of units, whether the running
   image is the one the plan was rendered for). A check that listed only what it covers
   would read as if it measured everything. A value it cannot read on this machine is
   reported as unmeasured with the reason, never as a difference: a value nobody read is
   not a value that drifted.
3. **`deskos update` updates the image with a guard.** The guard acts on what prevents
   a safe update — this is not a bootc deployment, bootc cannot read the deployment —
   and only **reports** the rest: an image that updates itself with its own timer, and
   what the plan owns under `/etc`. A machine a user customized is still a machine they
   may update; DeskOS does not reconcile `/etc` and does not block an update for it
   (invariant 8, "defaults are not locks"). It is an action someone asked for, not a
   reconciler: it changes the deployment the machine boots from and nothing else, it
   never touches a home, and it stages by default, applying only when asked.

**What it does not have:** no recipe catalog, nothing that writes to a user's home,
nothing that crosses layers, no reconciliation of the machine against the plan, no
network of its own beyond what `bootc` does, and no dependence on a model. The
compiler stays the only thing that decides what an artifact is.

### How it gets onto a machine

`deskos` is a **pinned `BinaryArtifact` in Core's foundation**, the same shape a flavor
uses for any verified binary, so every Core image installs `/usr/local/bin/deskos` from
a published release asset with its checksum, and the pin can be watched and bumped
deliberately like a base image's digest. (`/usr/local/bin`, not `/usr/bin`: the model
lets a `BinaryArtifact` write only there, because `/usr/bin` belongs to the image and
its packages.) Two alternatives were considered and rejected:

- committing a built binary into the repository would put a build artifact in git and
  let it drift from the source it was built from;
- building it inside the image does not reach the consumers who render from the
  resources tarball, because they build without this source tree.

A pinned asset has to exist before a pin can point at it, so the pin lands with the
first release that ships the binary. Until then the images do not carry it, and that is
stated rather than assumed: a release that ships `deskos` is what unblocks the pin,
after which every Core image carries it by default.

## Consequences

- The logic lives in `internal/evidence`, which takes a `Probe` and never executes a
  command, and the machine's access lives in `cmd/deskos`: the repository's structural
  rule that nothing under `internal/` runs a process keeps the two apart, and the
  checks are exercised against a machine a test describes.
- The two binaries answer two questions and can be shipped apart: `deskosctl` is what a
  flavor pins and runs to render its image, `deskos` is what a machine carries. The
  release builds and attaches both, with their checksums in `SHA256SUMS`.
- `deskos update` acts on the machine it runs on and takes no `--root`: reading one
  tree's plan while asking the host's bootc to upgrade would mix two machines. It is
  read-only commands that accept a root, for images and tests.
- **Every Core image carries `deskos`**, from a pinned release asset (above), so a
  machine can always be asked what it was built from without anyone having to install a
  tool first. The pin lands with the first release that ships the binary.
- `check` cannot see drift it does not own, and it says so on every run. That is the
  honest version of "measure the drift": `ujust`'s `check-local-overrides` admits the
  gap, and this admits it in writing.
- The guard refuses only on conditions that make an update unsafe, so a machine whose
  owner changed files under `/etc` is never held back and never silently reverted.
- ADR 0010 stage 4 is half done: the command that reports evidence exists; the SBOM
  still does not travel with the artifact, and until it does, a report repeats that
  rather than implying otherwise.
- The binary is not a place to grow a distribution: every promise it makes is about the
  artifact the machine was built from, and the answer to "I want feature X on my
  machines" stays where it belongs — a resource, composed by `deskosctl`.
