# ADR 0011: The agentic contribution layer

**Status:** accepted (2026-10-06)

## Context

Vendor projects describe "agentic software factories": agents that triage issues, propose fixes and review
changes. Two things get conflated there. One is an **industrial** factory that produces hardened,
reproducible, attested artifacts ([ADR 0010](0010-artifact-evidence-and-supply-chain.md)); the other is an
**agentic maintenance** layer that proposes changes. `AGENTS.md` carries the contribution rules, but the
decision itself had no ADR.

## Decision

Keep the two layers separate. The agentic maintenance layer is **infrastructure around DeskOS**, never
part of it:

- **Not in compiler semantics.** No kind, provider, Plan IR or backend depends on a model. Compilation
  stays deterministic and works with no model, no network and no SaaS API (AGENTS.md invariant 11,
  CLAUDE.md "AI").
- **Propose, then a human decides.** The layer works on the repository, CI, the issue tracker,
  documentation and tests. No self-merge, no push to `main`, no signing, secrets, release or registry
  actions (AGENTS.md invariant 12).
- **Reuse, do not rebuild.** Prefer an orchestrator that already exists in a CNCF project over an agent
  runner written into this repository. The orchestrator must never become a compiler dependency.
- **Records, not telemetry.** Reports and feedback are user-owned and opt-in; DeskOS never phones home.

The contribution workflow (scope, branch and pull-request rules) lives in [`AGENTS.md`](../../AGENTS.md),
"Agentic contribution", and is the operating procedure this ADR formalizes.

## Consequences

- DeskOS may adopt or ignore the agentic layer without touching the compiler; the compiler is unaffected
  either way.
- The value of automation is **review capacity and records**, not unattended change. That boundary keeps
  the artifact factory (ADR 0001) deterministic and auditable.
- ADR 0010 is the industrial half; this ADR is the maintenance half.
