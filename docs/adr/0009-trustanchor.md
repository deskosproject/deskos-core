# ADR 0009: TrustAnchor as the twelfth public kind

**Status:** accepted (2026-10-06)

## Context

Organizations run internal TLS, and trust must sit in the platform store before any TLS client runs, so `dnf`, `curl`
and browsers accept the certificates from first boot. No kind can hold it: `BinaryArtifact` allows only executables in
`/usr/local/bin` (except `/usr/bin/deskos`, ADR 0018), and `FileInstall` is lowered IR. On EL10 a PEM certificate in
`/etc/pki/ca-trust/source/anchors/` plus `update-ca-trust` works.

## Decision

Add `system.deskos.org/v1alpha1` `TrustAnchor`, with `anchors[].name` (the anchor file stem in the trust store) and
`anchors[].file` (a PEM X.509 certificate asset inside the resource root). Each anchor is a **keyed** definition
(key: `name`): identical ones deduplicate, and two different ones for a name conflict at any layer. The provider
hashes the asset and requires a PEM `CERTIFICATE` block that parses as X.509.

The Platform supplies `trust.anchorsDir` and `trust.updateCommand`, so Core stays platform-independent. Lowering
copies each anchor to `<anchorsDir>/<name>.crt` and emits one trust-store update; on `rhel-10` and `centos-stream-10`
that is `/etc/pki/ca-trust/source/anchors/` and `update-ca-trust`. The command is a platform fact: a bare name with no
arguments, and no field accepts a command, a path or a script.

A destination is never replaced silently: the Plan rejects an anchor that shares a path with another image file, and
the build fails if the base or a package already provides it.

## Consequences

- The public API has **fifteen kinds** today (twelve when this decision was taken); a test enforces the count.
- A trust anchor applies to every user, before any session, and travels with the deployment across updates and
  rollbacks; an administrator may still add or remove anchors at runtime.

## Update (2026-10-23): distrusting a CA

### Context

In October 2026 the private keys of eight eMudhra intermediates were recovered; six issued TLS certificates under
`emSign ECC Root CA - G3` and the Sectigo `AAA Certificate Services` cross-sign, which RHEL 10's `ca-certificates`
still shipped (RHSB-2026-012). The vendor fix is a new package, and until a base carries it the store accepts what the
root programs rejected. p11-kit reads `/etc/pki/ca-trust/source/blocklist/`, and `update-ca-trust` leaves those
certificates out.

### Decision

`TrustAnchor.spec` gains an optional `blocklist[]` with the same `name`/`file` shape, keying and asset rules as
`anchors[]`. A certificate is either anchored or distrusted, never both: one `name` appears in only one list. A
platform supplies `trust.blocklistDir`, and one without it fails composition rather than dropping the entry. Lowering
copies each certificate to `<blocklistDir>/<name>.crt` and runs the same single `trust.updateCommand` after `COPY
rootfs/`; the Plan carries `trustBlocklist` next to `trustAnchors`, and the SBOM records
`deskos:source=trust-blocklist`.

The field is a **profile resource like any other**, so any layer may declare a distrusted certificate; Core ships
`deskos-trust` with the eMudhra roots, and the kind count does not change. It is not a lock: an administrator may still
add the CA at runtime, and an entry stops mattering once the platform stops shipping that root.

### Consequences

- A platform without a p11-kit blocklist cannot select the profile; that is a named composition gap. Distrusting a
  root also rejects certificates that chain to it, as the root programs already do.
