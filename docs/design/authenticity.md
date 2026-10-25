# Consumer-side image authenticity

**Status: research design, 2026-10-25. No image policy, key or install path is
changed by this document.** The upstream mechanisms below are candidates; the
repository has not run the prototype yet.

CI verifies signatures with `cosign`. That proves the gate checked a digest; it
does not prove an installer or an installed workstation rejects an unsigned or
wrongly signed image. This page defines what to test and where the limits are.

## What is true today

- Core images are signed **keyless** with cosign: Fulcio issues a certificate
  for the workflow's OIDC identity and Rekor records the signature. Three
  attestations are attached (SBOM reference, reviewed exceptions, accepted
  matches).
- DayTwo RHEL images are signed with a **key** kept outside its repository, with
  no transparency log.
- No generated image currently ships a `/etc/containers/policy.json`; the
  containers/image default accepts an image without a signature.
- `bootc` honors `containers-policy.json` when fetching images. Its upstream
  documentation says the default does not require signatures, and that this is
  not a bootc vulnerability.
- `bootc install`/`switch` and the install config expose
  `--enforce-container-sigpolicy` / `enforce-container-sigpolicy`. That flag only
  requires the policy's **default** to demand signatures; it does not author the
  identity constraints for us.
- The Anaconda `ostreecontainer` path used by the ISO has not been tested with a
  requiring policy.

## Candidate mechanisms

| Mechanism | What it can require | Limits to test |
|---|---|---|
| `signedBy` (GPG simple signing) | a GPG key over the image | cosign does not produce this signature format; not a match for the current signatures |
| `sigstoreSigned` with `keyPath`/`keyData` | a cosign signature by a named public key | Core is keyless today; DayTwo already has a key. Verification and policy provisioning are local/offline |
| `sigstoreSigned` with `fulcio` | a Fulcio certificate from an expected OIDC issuer, constrained to a subject (the documented policy field is an email) | **the workflow identity we use is a GitHub URL, not an email.** Whether the exact workflow/ref can be expressed needs a lab; a repository-wide identity regex would be too broad for release authorization |
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
The design must answer:

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

**Results on bootc 1.17.1, containers/image from Podman 5.8.9, cosign 3.1.3:**

| Case | Result |
|---|---|
| signed with the pinned key, policy `sigstoreSigned` + `matchRepository` | **accepted and staged** (rc 0); `--enforce-container-sigpolicy` also rc 0 |
| same, unsigned image | rejected: “A signature was required, but no signature exists” |
| same, signed by another key | rejected: “cryptographic signature verification failed” |
| signature identity `127.0.0.1:5000/…`, image pulled as `registry.lab:5000/…` | rejected: identity not accepted |
| no policy, `--enforce-container-sigpolicy` | **accepted the unsigned image** (rc 0) |
| `default: reject` | rejected |
| cosign bundle copied into the local sigstore store | rejected: unrecognized signature format |

So key-based enforcement works **only when all three pieces are present**: a
policy that names the key and identity, `use-sigstore-attachments` for the
registry, and a signature stored in the legacy referrers format that this
containers/image reads. `--enforce-container-sigpolicy` alone is still not a
control: with no policy it let an unsigned switch through.

**Bluefin / Universal Blue** documents the same shape: a `sigstoreSigned`
entry with a key and `use-sigstore-attachments`, plus the cosign 3
compatibility flags (`--new-bundle-format=false`, `--registry-referrers-mode=legacy`).
Its open issue is about keyless Fulcio identity, not about whether key-based
policy works. No repository-wide identity regex is used as release authorization
here.

**Not yet tested:** the Anaconda `ostreecontainer` install path; keyless Fulcio
(containers/image binds Fulcio identity by issuer and subject email, while a
GitHub Actions certificate carries a URI SAN); both update paths (`bootc
upgrade` timer and `deskos update`, which use the same library); offline
verification; key rotation and loss of trust in a digest that previously
verified.

## Prototype plan (disposable lab)

1. Build a scratch image carrying a `sigstoreSigned` policy for a test key.
   Sign a test image with that key; prove an unsigned image, a wrong key and a
   tampered manifest are rejected on `bootc switch` and `bootc upgrade`.
2. Repeat with `bootc --enforce-container-sigpolicy` and an
   `insecureAcceptAnything` default; prove the flag fails.
3. Run the Anaconda `ostreecontainer` install path with the requiring policy;
   record whether install is enforced or silently accepts.
4. Test a keyless `fulcio` policy. Determine whether the workflow URL can be
   constrained at all; if the policy can only name an email, record that keyless
   Core signatures cannot be pinned exactly by this mechanism and choose
   key-based signing for the enforced path, or a different mechanism.
5. Cut the network to Fulcio/Rekor and repeat the key and keyless cases; record
   which verification still works offline.
6. Test both update paths: the generated image timer (`bootc upgrade`) and
   `deskos update`.
7. Exercise rotation: a new key/identity, a policy update, and loss of trust in
   a digest that previously verified.

Each case needs a positive and a negative result, a recorded digest, the bootc
version and the containers/image version.

## Decisions this design does not make

- Core keyless or key-based signatures for the enforced path; if key-based, the
  key's custody, rotation and backup (P0-04).
- Whether the policy is baked into the image, provisioned by the installer, or
  both; and how a compromised-release policy update is delivered.
- Whether a DeskOS-specific verifier is acceptable, given that a manual
  `cosign verify` is explicitly insufficient.
- The exact identity syntax supported by the installed containers/image for
  Fulcio certificates.
- RHEL: the downstream organization owns the key and policy; this repository
  only documents the mechanism.

No new resource kind is proposed. A working experiment and a human-approved ADR
would be required first.
