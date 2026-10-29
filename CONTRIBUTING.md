# Contributing

DeskOS compiles workstations from declared intent. A patch is welcome whether a
person or an agent wrote it, under the same rules: review, tests, a human
decision.

## What this repository is, and what it is not

- **It compiles artifacts.** Resources → validation → deterministic composition
  → a typed Plan → a backend. There is no controller, agent or reconciliation
  loop, and a patch that adds one is out of scope (ADR 0001).
- **The compiler stays deterministic and offline.** Same resources, same Plan.
  Nothing in a resource, provider, Plan or backend may depend on a model or the
  network (ADR 0002, AGENTS.md).
- **Core carries no organization's software.** An organization extends Core from
  its own resource root.

A good first contribution is a correction under `resources/**`, a documentation
fix, a test, or a pin bump that carries its checksum.

## Before you write code

1. **Open an issue** that says what you want and why, unless the change is a typo
   or a broken link.
2. **For a decision, an ADR.** A new resource kind, a new backend, or anything
   that changes the public model needs an ADR under `docs/adr/` and a discussion
   before code. Adding a kind also needs an explicit decision: fourteen is the
   ceiling in v1alpha1.
3. **Small and bounded beats large and surprising.** One change, one reason.

## Reproduce the checks

```bash
make check                        # gofmt, vet, go test, validate
python3 -m unittest discover -s tests/supply-chain   # and tests/vm, tests/lifecycle, tests/artifacts, tests/workflows
go test ./internal/compiler -update                  # refresh goldens — then REVIEW the diff
```

`make check` does not run the VM harness; CI does. A golden diff is a change
under review, not a fix for a failing test: if you cannot explain every line of
the diff, the change is not ready. See [development.md](docs/development.md) and,
for what each workflow proves, [checks.md](docs/checks.md).

## Commits and pull requests

[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):
`type(scope): imperative summary`, 50 characters ideally and 72 at most, English
in code, docs, commits and reviews. Branch from `main`; **never push to `main`**,
which is protected. The pull request follows
[the template](.github/pull_request_template.md): summary, changes, testing and
risks.

A human reviews and merges. An agent may author a patch and may not self-merge,
touch signing or secrets, or weaken a check to make it pass.

## AI-assisted contributions

Allowed, and verified like any other patch. Say so when it matters; the reviewer
will check the same things: does it compile, is it deterministic, does it keep
the compiler free of models and network, and does the evidence match the claim.
The model is never part of the compiler's behaviour.

## Where each concern goes

| Concern | Where |
|---|---|
| A security vulnerability | **Privately**, through [SECURITY.md](SECURITY.md). Never a public issue. |
| A bug or a feature | An [issue](https://github.com/deskosproject/deskos-core/issues) with what you ran and what happened |
| A question about using or building a machine | [Discussions](https://github.com/deskosproject/deskos-core/discussions) |
| Support, and what is promised | [support.md](docs/support.md): best effort, no SLA |

## Conduct

Be respectful of the person, direct about the work. Disagree with a design and
say why; do not make it personal. Harassment, discrimination and disclosure of
private information are grounds for removal from the conversation.

Report a conduct concern to the maintainer through the same private channel as a
security report if you would rather not raise it in public. There is one
maintainer; the project does not pretend to have a committee.

## Supported versions

Before 1.0 there is no LTS and no backport: the current release is supported, and
[migrations.md](docs/migrations.md) says what to edit when you move. A published
image digest keeps its evidence; it does not keep a promise of fixes.
