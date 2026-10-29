# Consumer-side image authenticity

**Status: decided, 2026-10-25.** Core signs keyless with OIDC: no key file, no
consumer enforcement, manual verification. An organization that chooses
enforcement keeps a key outside its repository and a key-based policy, so `bootc`
can reject an unsigned update. The key-based recipe is verified in the lab; that
organization's policy, installer and key-rotation paths are not. This document
changes no Core policy or key. CI verifies signatures with `cosign`: the gate
checked a digest, but no installer or installed workstation was shown to reject
an unsigned image.

## What is true today

- Core images are signed **keyless** with cosign: Fulcio issues a certificate
  for the workflow's OIDC identity and Rekor records the signature. Three
  attestations are attached (SBOM reference, reviewed exceptions, accepted
  matches).
- Organization-owned RHEL images are signed with a **key** kept outside its
  repository, with no transparency log.
- No generated **Core** image ships a `/etc/containers/policy.json`, so the
  containers/image default accepts an unsigned image.
- `bootc` honors `containers-policy.json` when fetching images; its upstream
  documentation says the default does not require signatures, and that this is
  not a bootc vulnerability.
- `bootc install`/`switch` and the install config expose
  `--enforce-container-sigpolicy` / `enforce-container-sigpolicy`, which only
  requires the policy's **default** to demand signatures, not the identity.

## Core: keyless, manual verification

Core's signature is evidence, not enforcement. The release page must publish the
digest and the workflow identity; a person verifies before installing or
updating, and `bootc` still accepts an unsigned image.

## Candidate mechanisms

| Mechanism | What it can require | Limits to test |
|---|---|---|
| `signedBy` (GPG simple signing) | a GPG key over the image | cosign does not produce this signature format; not a match for the current signatures |
| `sigstoreSigned` with `keyPath`/`keyData` | a cosign signature by a named public key | Core is keyless; an organization that enforces already has a key. Verification and policy provisioning are local/offline |
| `sigstoreSigned` with `fulcio` | a Fulcio certificate from an expected OIDC issuer, constrained to a subject (the documented policy field is an email) | **the workflow identity we use is a GitHub URL, not an email.** containers/image cannot express it; a repository-wide identity regex would be too broad for release authorization. Core therefore does not enforce keyless signatures |
| `bootc --enforce-container-sigpolicy` | that the policy is not `insecureAcceptAnything` | it does not check which identity is allowed; the policy must do that |
| Installer/Anaconda `ostreecontainer` | unclear | test with a requiring policy; if it does not enforce, the install path needs a separate control |
| A `deskos` wrapper or hook that calls `cosign verify` | exact issuer/identity | a command, not enforcement of every pull; acceptable only if it cannot be bypassed and covers install, switch and update |

## Provisioning and recovery

A policy is part of the image, which is circular: a compromised release would
carry the next trusted policy. An enforcing organization must answer:

- where the policy and public trust roots live (image `/etc`, local
  configuration, or an installer-provided file);
- how a policy or key rotation reaches machines after a compromise **without
  trusting the compromised release**;
- what an offline machine can verify without a live Fulcio/Rekor fetch;
- the emergency path if a trusted identity must be distrusted, and recovery of a
  machine that already staged the release.

## Working recipe and lab result (2026-10-25)

cosign 3's default bundle is an OCI index this containers/image does not fetch as
an attachment; the legacy referrers format with `matchRepository` works.

**Image policy** (`/etc/containers/policy.json`), with the public key baked into
the image:

```json
{"default":[{"type":"sigstoreSigned",
            "keyPath":"/etc/containers/cosign.pub",
            "signedIdentity":{"type":"matchRepository"}}]}
```

**Signature discovery** (`/etc/containers/registries.d/<name>.yaml`):

```yaml
docker:
  registry.example:5000:
    use-sigstore-attachments: true
```

**Signing** with cosign 3, then switch or upgrade normally:

```bash
cosign sign --key cosign.key \
  --new-bundle-format=false --use-signing-config=false --tlog-upload=false \
  --registry-referrers-mode=legacy --yes \
  registry.example:5000/os/image@sha256:<digest>
```

**Results on bootc 1.17.1, containers/image from Podman 5.8.9, cosign 3.1.3**
(the enforced route; Core does not use it).

| Case | Result |
|---|---|
| signed with the pinned key, policy `sigstoreSigned` + `matchRepository` | **accepted and staged** (rc 0); `--enforce-container-sigpolicy` also rc 0 |
| same, unsigned image | rejected: “A signature was required, but no signature exists” |
| same, signed by another key | rejected: “cryptographic signature verification failed” |
| signature identity `127.0.0.1:5000/…`, image pulled as `registry.lab:5000/…` | rejected: identity not accepted |
| no policy, `--enforce-container-sigpolicy` | **accepted the unsigned image** (rc 0) |
| `default: reject` | rejected |
| `bootc upgrade` to a newly signed digest | **accepted and staged** |
| `bootc upgrade` to an unsigned digest | rejected: “A signature was required, but no signature exists” |
| cosign bundle copied into the local sigstore store | rejected: unrecognized signature format |

Key-based enforcement works **only when all three pieces are present**: a policy
naming the key and identity, `use-sigstore-attachments`, and a signature in the
legacy referrers format that this containers/image reads.
`--enforce-container-sigpolicy` alone is no control.

**Key-based policy and keyless signing are different things, not two steps of
one migration.** An image shipping a `cosign.pub` and a `sigstoreSigned` policy
for its key-based predecessor gets no enforcement on its keyless tags: the policy
names a public key, and there is a Fulcio certificate instead.

**Upstream status (2026-10-25).** containers/image's `fulcio` policy accepts
`oidcIssuer` and `subjectEmail`, not a URI SAN. The pull request for URI matching
([containers/image#2235](https://github.com/containers/image/pull/2235)) was
closed unmerged; BYOPKI (`pki`) covers a custom CA, not Fulcio, and sigstore
bundle support is still open
([container-libs#388](https://github.com/podman-container-tools/container-libs/issues/388)).

## Enforced prototype plan (disposable lab)

Build a scratch image carrying a `sigstoreSigned` policy for a test key, sign a
test image with that key, and prove an unsigned image, a wrong key and a tampered
manifest are rejected on `bootc switch` and `bootc upgrade`. Repeat with
`bootc --enforce-container-sigpolicy` and an `insecureAcceptAnything` default.
Each case needs a positive and a negative result, a recorded digest, the bootc
version and the containers/image version.

Not yet tested: the Anaconda `ostreecontainer` install path, offline key-based
verification, key rotation and lost-key recovery. Keyless Core enforcement is not
a pending test: containers/image cannot pin a GitHub URI SAN.

## Decisions

- **Decided 2026-10-25:** Core signs keyless with OIDC — no key file, no
  consumer enforcement, manual verification. An organization that chooses
  enforcement keeps key-based signing; custody, rotation and backup are
  downstream work (P0-04).
- How an enforcing organization bakes the policy into the image or provisions it
  from the installer, and how a compromised-release policy update is delivered.
- Whether a DeskOS-specific verifier is acceptable, given that a manual
  `cosign verify` is insufficient for enforcement.
- The exact identity syntax for Fulcio in the installed containers/image, and a
  RHEL split where the organization owns the key and policy.

No new resource kind is proposed. The split matches
[ADR 0010](../adr/0010-artifact-evidence-and-supply-chain.md); the organization
records its policy and rotation itself.
