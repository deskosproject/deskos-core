# Consumer-side image authenticity

**Status: decided, 2026-10-25.** DeskOS Core signs keyless with OIDC: no key
file, no consumer enforcement, and verification is manual. An organization that
chooses enforcement keeps a key outside its repository and uses a key-based
policy, so `bootc` can reject an unsigned update. The key-based recipe is verified in the lab; that organization's policy,
installer and key-rotation paths are not. This document changes no DeskOS Core
image policy or key.

CI verifies signatures with `cosign`. That proves the gate checked a digest; it
does not prove an installer or an installed workstation rejects an unsigned or
wrongly signed image. Core accepts that gap; an organization that requires
enforcement closes it with a key. This page records the verified recipe and
where each model stops.

## What is true today

- Core images are signed **keyless** with cosign: Fulcio issues a certificate
  for the workflow's OIDC identity and Rekor records the signature. Three
  attestations are attached (SBOM reference, reviewed exceptions, accepted
  matches).
- **Core decided on 2026-10-25:** keyless signatures, no key file, and no
  enforcement on the installed machine. A person verifies a
  digest manually. An organization that chooses enforcement keeps key-based
  signing; making that signature a policy requirement on update is that
  organization's work.
- Organization-owned RHEL images are signed with a **key** kept outside its
  repository, with no transparency log.
- No generated **Core** image ships a `/etc/containers/policy.json`; the
  containers/image default accepts an image without a signature. That
  organization's policy would be the enforcing half of the split.
- `bootc` honors `containers-policy.json` when fetching images. Its upstream
  documentation says the default does not require signatures, and that this is
  not a bootc vulnerability.
- `bootc install`/`switch` and the install config expose
  `--enforce-container-sigpolicy` / `enforce-container-sigpolicy`. That flag only
  requires the policy's **default** to demand signatures; it does not author the
  identity constraints for us.
- The Anaconda `ostreecontainer` path used by the ISO has not been tested with a
  requiring policy; this matters to an organization that enforces, not to
  Core's manual path.

## Core: keyless, manual verification

Core's signature is evidence, not enforcement. The release page must publish
the digest and the exact workflow identity; a person verifies before installing
or updating. `bootc` still accepts an unsigned image, so a user who does not run
the check is not protected by it. Rekor remains the transparency record: an
unexpected signature from the workflow identity can be investigated after the
fact.

## Candidate mechanisms

| Mechanism | What it can require | Limits to test |
|---|---|---|
| `signedBy` (GPG simple signing) | a GPG key over the image | cosign does not produce this signature format; not a match for the current signatures |
| `sigstoreSigned` with `keyPath`/`keyData` | a cosign signature by a named public key | Core is keyless; an organization that enforces already has a key. Verification and policy provisioning are local/offline |
| `sigstoreSigned` with `fulcio` | a Fulcio certificate from an expected OIDC issuer, constrained to a subject (the documented policy field is an email) | **the workflow identity we use is a GitHub URL, not an email.** containers/image cannot express it; a repository-wide identity regex would be too broad for release authorization. Core therefore does not enforce keyless signatures |
| `bootc --enforce-container-sigpolicy` | that the policy is not `insecureAcceptAnything` | it does not check which identity is allowed; the policy must do that |
| Installer/Anaconda `ostreecontainer` | unclear | test with a requiring policy; if it does not enforce, the install path needs a separate control |
| A `deskos` wrapper or hook that calls `cosign verify` | exact issuer/identity | a command, not enforcement of every pull; acceptable only if it cannot be bypassed and covers install, switch and update |

The exact identity authorization matters: “signed by this repository's
workflow” is weaker than “signed by the release workflow for this digest”. The
policy must name the allowed identity and, where the mechanism permits, the
digest or repository.

## Provisioning and recovery

A policy is part of the image. That creates a circular problem: if a release is
compromised, the next trusted policy would arrive through the same update path.
For an organization that chooses enforcement, the design must answer:

- where the policy and public trust roots live (image `/etc`, a local
  configuration system, or an installer-provided file);
- how a policy or key rotation reaches machines after a compromise **without
  trusting the compromised release**;
- what an offline machine can verify (key-based signatures and local CA data are
  plausible; a keyless path must not require a live Fulcio/Rekor fetch that turns
  an offline update into a permanent failure);
- the emergency path if a previously trusted identity must be distrusted, and
  how a machine that already staged the release is recovered.

This is the part that cannot be solved by adding a JSON file and calling it done.

## Working recipe and lab result (2026-10-25)

