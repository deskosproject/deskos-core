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
deskosctl sbom ./examples/baseline-and-role \
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

## CI job

`.github/workflows/supply-chain.yml` (manual, `workflow_dispatch`) builds
one image per platform from a checkout — one matrix leg per publishable
workstation reference, each with its own exceptions record and destination
repository — and runs the steps above:
it writes the declared SBOM, produces `installed.sbom.cdx.json` with Syft
and `cves.json` with Grype, checks the exceptions record and generates the
gate configuration from it, fails on the chosen severity, compares the two
bills with `tests/supply-chain/compare.py`, and uploads all evidence. The
two evidence passes are pinned to `tests/supply-chain/grype-raw.yaml`
(`ignore: []`), so `grype.txt` and `cves.json` always show everything;
only the gate applies the exceptions.

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

## Signing and provenance

The same workflow, with `publish` on `main`, pushes the scanned image and
signs it **keyless** with [cosign](https://docs.sigstore.dev/): Fulcio
issues a short-lived certificate bound to the workflow's OIDC identity and
Rekor records the signature. Three attestations are attached: a reference to
the **installed** SBOM (the image digest, the SBOM's format and SHA-256, and
where to find it), the reviewed exceptions, and the matches this image
accepted.

Rekor's public instance caps an attestation at 100 KB, and an SBOM is far
larger, so the SBOM itself is not attached: the signed statement names its
exact SHA-256, and the digest is the unit of promotion. The SBOM travels as
workflow evidence; durable, digest-indexed storage for it is open work.

**Verify a digest.** Checking the signature is necessary but not sufficient:
require the three attestation types, pin the issuer and the workflow
identity, and compare the SBOM you obtained against the hash the reference
names. Do not use `--insecure-ignore-tlog` on this path.

```bash
# One repository per platform and release:
# quay.io/deskos/deskos-core-centos-stream-10, -almalinux10, …
ref=quay.io/deskos/deskos-core-centos-stream-10@sha256:…
issuer=https://token.actions.githubusercontent.com
identity='^https://github\.com/deskosproject/deskos-core/'

cosign verify --certificate-oidc-issuer "$issuer" \
  --certificate-identity-regexp "$identity" "$ref"

for type in \
  https://deskos.org/supply-chain/sbom/v1 \
  https://deskos.org/supply-chain/exceptions/v1 \
  https://deskos.org/supply-chain/effective-exceptions/v1; do
  cosign verify-attestation --type "$type" \
    --certificate-oidc-issuer "$issuer" \
    --certificate-identity-regexp "$identity" "$ref"
done

# The reference names the image digest and the SBOM's SHA-256; read it and
# check the SBOM you obtained against it.
cosign verify-attestation --type https://deskos.org/supply-chain/sbom/v1 \
  --certificate-oidc-issuer "$issuer" --certificate-identity-regexp "$identity" "$ref" \
  | jq -r '.payload' | base64 -d | jq '{image, sha256, components}'
```

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
platform that already did. The matrix has **not** been dispatched with
`publish=true` yet, so the promotion it implements is gated and reviewed, not
demonstrated: the first such dispatch is its evidence.

## Before accepting a risk, check whether the base moved

A gate that fails on Criticals inside the **base** is not asking for an exception
first. AlmaLinux publishes a new bootc base every few days, and re-pinning to it is
the fix that accepts nothing: on 2026-10-17 the gate found Criticals in Go modules
compiled into `podman`, `skopeo`, `toolbox` and `ipp-usb`, and `quay.io/almalinuxorg/almalinux-bootc:10`
already served a base built that morning, three days newer than the pinned one.
The platform moved to it, and the scan after the change is what says whether it was
enough.

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

Steps 1–4 run in `supply-chain.yml` today; what is still open is promoting
the same digest through channels (candidate, canary, pilot, stable) without
rebuilding. This is the "factory" layer around the compiler — it never
enters compiler semantics (see `AGENTS.md`, invariant 11).
