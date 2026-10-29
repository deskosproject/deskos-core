# AGENTS.md

Durable rules for any coding agent here. `CLAUDE.md` has the engineering rules
and `docs/adr/` the decisions; if this file disagrees with an ADR, the ADR wins
and this file is wrong.

## Invariants

1. **Artifact, not fleet.** DeskOS compiles workstation artifacts. No controllers,
   agents, reconciliation or runtime enforcement (ADR 0001).
2. **Typed Plan boundary.** Resources → validation → deterministic composition →
   typed Plan → backend. Backends read only `internal/plan`; never template YAML
   into a Containerfile (ADR 0002).
3. **Semantic layers.** `foundation < organization < role < workstation`. Sets
   union, keyed definitions must match, scalars take the highest layer and
   conflict within one layer. File, root and profile order never change output
   (ADR 0003).
4. **Exactly fourteen public kinds** in v1alpha1: Platform, Profile, Workstation,
   PackageSet, RpmRepository, AptRepository (ADR 0015), BinaryArtifact,
   FlatpakRemote, FlatpakSet, GnomeProfile, BootProfile (ADR 0006), UpdatePolicy
   (ADR 0007), TrustAnchor (ADR 0009), Theme (ADR 0013). A test enforces the count;
   adding a kind needs an explicit decision.
5. **No escape hatches.** No `script`, `command`, `postInstall` or other shell
   fields, no raw kernel arguments or Plymouth script themes, no templating, and
   nothing that manages a user's `$HOME` or personal tooling (ADR 0005).
6. **Each platform starts from its vendor's official base** (ADR 0004). A RHEL
   workstation is built `FROM registry.redhat.io/rhel10/rhel-bootc`, never from a
   CentOS-derived DeskOS image. RHEL-derived images are never published or
   redistributed. A platform whose vendor publishes no bootc base is added only as
   an organization-owned base, with a named owner and a support tier (ADR 0014).
7. **Core and organizations stay separate.** Core carries no organization or role
   software; organizations extend it from their own roots without copying it.
8. **Defaults are not locks.** Defaults may be overridden by users and by runtime
   configuration management. A lock is image-level enforcement that also beats
   `local`/`site` dconf; Core never locks.
9. **Platform gaps are reported, not hidden.** A requirement the platform cannot
   meet fails composition with a reason; never substitute silently.
10. **Say only what was verified.** Compilation is deterministic; images are not
    yet bit-for-bit reproducible. RHEL composition and rendering are covered by
    tests; the entitled RHEL build and boot were validated once on the retired
    factory host, and that validation now runs in a downstream organization's
    repository. The workstation E2E is partial: one AT-SPI step on CS10
    (`docs/architecture.md`, "Validation status").
11. **AI is not part of compiler semantics.** Compilation works with no model, no
    network and no SaaS API, and identical inputs keep producing identical plans
    and contexts. No kind, provider, Plan IR or backend may depend on a model
    (CLAUDE.md, "AI").
12. **Human-gated automation.** Automation may propose; a human approves. Agents
    never self-merge, never push to `main`, never touch signing, secrets, releases
    or registries, and never weaken a check to go green.

## Language

English throughout: code, comments, docs, commits, pull requests, issues and
review. Translate text you touch rather than adding more in another language.
Never name a downstream organization; write "a downstream organization" or "the
organization".

## Docs

Follow [docs/style.md](docs/style.md): one question per page, the current fact,
60–150 lines, no filler, every claim with its evidence. `make check` runs the
link checker.

## Commits and pull requests

[Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

    type(scope): imperative summary

- **Types:** `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `build`, `ci`,
  `chore`, `revert`.
- **Scope:** optional but preferred, a noun from the area changed.
- **Summary:** lowercase after the colon, imperative, no period, 50 characters,
  never over 72.
- **Body:** blank line, then what changed and why at 72 columns. The diff shows
  the mechanics.
- **Footers:** one blank line before `Refs: #123` or `Closes: #123`.
- **Breaking changes:** `!` on the type or scope and a `BREAKING CHANGE:` footer.

The pull-request title uses the same form, because a squash merge makes it the
commit on `main`. The body follows
[the template](.github/pull_request_template.md). Never a vague title.

## Design

Avoid unnecessary labels and descriptions. Let the design do the explaining.

## Requires maintainer approval

Publishing or pushing images, tags or commits; creating releases; changing the
license (Apache-2.0); handling credentials; changing shared infrastructure.

## Checks

Core workflows run on **GitHub-hosted standard runners only**. A self-hosted or
factory runner is never a fallback: it would change the trust boundary. An
organization may run its own pipeline; that is not a Core workflow.

    make check        # gofmt, go vet, go test, validate, links, style
    python3 -m unittest discover -s tests/vm   # VM harness; CI runs it, make check does not
    go test ./internal/compiler -update   # refresh goldens, then review the diff

Never run `-update` merely to make a red check pass: a golden diff is a change
under review.

## Agentic contribution

A patch may be authored or assisted by an agent under the same rules as any other
patch: review, tests, human decision. The value is review capacity and evidence,
not unattended change.

**The split.** Agents operate on the repository, CI, the issue tracker, docs and
tests. They do **not** operate on compiler semantics: no kind, provider, Plan IR
or backend may depend on a model (invariant 11).

**Start low-risk.** Triage, labelling, documentation, corrections under
`resources/**`, flaky-test fixes, and pin bumps with checksums. Architecture, new
kinds, signing, secrets, releases and registries are out of scope for automation.

**Workflow.**

- Work from a scoped, labelled issue; if it is unclear, ask.
- Branch from `main`; never commit or push to `main`. The branch is protected by
  a pull request and a green `validate` check, administrators included.
- Open a PR with `make check` and the golden diff green.
- A human reviews and merges. An agent never self-merges and never substitutes for
  a maintainer's approval.
- Never `Co-authored-by:`. A human reviews and owns every change.

**Records, not telemetry.** Reports are user-owned and opt-in; DeskOS never
phones home and never aggregates user data centrally.

**Reuse before building.** Prefer an orchestrator that already exists in a CNCF
project over an agent runner written into this repository. It is infrastructure
around DeskOS and never a compiler dependency.

This section is the operating procedure of
[ADR 0011](docs/adr/0011-agentic-contribution-layer.md).
