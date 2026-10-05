# ADR 0009: TrustAnchor as the twelfth public kind

**Status:** accepted (2026-10-06)

## Context

Organizations run internal TLS: a corporate root CA, a private PKI, an
internal package mirror. Trusting a CA in the image means placing its
certificate in the platform trust store before any TLS client runs, so
`dnf`, `curl` and every browser that reads the system store accept the
organization's certificates from first boot. There is no runtime
configuration step and no user involvement.

None of the eleven kinds can hold this intent. `PackageSet` installs
packages; `BinaryArtifact` is restricted to executables in
`/usr/local/bin`; `FileInstall` exists only as lowered IR and is not a
public kind. On EL10 the trust store is populated by copying a PEM
certificate into `/etc/pki/ca-trust/source/anchors/` and running
`update-ca-trust`, which regenerates `/etc/pki/ca-trust/extracted/`.

## Decision

Add `system.deskos.org/v1alpha1` `TrustAnchor`:

| Field | Value |
|---|---|
| `anchors[].name` | the anchor file stem in the trust store |
| `anchors[].file` | a PEM X.509 certificate asset inside the resource root |

Each anchor is a **keyed** definition (key: `name`): identical
definitions deduplicate and different definitions for one name conflict at
any layer. The provider reads and hashes the asset and requires at least
one PEM `CERTIFICATE` block that parses as X.509. Only certificates are
accepted; a private key would not parse and fails the build.

The Platform supplies the trust-store facts (`trust.anchorsDir`,
`trust.updateCommand`), so Core stays platform-independent. Lowering
copies each anchor to `<anchorsDir>/<name>.crt` and emits one trust-store
update. On `rhel-10` and `centos-stream-10` that is
`/etc/pki/ca-trust/source/anchors/` and `update-ca-trust`.

The command is a platform fact: a bare command name with no arguments. No
resource field accepts a command, a destination path or a script.

A destination is never replaced silently: the Plan rejects an anchor that
shares a path with another image file, and the backend fails the build if
the base image or a package already provides that file. Each `file` asset
is confined to the resource root (no absolute paths, no traversal, no
symlink out of the root).

## Consequences

- The public API has **exactly twelve kinds**; a test enforces the count.
- A trust anchor is image content: it applies to every user, before any
  session, and travels with the deployment across image updates and
  rollbacks.
- This is not configuration management: DeskOS places the anchor in the
  image; an administrator or configuration management may still add or
  remove anchors at runtime.
- Requiring a PEM `CERTIFICATE` block rejects OpenSSL's `TRUSTED
  CERTIFICATE` format, which carries auxiliary trust data.