The combination was found after the first attempts failed for two avoidable
reasons: cosign 3's default bundle is an OCI index that this containers/image
does not fetch as an attachment, and a digest signature has no tag to satisfy
`matchRepoDigestOrExact` when the image is pulled by tag. Signing in cosign's
legacy referrers format and using `matchRepository` works.

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

`--allow-http-registry` / `--allow-insecure-registry` were needed only for the
lab's HTTP registry; they are not part of the recipe for a TLS registry.

**Results on bootc 1.17.1, containers/image from Podman 5.8.9, cosign 3.1.3.**
This is the enforced route; Core does not use it.

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

So key-based enforcement works **only when all three pieces are present**: a
policy that names the key and identity, `use-sigstore-attachments` for the
registry, and a signature stored in the legacy referrers format that this
containers/image reads. `--enforce-container-sigpolicy` alone is still not a
control: with no policy it let an unsigned switch through.

The same policy was exercised through `bootc upgrade`, not only `switch`: a
new signed digest was staged, and an unsigned digest was rejected. The
`deskos update` guard and the generated image-update unit were blocked on
composefs by the ostree-only `/run/ostree-booted` test (#177); with the fix in
#178, `deskos update` stages the signed image and the unit runs `bootc upgrade`
instead of being skipped.

**Key-based policy and keyless signing are different things, not two steps of
one migration.** An image that ships a `cosign.pub` and a `sigstoreSigned`
policy for its *key-based* predecessor gets no enforcement on its own keyless
tags: the policy names a public key, and there is a Fulcio certificate instead.
`--enforce-container-sigpolicy` therefore protects the old repository and not
the keyless one, which is how an image can appear to enforce and not enforce at
the same time. The open problem is the keyless Fulcio identity, not whether the
key-based policy works. No repository-wide identity regex is used as release
authorization here.

**Upstream status (2026-10-25).** containers/image's `fulcio` policy accepts
`oidcIssuer` and `subjectEmail`; it does not match a URI SAN. The pull request
that would have added URI matching
([containers/image#2235](https://github.com/containers/image/pull/2235)) was
closed unmerged after the maintainers argued for a more specific typed claim.
BYOPKI (`pki`) supports `subjectEmail` and `subjectHostname` for a custom CA,
not a Fulcio certificate, and sigstore bundle support is still open
([container-libs#388](https://github.com/podman-container-tools/container-libs/issues/388)).
No merged or committed upstream change pins a GitHub Actions URI SAN, so
key-based signing remains the only verified enforcement route.

**Not yet tested for an organization that enforces:** the Anaconda
`ostreecontainer` install path; offline key-based verification; key rotation
and lost-key recovery. Keyless Core enforcement is not a pending test:
containers/image cannot pin a GitHub URI SAN, and Core accepts manual
verification instead.

## Enforced prototype plan (disposable lab)

1. Build a scratch image carrying a `sigstoreSigned` policy for a test key.
   Sign a test image with that key; prove an unsigned image, a wrong key and a
   tampered manifest are rejected on `bootc switch` and `bootc upgrade`.
2. Repeat with `bootc --enforce-container-sigpolicy` and an
   `insecureAcceptAnything` default; prove the flag fails.
3. Run the Anaconda `ostreecontainer` install path with the requiring policy;
   record whether install is enforced or silently accepts.
4. Keyless `fulcio` policy: confirm that containers/image can match only an
   issuer and subject email, not the GitHub URI SAN. Core stays manual; revisit
   only if upstream adds a typed identity claim.
5. Cut the network and repeat the key cases; record what an enforcing
   organization can verify offline.
6. Test both update paths: the generated image timer (`bootc upgrade`) and
   `deskos update`. Both were exercised with a signed and an unsigned digest;
   keep them in the evaluator.
7. Exercise key rotation: a new key, a policy update, and recovery after a key
   is lost.

Each case needs a positive and a negative result, a recorded digest, the bootc
version and the containers/image version.

## Decisions

- **Decided 2026-10-25:** Core signs keyless with OIDC — no key file, no
  consumer enforcement, manual verification. An organization that
  chooses enforcement keeps key-based signing; key custody, rotation and backup
  stay downstream (P0-04).
- How an enforcing organization bakes the policy into the image or provisions
  it from the installer, and how a compromised-release policy update is
  delivered.
- Whether a DeskOS-specific verifier is acceptable, given that a manual
  `cosign verify` is explicitly insufficient for enforcement.
- The exact identity syntax supported by the installed containers/image for
  Fulcio certificates.
- RHEL: the downstream organization owns the key and policy; this repository
  only documents the mechanism.

No new resource kind is proposed. The split matches
[ADR 0010](../adr/0010-artifact-evidence-and-supply-chain.md); the organization
records its policy and rotation in its own repository.
