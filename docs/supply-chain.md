# Supply chain

How a DeskOS image states what it is made of and what was checked. This is an
initial slice, not a finished attestation pipeline
([ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md)).

## Two bills of materials

| Bill | Producer | Describes | Deterministic |
|---|---|---|---|
| Declared | `deskosctl sbom` | the inputs the Plan selects | yes |
| Installed | Syft, on the built image | the bytes the image contains | no, build-time |

The comparison checks RPM packages. Files, binaries, Flatpaks and trust anchors
are reported, not compared.

The declared SBOM is CycloneDX 1.5 from the same Plan as `plan` and `render`. It
carries no timestamp and no `serialNumber`, so identical resources give
byte-identical output. It lists the base image, RPM package names, RPM files,
verified binaries, system Flatpaks and trust anchors.

```bash
deskosctl sbom ./examples/layers \
  --workstation example-devops-rhel-10 \
  --output dist/example-devops-rhel-10.sbom.cdx.json
```

The installed SBOM and the scan run on the built image, where versions and bytes
exist:

```bash
deskosctl render \
  --workstation deskos-core-centos-stream-10 \
  --output dist/deskos-core-centos-stream-10
sudo podman build \
  -t localhost/deskos-core-centos-stream-10:test \
  dist/deskos-core-centos-stream-10

image=localhost/deskos-core-centos-stream-10:test
syft "$image" -o cyclonedx-json > installed.sbom.cdx.json
grype "$image" -o table
grype "$image" -o json > cves.json
```

`grype` fails non-zero at the configured threshold (`--fail-on critical`), which
is what CI uses.

## Inputs and who moves them

| Input | Pinned in | Moved by | Checked by |
|---|---|---|---|
| CentOS Stream 10 base | `resources/platforms/centos-stream-10.yaml` (tag and digest) | Renovate Docker PR, then a person reviews the build and boot | validate, render, image build, boot and session checks, scan |
| RHEL 10 base | `resources/platforms/rhel-10.yaml` | hand; Renovate is off because the registry needs an entitlement | composition here; build and boot downstream |
| RPM repositories and keys | `RpmRepository` resources | hand, with the vendor | resolver probes `repomd.xml`; the build verifies signatures |
| Binary artifacts | `BinaryArtifact` resources | Renovate proposes a version; a person fills the SHA-256 | resolver downloads and hashes; the build verifies |
| Runtime companion | `resources/software/deskos.yaml` | hand, from a released asset | checksum, release version, image build |
| ISO builder | installer workflows | hand, by digest | image build, boot check, ISO install test |
| Syft and Grype | the publish workflow | Renovate proposes the installer tag | pinned installer, checked in the same run |
| Flatpak remotes and apps | `FlatpakRemote`, `FlatpakSet` | hand, on the vendor's release | build and first-boot preinstall |

Known gaps: the RHEL base cannot be checked anonymously, Flatpak remotes have no
URL probe like binary artifacts, and a version-only binary PR is incomplete until
the checksum is filled.

## CI job

`.github/workflows/image-publish.yml` runs weekly and on dispatch. A dispatch
publishes only with `publish=true` on `main`; otherwise it is a dry look at a base
bump.

Each leg builds one image from a checkout, one per publishable workstation. It
writes the declared SBOM, produces `installed.sbom.cdx.json` with Syft and
`cves.json` with Grype, checks the exceptions record, generates the Grype
configuration from it, fails at `critical`, compares the two bills with
`tests/supply-chain/compare.py`, and uploads everything.

Both scan passes use `tests/supply-chain/grype-raw.yaml` (`ignore: []`), so
`grype.txt` and `cves.json` show everything. The publish and promotion checks
apply the exceptions; the raw scans do not.

A scheduled run has no inputs, so it uses the explicit `FAIL_ON` fallback of
`critical`. The weekly run cannot silently weaken it.

`release-watchdog.yml` dispatches the publish with `publish=true` when `:stable`
is more than eight days old, so a missed scheduler tick costs a day, not a
release.

