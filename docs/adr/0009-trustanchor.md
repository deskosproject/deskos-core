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
`/usr/local/bin` (with the named exception of the runtime companion at
`/usr/bin/deskos`, ADR 0018); `FileInstall` exists only as lowered IR and is not a
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

## Update (2026-10-23): distrusting a CA

**Status:** accepted.

### Context

In October 2026 the private keys of eight eMudhra intermediates were
recovered; six issued TLS certificates under `emSign ECC Root CA - G3`
and the Sectigo `AAA Certificate Services` cross-sign, which RHEL 10's
`ca-certificates` still shipped (RHSB-2026-012). The vendor fix is a new
package. Until a platform's base or compose carries it, a descent system
trust store accepts certificates the root programs have already rejected.

The vendor's own interim mitigation is to blocklist the roots. p11-kit
reads `/etc/pki/ca-trust/source/blocklist/` and `update-ca-trust` leaves
those certificates out of every extracted bundle. That is a typed,
build-time operation, but `TrustAnchor` could only say *trust this CA*:
there was no way to express *do not trust this CA* without an arbitrary
file install or a shell step.

### Decision

`TrustAnchor.spec` gains an optional `blocklist[]` with the same
`name`/`file` shape, keying and asset rules as `anchors[]`:

- a certificate is either anchored or distrusted, never both: one `name`
  may appear in only one list;
- the Platform supplies `trust.blocklistDir`; a platform that has anchors
  but no blocklist directory fails composition when a `blocklist` entry is
  requested, rather than dropping it;
- lowering copies each certificate to `<blocklistDir>/<name>.crt` and emits
  the same single `trust.updateCommand` run after `COPY rootfs/`;
- the Plan carries `trustBlocklist` next to `trustAnchors`, and the SBOM
  records each entry with `deskos:source=trust-blocklist`.

The field is a **profile resource like any other**, so an organization can
declare its own distrusted certificates at any layer, including the
workstation layer. Core ships `deskos-trust` with the eMudhra roots for
the workstations that select it. The kind count stays fourteen.

### Consequences

- Removing trust is image content, and it travels with the deployment
  across updates and rollbacks.
- It is not a lock: an administrator may still add the CA at runtime, and
  an entry stops mattering as soon as the platform's package stops
  shipping that root. The list is meant to be emptied when it does.
- A platform whose trust store cannot express distrust (no p11-kit
  blocklist) cannot select the profile; it is a named composition gap, not
  a silent omission.
- Distrusting a root also rejects legitimate certificates that chain to
  it, exactly as the browser root programs already do; that is the
  intended trade, and it is why the entry is tied to a vendor advisory
  rather than a preference.
