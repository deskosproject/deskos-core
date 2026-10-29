# Supply chain

How a DeskOS artifact states what it is made of, and where a CVE scan fits. This
is an initial slice, not a finished attestation pipeline; see
[ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md).

## Two bills of materials

| Bill | Producer | Describes | Deterministic |
|---|---|---|---|
| **Declared** | `deskosctl sbom` | the inputs the Plan selects | yes |
| **Installed** | a scanner on the built OCI image (Syft) | the bytes the image contains | no (build-time) |

The check compares RPM packages; the rest is reported, not compared.

### The declared SBOM

`deskosctl sbom ROOT... --workstation NAME [--output FILE]` renders
[CycloneDX](https://cyclonedx.org/) 1.5 JSON from the same typed Plan as `plan`,
`render` and the build. It carries no timestamp and no `serialNumber`, so identical
resources give byte-identical output. It lists:

- the base image (name and digest-pinned hash);
- RPM packages by name (the repository resolves exact versions at build);
- RPM files, with their pinned SHA-256 and source URL;
- verified binaries, with version, destination and SHA-256;
- system Flatpaks, by application id and branch; CA trust anchors, with SHA-256.

It resolves no versions and scans no bytes: it lists ingredients, not installed
content.

```bash
deskosctl sbom ./examples/organization-and-role \
  --workstation example-devops-rhel-10 \
  --output dist/example-devops-rhel-10.sbom.cdx.json
```

### The installed SBOM and the CVE scan

The authoritative SBOM and the vulnerability scan run on the built image, where
package versions and file contents exist:

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
(`--fail-on critical`), which is what CI uses.

## Inputs and who moves them

Every input is pinned; the column says what moves it. The trigger is a pull request
or a workflow dispatch, never an automatic merge (ADR 0011). "Resolver" is
[`tests/artifacts/resolve.py`](../tests/artifacts/resolve.py).

| Input | Pinned in | Moved by | Checked by |
|---|---|---|---|
| CentOS Stream 10 base | `resources/platforms/centos-stream-10.yaml` (tag + digest) | Renovate Docker PR, then a person reviews the build/boot | `validate`/`render`, image build, boot/session checks, scan |
| RHEL 10 base | `resources/platforms/rhel-10.yaml` (tag + digest) | Hand; Renovate is disabled because the registry needs an entitlement | composition here; build/boot in the downstream organization |
| RPM repositories and GPG keys | `RpmRepository` resources | Hand, with the vendor | resolver probes `repomd.xml`; the build verifies signatures |
| Binary artifacts (CLI tools, extensions) | `BinaryArtifact` resources | Renovate proposes a release version; a person fills the SHA-256, which Renovate cannot compute | resolver downloads and hashes; image build |
| Runtime companion (`deskos`) | `resources/software/deskos.yaml` | Hand, from a released asset | checksum, release version check, image build; it must read the plan the same release renders |
| ISO builder (BIB) | `docs/installer-iso.md`, the ISO workflows | Hand, by digest | image build, boot check, ISO install test |
| Syft and Grype | `.github/workflows/supply-chain.yml` | Renovate proposes the installer tag | pinned installer, check in the same run |
| GitHub Actions | workflow files | Hand; the token has no workflows permission | action version review; no action runs with registry or signing credentials |
| Flatpak remotes and apps | `FlatpakRemote`/`FlatpakSet` resources | Hand, on the vendor's release | build and first-boot preinstall; **not** the resolver |

Known gaps: the RHEL base cannot be checked anonymously, so its check is
downstream; Flatpak remotes and apps have no URL/checksum probe like binary
artifacts; a version-only binary PR is incomplete until the checksum is filled.
Compilation makes no network request; everything above is checked in CI.

## CI job

`.github/workflows/supply-chain.yml` runs weekly and on `workflow_dispatch`. The
weekly run publishes; a dispatch publishes only with `publish=true` on `main`,
and otherwise is a dry look at a base bump.

It builds one image per platform from a checkout, one matrix leg per publishable
workstation reference, each with its own exceptions record and destination
repository. Each leg writes the declared SBOM, produces `installed.sbom.cdx.json`
with Syft and `cves.json` with Grype, checks the exceptions record, generates the
check configuration from it, fails on the chosen severity, compares the two bills
with `tests/supply-chain/compare.py`, and uploads everything. Both scan passes use
`tests/supply-chain/grype-raw.yaml` (`ignore: []`), so `grype.txt` and `cves.json`
always show everything; only the promotion check applies the exceptions.

A scheduled run has no dispatch inputs, so it uses the explicit `FAIL_ON` fallback
of `critical`, the same baseline as a dispatch's default. The weekly run cannot
silently weaken it.

### The scheduler

GitHub's scheduler can run hours late or drop a tick. `release-watchdog.yml` runs
daily and dispatches the publish with `publish=true` when `:stable` is more than
eight days old, so a missed tick costs a day, not a release.

### Freezing publication

Publication stops from the repository settings: set the variable `PUBLISH_FREEZE`
with a sentence saying why. The weekly build, the installer, the promotion and the
mirror then do nothing, and `:stable` keeps serving the last good digest
([tabletop](threat-model.md)).

### The installer

`.github/workflows/iso.yml` is a workflow of its own: the weekly build calls it
with the digest it just published, and a dispatch can do the same. It is separate
so that an image which passed its checks is published and promoted whether or not
its installer built, and a retry costs one ISO build. The promotion reads the
`publish` job's conclusion, not the run's.

It verifies the image's keyless signature before building, then signs the installer
with its own workflow identity:
`…/deskos-core/.github/workflows/iso.yml@refs/heads/main`. The OIDC token names the
file that holds the job, so a verifier that pins the wrong identity rejects a valid
installer. It tags `sha256-<image digest>` for promotion, and moves its own
`:stable` when the image is already stable.

```bash
# The installer for the digest :stable serves.
cosign verify quay.io/deskos/deskos-core-centos-stream-10-iso@sha256:… \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity 'https://github.com/deskosproject/deskos-core/.github/workflows/iso.yml@refs/heads/main'
```

The ISO is signed, not attested: a suffix on a 3 GB disk image would claim what the
pipeline does not measure. Its `org.deskos.image.digest` annotation names the signed
image it carries.

### Results kept from a failed build

The workflow keeps the local image ID, the OCI archive checksum, the disk checksum
and the full `bootcheck`, `session`, `firstboot` and `journal-failed-boot`
directories for 14 days, QCOW2 files excluded because the checksums identify them.

`compare.py` checks **RPM packages**: every declared `deskos:source=rpm` component
must appear, by name, in the installed SBOM. Binaries, RPM files, Flatpaks and
trust anchors are reported but not checked, because a scanner does not enumerate all
of them; `generated-manifest.json` and the boot/session checks cover those.

The job first ran on 2026-10-06 (run `37466048646`, success, 0 Critical). Syft and
Grype are pinned in the workflow before the check can block a release.

## Severity policy and VEX

The policy the check applies is
[ADR 0012](adr/0012-vulnerability-severity-and-vex-policy.md):

- **the blocking threshold is `Critical`**, by the vendor's severity for a package
  the distribution ships (ADR 0012, amended 2026-10-29); an NVD score that differs
  is recorded beside the match and does not move it. A digest with an unresolved
  vendor Critical is not promoted;
- **High is tracked, not blocking** by default: a GNOME desktop with a browser and a
  distribution kernel always carries many, mostly backported or not reachable;
- **vendor-controlled components** (the kernel, distribution packages) follow the
  distribution's security response, and a Critical in them still blocks;
- **every exception is explicit**: an OpenVEX statement
  (`tests/supply-chain/vex.openvex.json`) or an entry in the reviewed record
  `tests/supply-chain/exceptions.json`. Nothing is silently ignored.

The record names the owner, the review date, the base image digest it was reviewed
against and, per exception, the matches it covers (package, type, version,
location). `tests/supply-chain/check_exceptions.py` enforces it between the raw
report and the check: an expired review, a moved base layer, a new component, a new
version or an unapproved Critical fails the build. `--emit-gate` writes the Grype
configuration from the record, so the check cannot accept more than the record
approves. The record is attested on the published digest, with a second attestation
of the matches this image accepted; both are verified by digest, signature and
predicated type.

CentOS Stream 10 baseline from that first run (Grype, distro `centos-10`):
0 Critical, 1866 High, 26735 Medium, 10263 Low, 124 Unknown, mostly the kernel (one
entry per subpackage) and Firefox.

## Artifact identity

Each leg writes `identity.json`, a small canonical record, not an SBOM or a
replacement for `/etc/os-release`. It carries:

- source commit, workflow, run, attempt and job;
- platform and workstation;
- the declared plan and generated manifest, by SHA-256;
- the local Podman image ID and digest;
- the exported OCI archive, the tested QCOW2 or raw disk, by SHA-256;
- the published manifest digest and reference, when `publish=true`;
- the signature-verification file and the boot, session and first-boot summaries.

`tests/supply-chain/identity.py` writes it with sorted keys and records every
missing entry instead of inventing a value. The workflow uploads it even on failure,
and a successful leg runs `--check`. A local OCI archive digest and a registry
manifest digest are recorded separately: compression and registry conversion can
change the latter.

## Signing and provenance

The publish step attests **SLSA v1 provenance** (predicate type
`https://slsa.dev/provenance/v1`, subject: the image digest): the source commit, the
base image digest the plan pins, and this workflow as the builder. The building
pipeline produces it, so it claims SLSA's Build L2 shape, not a dedicated L3
builder's isolation.

The same workflow, with `publish` on `main`, signs the scanned image **keyless**
with [cosign](https://docs.sigstore.dev/): Fulcio issues a short-lived certificate
bound to the workflow's OIDC identity, and Rekor records the signature. Four
attestations are attached: the provenance, a reference to the installed SBOM (digest,
format, SHA-256, location), the reviewed exceptions, and the matches this image
accepted. Rekor caps an attestation at 100 KB, so the SBOM is not attached; the
statement names its SHA-256 ([evidence design](design/evidence-retention.md)). What a
consumer needs is [the handoff contract](design/handoff.md), which names the gaps,
including that the index and the approval record are unsigned.

**Verify a digest.** A signature is necessary but not sufficient: require the four
attestation types, pin the issuer and the workflow identity, and compare the SBOM
you obtained against the hash the reference names. Do not use
`--insecure-ignore-tlog` here. Verification is not enforcement either: the machine
accepts what its `containers-policy.json` allows. Core chose keyless signatures
with manual verification on 2026-10-25; an organization that wants enforcement
keeps key-based signing ([authenticity](design/authenticity.md)).

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

**Rescan a published digest.** The weekly job builds from the checkout and does not
rescan older digests, listed in
[architecture.md](architecture.md#validation-status); RHEL digests belong to the
downstream organization. To review one:

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
three routes and records it on an issue with the digest, scanner/database versions
and the matching package, type, version and location:

1. **Revalidate**: prove the finding does not apply (a reachability argument, or a
   vendor advisory that shows the shipped version is the fixed one), and record
   the result; do not add a blanket ignore.
2. **Exception**: add an exact, owned and dated entry to
   `tests/supply-chain/exceptions.json`, with a review date that forces the
   question to return (ADR 0012).
3. **Replace or withdraw**: publish a fix or stop recommending the digest. A
   deleted tag or signature does not revoke copies already pulled or cached;
   consumers move by changing the reference or pin.

The raw report and the effective exceptions belong with the issue; artifact
retention is 14 days, so use the digest-keyed records under `evidence.deskos.org`
for a published image.

**RHEL builds** run in a downstream organization's pipeline, which owns the
entitled build host, with no OIDC: they are signed with a cosign key kept outside
this repository and verified against its public key. That key's rotation, and the
fact that verification without a transparency log proves less than the keyless
path, are recorded with the organization.

`supply-chain.yml` is the single publish path, and it boots, scans, signs and
attests the same image before it is promoted. The weekly run publishes by itself;
a dispatch with `publish=true` does the same on demand. `vm-bootcheck.yml` is a
pure check.

The matrix runs with `fail-fast: false`, so one platform failing does not stop
another: promotions are independent per platform, each checked on its own build,
boot, session and first-boot checks. A red leg means that platform did not move; it
does not retract a platform that already did. A `publish=true` dispatch has built,
scanned, booted, pushed and signed one digest per platform, listed in
[architecture.md](architecture.md#validation-status) with its run.

## Before accepting a risk, check whether the base moved

A run that fails on Criticals inside the **base** is not asking for an exception
first. The vendor may have published a new bootc base that fixes them, and
re-pinning to it accepts nothing: scan after the change and let the result decide.
Try this before writing an exception, which ships a known Critical.

It does not always work. CentOS Stream 10's `:stream10` served a manifest with the
same config as the pin (same build, re-pushed), so re-pinning changed nothing. The
check on 2026-10-28 found the same three Criticals the record accepts
(`GO-2026-6604`, `GO-2026-6612`, `CVE-2026-114406`) against
`sha256:474b191a…` (2026-10-26). The acceptance stands until the distribution
rebuilds those Go binaries or the review date arrives.

## Promotion check

A digest moves forward only when, for that digest:

1. the render validates and the boot, session and first-boot checks pass (see
   [development.md](development.md) and `tests/vm`);
2. the installed SBOM contains every component the declared SBOM lists;
3. the CVE scan is at or below the agreed threshold;
4. the image carries a signature over its digest.

Steps 1 to 4 run in `supply-chain.yml`. Promotion is a separate, recorded move:

- the publish step writes `:<commit>`, `:latest` and `:candidate`, all by digest.
  `:candidate` is the digest a promotion checks, `:latest` is the fast lane for
  the newest build, and `:stable` is the recommended reference;
- `promotion.yml` is the only writer of `:stable`. The first promotion (run
  `40317904401`, 2026-10-26) moved `sha256:96fe80d3…`, with its record in
  `approvals/deskos-core-centos-stream-10/stable/`; a second, after the base bump
  (run `40412048361`), moved `sha256:649aff75…`. It checks that the source channel
  serves the digest, verifies the keyless signature, fetches and verifies every
  object the digest's index names, stores an approval record under
  `approvals/<repository>/stable/` in the public evidence bucket, and only then
  moves `:stable` by digest;
- the record names the digest, the previous stable digest, the actor, the
  approver, the reason, the index hash and the run.

This is the factory layer around the compiler, and it never enters compiler
semantics (`AGENTS.md`, invariant 11). No canary or pilot channel exists.
