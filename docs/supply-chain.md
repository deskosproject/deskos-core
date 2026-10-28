# Supply chain

How a DeskOS artifact states what it is made of, and where a CVE scan
fits. This is an **initial** slice, not a finished attestation pipeline.
The decision and staged plan are
[ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md).

## Two bills of materials

| Bill | Producer | Describes | Deterministic |
|---|---|---|---|
| **Declared** | `deskosctl sbom` | the inputs the Plan selects | yes |
| **Installed** | a scanner on the built OCI image (Syft) | the bytes the image contains | no (build-time) |

They are complementary, and a promotion gate compares them.

### The declared SBOM

`deskosctl sbom ROOT... --workstation NAME [--output FILE]` renders
[CycloneDX](https://cyclonedx.org/) 1.5 JSON from the same typed Plan that
`plan`, `render` and the build use. It carries no timestamp and no
`serialNumber`, so identical resources render byte-identical output.

It lists:

- the base image (name and digest-pinned hash);
- RPM packages by name (the repository resolves exact versions at build);
- RPM files, with their pinned SHA-256 and source URL;
- verified binaries, with version, destination and SHA-256;
- system Flatpaks, by application id and branch;
- CA trust anchors, with SHA-256 and destination.

Its limits are deliberate: it resolves no package versions and scans no
bytes. It is an **ingredient list with provenance** — the compiler knows
intent, not installed content.

```bash
deskosctl sbom ./examples/organization-and-role \
  --workstation example-devops-rhel-10 \
  --output dist/example-devops-rhel-10.sbom.cdx.json
```

### The installed SBOM and the CVE scan

The authoritative SBOM and the vulnerability scan run on the **built
image**, because only then do package versions and file contents exist:

```bash
# Build the Core image from its rendered context.
mkdir -p dist
deskosctl render --workstation deskos-core-centos-stream-10 --output dist/deskos-core-centos-stream-10
sudo podman build -t localhost/deskos-core-centos-stream-10:test dist/deskos-core-centos-stream-10

image=localhost/deskos-core-centos-stream-10:test
syft "$image" -o cyclonedx-json > installed.sbom.cdx.json
grype "$image" -o table
grype "$image" -o json > cves.json
```

`grype` fails non-zero when the configured severity threshold is met
(`--fail-on critical`), which is what a CI gate uses.

## Inputs and who moves them

Every input below is pinned; the column says what moves it, not who is on call.
The trigger is a deliberate pull request or a workflow dispatch, never an
automatic merge (ADR 0011). "Resolver" is
[`tests/artifacts/resolve.py`](../tests/artifacts/resolve.py).

| Input | Pinned in | Moved by | Checked by |
|---|---|---|---|
| CentOS Stream 10 base | `resources/platforms/centos-stream-10.yaml` (tag + digest) | Renovate Docker PR, then a person reviews the build/boot | `validate`/`render`, image build, boot/session checks, scan |
| RHEL 10 base | `resources/platforms/rhel-10.yaml` (tag + digest) | Hand; Renovate is disabled because the registry needs an entitlement | composition here; build/boot in the downstream organization |
| RPM repositories and GPG keys | `RpmRepository` resources | Hand, with the vendor | resolver probes `repomd.xml`; the build verifies signatures |
| Binary artifacts (CLI tools, extensions) | `BinaryArtifact` resources | Renovate proposes a release version; a person fills the SHA-256, which Renovate cannot compute | resolver downloads and hashes; image build |
| Runtime companion (`deskos`) | `resources/software/deskos.yaml` | Hand, from a released asset | checksum, release version check, image build; it must read the plan the same release renders |
| ISO builder (BIB) | `docs/installer-iso.md`, the ISO workflows | Hand, by digest | image build, boot check, ISO install test |
| Syft and Grype | `.github/workflows/supply-chain.yml` | Renovate proposes the installer tag | pinned installer, gate in the same run |
| GitHub Actions | workflow files | Hand; the token has no workflows permission | action version review; no action runs with registry or signing credentials |
| Flatpak remotes and apps | `FlatpakRemote`/`FlatpakSet` resources | Hand, on the vendor's release | build and first-boot preinstall; **not** the resolver |

Known gaps, stated rather than implied:

- the RHEL base cannot be checked anonymously; its check is downstream;
- Flatpak remotes and apps have no URL/checksum probe like binary artifacts;
- a version-only binary PR is incomplete until the checksum is filled;
- action versions are reviewed by hand, and this repository has no workflow
  token that could change them.

Compilation itself makes no network request; everything above is checked in CI
or at build time, not while rendering.

## CI job

`.github/workflows/supply-chain.yml` runs on a weekly schedule and on
`workflow_dispatch`; only a dispatch with `publish=true` on `main` publishes.
It builds one image per platform from a checkout — one matrix leg per publishable
workstation reference, each with its own exceptions record and destination
repository — and runs the steps above:
it writes the declared SBOM, produces `installed.sbom.cdx.json` with Syft
and `cves.json` with Grype, checks the exceptions record and generates the
gate configuration from it, fails on the chosen severity, compares the two
bills with `tests/supply-chain/compare.py`, and uploads all evidence. The
two evidence passes are pinned to `tests/supply-chain/grype-raw.yaml`
(`ignore: []`), so `grype.txt` and `cves.json` always show everything;
only the gate applies the exceptions.

A scheduled run has no dispatch inputs, so the gate uses the explicit
`FAIL_ON` fallback of `critical`. That is the same baseline as a manual
dispatch's default; the weekly watch cannot silently weaken it.

### Evidence retained for a failed gate

The workflow keeps the local image ID, the OCI archive checksum, the disk
checksum, and the full `bootcheck`, `session`, `firstboot` and
`journal-failed-boot` directories for 14 days. That includes each frame,
`serial.log`, `qemu.log` and the structured summaries; QCOW2 files are
excluded from the artifact because the checksums identify them. A retry is
visible beside the first attempt rather than replacing it. The artifact is
therefore evidence of what ran, not only a green summary.

`compare.py` gates **RPM packages**: every declared `deskos:source=rpm`
component must appear, by name, in the installed SBOM. Binaries, RPM
files, Flatpaks and trust anchors are reported but not gated there — a
scanner does not enumerate all of them, and `generated-manifest.json` and
the boot/session checks cover those.

The job first ran on 2026-10-06 (run `37466048646`, success, 0 Critical).
Syft and Grype are pinned in the workflow before the gate can block a
release.

## Severity policy and VEX

The policy the gate applies is
[ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md):

- **the blocking gate is `Critical`**; a digest with an unresolved Critical
  is not promoted;
- **High is tracked, not blocking** by default: a full GNOME desktop with a
  browser and a distribution kernel always carries a large High count,
  mostly backported upstream or not reachable in context;
- **vendor-controlled components** (the kernel, distribution packages)
  follow the distribution's security response; a Critical in them still
  blocks;
- **every exception is explicit** — an OpenVEX statement
  (`tests/supply-chain/vex.openvex.json`) or an entry in the reviewed
  record `tests/supply-chain/exceptions.json`. Nothing is silently ignored
  and there are no blanket exclusions.

The record is the single source of truth: it names the owner, the review
date, the base image digest it was reviewed against and, for each
exception, the exact matches it covers (package, type, version and
location). `tests/supply-chain/check_exceptions.py` enforces it between the
raw report and the gate — an expired review, a moved base layer, a new
component, a new version or an unapproved Critical fails the build — and
`--emit-gate` writes the Grype gate configuration from it, so the gate can
never accept more than the record approves. The record is uploaded with the
evidence and attested on the published digest, together with a second
attestation of the matches this image actually accepted (a standing
approval is often wider than one image's need); both are verified by
digest, signature and predicated type.

CentOS Stream 10 baseline from that first run (Grype, distro `centos-10`):
0 Critical, 1866 High, 26735 Medium, 10263 Low, 124 Unknown. The High count
is dominated by the kernel (one entry per subpackage) and Firefox.

## Artifact identity

Each gate leg writes `identity.json` beside the raw evidence. It is a small,
canonical record, not an SBOM and not a replacement for `/etc/os-release`:

- source commit, workflow, run, attempt and job;
- platform and workstation;
- the declared plan and generated manifest, by SHA-256;
- the local Podman image ID and digest;
- the exported OCI archive, when the leg exports one;
- the tested QCOW2 or raw disk, by SHA-256;
- the published manifest digest and reference, when `publish=true`;
- the signature-verification file and the boot, session and first-boot
  summaries when they exist.

`tests/supply-chain/identity.py` writes it with sorted keys and records every
missing entry instead of inventing a value. The workflow uploads the record even
on failure; a successful leg runs `--check`, so an incomplete success cannot
pass silently. The record links the tested identity to the published digest
without pretending that a local OCI archive digest and a registry manifest
digest are the same bytes: compression and registry conversion can change the
latter, and the record names each one separately.

The record is workflow evidence. The workflow artifact keeps its 14-day
retention; for a published digest the same evidence is also stored under
`evidence.deskos.org`, keyed by digest.

## Signing and provenance

The publish step attests **SLSA v1 provenance** (predicate type
`https://slsa.dev/provenance/v1`, subject: the image digest) beside the SBOM and
the exceptions: the source commit, the base image digest the plan pins, and this
workflow as the builder. cosign builds the in-toto statement around that
predicate and sets the subject from the digest the publish step pushed.

It is produced by the pipeline that builds the image, so it claims what SLSA
calls a Build L2 shape — the build is what says it built — and not the isolation
of a dedicated L3 builder. Claiming L3 for a self-produced statement would be
false.

The same workflow, with `publish` on `main`, pushes the scanned image and
signs it **keyless** with [cosign](https://docs.sigstore.dev/): Fulcio
issues a short-lived certificate bound to the workflow's OIDC identity and
Rekor records the signature. Four attestations are attached: the SLSA
provenance above, a reference to the **installed** SBOM (the image digest, the
SBOM's format and SHA-256, and where to find it), the reviewed exceptions, and
the matches this image accepted.

Rekor's public instance caps an attestation at 100 KB, and an SBOM is far
larger, so the SBOM itself is not attached: the signed statement names its
exact SHA-256, and the digest is the unit of promotion. The SBOM travels as
workflow evidence and, for a published digest, in the digest-keyed
`evidence.deskos.org` store; see
[the evidence design](design/evidence-retention.md).

**Verify a digest.** Checking the signature is necessary but not sufficient:
require the four attestation types, pin the issuer and the workflow
identity, and compare the SBOM you obtained against the hash the reference
names. Do not use `--insecure-ignore-tlog` on this path.

A verification command is not enforcement: the installer and the installed
machine accept what their `containers-policy.json` allows. Core's decision on
2026-10-25 is keyless signatures with manual verification; an organization that
chooses enforcement keeps key-based signing so its policy can reject unsigned
updates. That split, the verified recipe and the remaining limits are in
[the authenticity design](design/authenticity.md).

```bash
# One repository per platform and release:
# quay.io/deskos/deskos-core-centos-stream-10
ref=quay.io/deskos/deskos-core-centos-stream-10@sha256:…
issuer=https://token.actions.githubusercontent.com
identity='https://github.com/deskosproject/deskos-core/.github/workflows/supply-chain.yml@refs/heads/main'

cosign verify --certificate-oidc-issuer "$issuer" \
  --certificate-identity "$identity" "$ref"

for type in \
  slsaprovenance1 \
  https://deskos.org/supply-chain/sbom/v1 \
  https://deskos.org/supply-chain/exceptions/v1 \
  https://deskos.org/supply-chain/effective-exceptions/v1; do
  cosign verify-attestation --type "$type" \
    --certificate-oidc-issuer "$issuer" \
    --certificate-identity "$identity" "$ref"
done

# The reference names the image digest and the SBOM's SHA-256; read it and
# check the SBOM you obtained against it.
cosign verify-attestation --type https://deskos.org/supply-chain/sbom/v1 \
  --certificate-oidc-issuer "$issuer" --certificate-identity "$identity" "$ref" \
  | jq -r '.payload' | base64 -d | jq '{image, sha256, components}'
```

**Rescan a published digest.** The weekly job builds from the checkout; it
does not rescan every digest already published. The list of published digests
is in [architecture.md](architecture.md#validation-status); RHEL digests belong
to the downstream organization. To review one later:

```bash
ref=quay.io/deskos/deskos-core-centos-stream-10@sha256:…
podman pull "$ref"

# Record what produced the result, not only the result.
grype version            # scanner and vulnerability database versions
syft --from containers-storage "$ref"   --select-catalogers "-file-content-cataloger,-file-digest-cataloger,-file-executable-cataloger,-file-metadata-cataloger"   -o cyclonedx-json=installed.sbom.cdx.json

# Evidence pass, unfiltered.
grype --from containers-storage "$ref"   --config tests/supply-chain/grype-raw.yaml -o json > cves.json

# The plan the image carries names its base digest; the exception record
# must have been reviewed against that base.
id=$(podman create "$ref")
podman cp "$id:/usr/share/deskos/plan.json" plan.json
podman rm "$id"

python3 tests/supply-chain/check_exceptions.py   --cves cves.json --exceptions tests/supply-chain/exceptions.json   --plan plan.json --emit-gate gate.grype.yaml

# The gate: Critical blocks, with only the reviewed exceptions applied.
grype --from containers-storage "$ref"   --config gate.grype.yaml --fail-on critical
```

When a new Critical appears, the release owner for that digest chooses one of
three routes and records it on an issue with the digest, scanner/database
versions and the matching package, type, version and location:

1. **Revalidate**: prove the finding does not apply (a reachability argument,
   or a vendor advisory that shows the shipped version is the fixed one), and
   record the evidence; do not add a blanket ignore.
2. **Exception**: add an exact, owned and dated entry to
   `tests/supply-chain/exceptions.json`, with a review date that forces the
   question to return (ADR 0012).
3. **Replace or withdraw**: publish a fix or stop recommending the digest. A
   deleted tag or signature does **not** revoke copies already pulled or
   cached; consumers move by changing the reference or pin. Publication
   authority remains with the release owner.

The raw report and the effective exceptions belong with the issue; artifact
retention is 14 days, so use the digest-keyed evidence under
`evidence.deskos.org` for a published image.

**RHEL builds** run in a downstream organization's pipeline, which owns the
entitled build host, with no OIDC: they are signed with a cosign **key** kept
outside this repository and verified against its public key. That key's
rotation, and the fact that verification without a transparency log carries
less evidence than the keyless path, are recorded with the organization.

The gates are unified: `supply-chain.yml` is the single publish path, and it
boots, scans, signs and attests the same image before it is promoted; it is
dispatched manually with `publish=true`. `vm-bootcheck.yml` is a pure check.

The matrix runs with `fail-fast: false`, so one platform failing does not
stop another from being published: promotions are **independent per
platform**, and each is gated on its own build, boot, session and first-boot
checks. A red leg means that platform did not move; it does not retract a
platform that already did. The matrix **has** been dispatched with
`publish=true`: that dispatch built, scanned, booted and then pushed and signed
one digest per platform, and its digests are in
[architecture.md](architecture.md#validation-status) with the run that produced
them.

## Before accepting a risk, check whether the base moved

A gate that fails on Criticals inside the **base** is not asking for an exception
first. A vendor can publish a new bootc base that fixes Criticals already present in the
pinned one. Re-pinning to it is the fix that accepts nothing: run the scan after the
change and let it decide whether the new base was enough.

The check is cheap and it is the first thing to do, because the alternative —
writing an exception — is a decision to ship a known Critical, and that decision
should be the one left over, not the one reached for.

It does not always work: CentOS Stream 10's `:stream10` served a manifest with the
**same** config as the pin (same build, re-pushed), so re-pinning could not change
anything there. That is the case where the risk goes on the record, with a reason,
an owner and a review date — and not before.

## Promotion gate

A digest moves forward only when, for that digest:

1. the render validates and the boot, session and first-boot checks pass (see
   [development.md](development.md) and `tests/vm`);
2. the installed SBOM contains every component the declared SBOM lists;
3. the CVE scan is at or below the agreed threshold;
4. the image carries a signature over its digest.

Steps 1–4 run in `supply-chain.yml`. Promotion is a separate, recorded move:

- the gate publishes `:<commit>`, `:latest` and `:candidate`, all by digest.
  `:candidate` is the digest a promotion checks, `:latest` is the fast lane for
  whoever wants the newest build, and `:stable` is the recommended reference and
  the only tag `promotion.yml` writes;
- `promotion.yml` is the only writer of `:stable`. The first promotion ran on
  2026-10-26 (run `40317904401`): `sha256:96fe80d3…` moved to `:stable`, with
  its record in `approvals/deskos-core-centos-stream-10/stable/`. A second,
  after the base bump (run `40412048361`), moved `sha256:649aff75…`. It checks that
  the source channel serves the digest, verifies the keyless signature, fetches and verifies every
  object the digest's evidence index names, stores an approval record under
  `approvals/<repository>/stable/` in the public evidence bucket, and only then
  moves `:stable` by digest;
- the record names the digest, the previous stable digest, the actor, the
  approver, the reason, the evidence index hash and the run.

This is the "factory" layer around the compiler — it never enters compiler
semantics (see `AGENTS.md`, invariant 11). No canary or pilot channel exists.