### Freezing publication

Set the repository variable `PUBLISH_FREEZE` with a sentence saying why. The
weekly build, the installer, the promotion and the mirror then do nothing, and
`:stable` keeps serving the last good digest.

### Installer

`.github/workflows/installer-build.yml` is separate so an image that passed its
checks is published whether or not its installer built. Only CentOS Stream 10
publishes a Core installer.

The job verifies the image's keyless signature, builds with the image's own root
filesystem, signs the ISO with the workflow identity, and tags `sha256-<image
digest>`. Its `:stable` moves when the image is already stable.

A verifier must pin the workflow identity, not the repository.

### Results from a failed build

The workflow keeps the local image ID, OCI archive checksum, disk checksum and
the full `bootcheck`, `session` and `firstboot` directories for 14 days, with the
QCOW2 files excluded because the checksums identify them.

## Severity policy and VEX

The policy is [ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md):

| Level | Rule |
|---|---|
| Critical | blocks; the vendor's severity wins for a package the distribution ships, and an NVD score beside it does not move the check |
| High | tracked, not blocking by default; a desktop with a browser and a distribution kernel always carries many |
| Vendor-controlled components | follow the distribution's response; a Critical still blocks |

Every exception is explicit: an OpenVEX statement
(`tests/supply-chain/vex.openvex.json`) or an entry in the platform's record:
`exceptions.json` for CentOS Stream 10, `exceptions-fedora-45.json` for Fedora,
`exceptions-almalinux-10.json` for AlmaLinux. Nothing is silently ignored.

Each record names the owner, the review date, the base digest it was reviewed
against, and the exact matches it covers (package, type, version, location).
`tests/supply-chain/check_exceptions.py` enforces it: an expired review, a moved
base layer, a new component, a new version or an unapproved Critical fails before
the gate. `--emit-gate` writes the Grype configuration from the record, so the
gate cannot accept more than the record approves.

The CentOS Stream 10 baseline from the first run (Grype, distro `centos-10`,
2026-10-06): 0 Critical, 1866 High, 26735 Medium, 10263 Low, 124 Unknown, mostly
kernel subpackages and Firefox.

## Artifact identity

Each leg writes `identity.json`, a small canonical record:

| Field | Holds |
|---|---|
| source | commit, workflow, run, attempt, job |
| targets | platform and workstation |
| inputs | plan and generated manifest, by SHA-256 |
| local image | podman image ID and digest |
| artifacts | exported OCI archive, tested QCOW2 or raw disk, by SHA-256 |
| published | manifest digest and reference, when `publish=true` |
| checks | signature verification, boot, session and first-boot summaries |

`tests/supply-chain/identity.py` writes it with sorted keys and records every
missing entry instead of inventing one. A successful leg runs `--check`.

## Signing and provenance

The publish step attests SLSA v1 provenance: source commit, base digest and this
workflow as builder. That is the Build L2 shape, not a dedicated L3 builder.

On `main` with `publish=true`, the workflow signs the scanned image keyless with
cosign. Fulcio binds a short-lived certificate to the workflow's OIDC identity,
and Rekor records the signature.

Four attestations are attached: provenance, a reference to the installed SBOM
(digest, format, SHA-256, location), the reviewed exceptions, and the matches
this image accepted. Rekor caps an attestation at 100 KB, so the SBOM is not
attached; the statement names its SHA-256.

Verify a digest:

```bash
ref=quay.io/deskos/deskos-core-centos-stream-10@sha256:...
issuer=https://token.actions.githubusercontent.com
identity='https://github.com/deskosproject/deskos-core/.github/workflows/image-publish.yml@refs/heads/main'

cosign verify \
  --certificate-oidc-issuer "$issuer" \
  --certificate-identity "$identity" \
  "$ref"

for type in \
  slsaprovenance1 \
  https://deskos.org/supply-chain/sbom/v1 \
  https://deskos.org/supply-chain/exceptions/v1 \
  https://deskos.org/supply-chain/effective-exceptions/v1; do
  cosign verify-attestation \
    --type "$type" \
    --certificate-oidc-issuer "$issuer" \
    --certificate-identity "$identity" \
    "$ref"
done
```

