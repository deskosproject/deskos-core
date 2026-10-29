# ADR 0002: Declarative resources and a typed IR

**Status:** accepted

## Context

Templating YAML into Containerfiles ties the public API to one backend, invites embedded shell, and
makes composition depend on order.

## Decision

Public configuration is **versioned resource envelopes** (`apiVersion`, `kind`, `metadata`, `spec`)
under `*.deskos.org`, validated by JSON Schema and by typed decoding. No loops, templates,
conditionals or shell fields. Providers, dispatched by GVK, lower composed intent into a **typed
Plan**. Backends read only the Plan. The envelope design follows Kubernetes; no Kubernetes
machinery is used.

## Consequences

The IR is inspectable (`deskosctl plan --format json`), deterministic and backend-independent.
Every new capability needs a *typed abstraction* rather than an escape hatch.
