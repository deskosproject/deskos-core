# ADR 0003: Explicit composition precedence

**Status:** accepted

## Context

Last-writer-wins makes results depend on file and list order and hides
disagreements between teams.

## Decision

Profiles declare a layer:
`foundation` < `organization` < `role` < `workstation`.

- **Sets** union.
- **Keyed** definitions must be identical.
- **Scalar** settings take the highest layer and conflict when one layer
  disagrees with itself.

**Order never matters.** All composed values keep provenance, and
conflicts name every contributor.

## Consequences

Organizations override DeskOS Core *deliberately*. Two teams at one layer
must resolve their disagreement explicitly instead of racing on file
names. There are no removal semantics in `v1alpha1`.