The reference names the image digest and the SBOM's SHA-256. Check the SBOM you
obtained against that hash. Do not use `--insecure-ignore-tlog` here.

Verification is not enforcement: the machine accepts what its
`containers-policy.json` allows. Core chose keyless signatures with manual
verification on 2026-10-25; an organization that wants enforcement keeps
key-based signing.

RHEL builds run in a downstream organization's pipeline, with no OIDC. They are
signed with a cosign key kept outside this repository and verified against its
public key.

## Before accepting a risk, check the base

A run that fails on Criticals inside the base is not asking for an exception
first. The vendor may have published a base that fixes them, and re-pinning it
accepts nothing.

It does not always work. On 2026-11-04 the CentOS Stream 10 base moved to
`f43c24c5` (2026-11-03, kernel 6.12.0-280, glibc 148).

The two Go exceptions still match their approved combinations. `CVE-2026-114406`
still matches `ca-certificates-...-100.3`, the version whose arrival was the
condition to drop that exception, so the decision is open in [issue

#411](https://github.com/deskosproject/deskos-core/issues/411).

When a new Critical appears, the release owner chooses one route and records it
on an issue with the digest, scanner and database versions, and the matching
package, type, version and location:

| Route | What it means |
|---|---|
| Revalidate | prove the finding does not apply, with reachability or a vendor advisory that shows the shipped version is fixed; no blanket ignore |
| Exception | add an exact, owned and dated entry to the record, with a review date that forces the question back |
| Replace or withdraw | publish a fix or stop recommending the digest; a deleted tag does not revoke copies already pulled |

## Promotion

A digest moves only when, for that digest:

1. the render validates and the boot, session and first-boot checks pass;
2. the installed SBOM contains every declared RPM package name;
3. the CVE scan is at or below `critical`;
4. the image carries a signature over its digest.

`promotion.yml` runs once for every successful `publish (…)` leg of the run:
a `prepare` job builds the station list and a matrix promotes each one. For each
digest it checks that the source channel serves it, verifies the keyless
signature, fetches and verifies every object the index names, stores an approval
record under `approvals/<repository>/stable/`, moves `:stable` by digest, and
writes the release record under `releases/<repository>/stable/`. The approval
names the digest, the previous stable digest, the actor, the approver, the reason,
the index hash and the run; the release record adds the channels, the signature,
the installer digest and the evidence objects (see
[release-contract.md](design/release-contract.md)).

The first promotion (run `40317904401`, 2026-10-26) moved `sha256:96fe80d3…`; a
second after the Stream 10 base bump (run `40412048361`) moved
`sha256:649aff75…`. There is no canary or pilot channel.

Rescan a published digest:

```bash
ref=quay.io/deskos/deskos-core-centos-stream-10@sha256:...
podman pull "$ref"

grype version
syft --from containers-storage "$ref" \
  --select-catalogers "-file-content-cataloger,-file-digest-cataloger,-file-executable-cataloger,-file-metadata-cataloger" \
  -o cyclonedx-json=installed.sbom.cdx.json

grype --from containers-storage "$ref" \
  --config tests/supply-chain/grype-raw.yaml \
  -o json > cves.json

id=$(podman create "$ref")
podman cp "$id:/usr/share/deskos/plan.json" plan.json
podman rm "$id"

python3 tests/supply-chain/check_exceptions.py \
  --cves cves.json \
  --exceptions tests/supply-chain/exceptions.json \
  --plan plan.json \
  --emit-gate gate.grype.yaml

grype --from containers-storage "$ref" \
  --config gate.grype.yaml \
  --vex tests/supply-chain/vex.openvex.json \
  --fail-on critical
```
