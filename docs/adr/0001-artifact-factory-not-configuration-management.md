# ADR 0001: Artifact factory, not configuration management

**Status:** accepted

## Context

Organizations moving thousands of PCs to Linux need a *defined workstation*, not only a
distribution. Ansible Automation Platform, Satellite, Foreman and MDM products already enforce
state across a fleet.

## Decision

**DeskOS compiles organizational intent into bootc OCI artifacts and stops there.** It has no
controller, agent, reconciliation loop or fleet state. `/etc` drift on deployed machines is
expected and left to configuration management. Endpoint tooling observes (`status`) and acts only
when a person asks, once (`update`, `rollback`). A single requested action is not reconciliation,
and the endpoint holds no loop, schedule or automatic enforcement.

**Amended 2026-11-10.** `UpdatePolicy` may ship systemd timers that stage or apply images. They
are declarative image content, not a controller or a reconciliation loop: each run is a one-shot
command on a schedule, and the endpoint still holds no agent watching desired state. The
no-controller invariant is unchanged.

## Consequences

DeskOS stays small and composes with existing fleet tools. An organization that needs guaranteed
runtime enforcement must pair it with configuration management.
