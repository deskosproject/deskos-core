# ADR 0001: Artifact factory, not configuration management

Status: accepted

## Context

Organizations moving thousands of PCs to Linux need a defined workstation,
not only a distribution. Runtime enforcement across a fleet is already
solved by Ansible Automation Platform, Satellite, Foreman and MDM products.

## Decision

DeskOS compiles organizational intent into bootc OCI artifacts and stops
there. It has no controller, agent, reconciliation loop or fleet state.
Mutable `/etc` drift on deployed machines is acknowledged and left to
configuration management. Future endpoint tooling may observe (`status`,
`diff`) but never enforce.

## Consequences

DeskOS stays small and composes with existing fleet tools. An
organization that requires guaranteed runtime enforcement must pair it
with configuration management.
