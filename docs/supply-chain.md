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
  --workstation example-devops-rhel10 \
  --output dist/example-devops-rhel10.sbom.cdx.json
```

### The installed SBOM and the CVE scan

The authoritative SBOM and the vulnerability scan run on the **built
image**, because only then do package versions and file contents exist:

```bash
# A built image, for example from `make build`.
image=localhost/daytwo-devops-rhel10:latest

syft "$image" -o cyclonedx-json > installed.sbom.cdx.json
grype "$image" -o table
grype "$image" -o json > cves.json
```

`grype` fails non-zero when the configured severity threshold is met
(`--fail-on critical`), which is what a CI gate uses.

## CI job

`.github/workflows/supply-chain.yml` (manual, `workflow_dispatch`) builds
the CentOS Stream 10 Core image from a checkout and runs the steps above:
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
The Syft/Grype versions must be pinned before this gate blocks a release.

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
issues a short-lived certificate bound to the workflow's OIDC identity,
Rekor records the signature, and the **installed** SBOM is attached as a
CycloneDX attestation. The digest is the unit of promotion.

```bash
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/deskosproject/deskos-core/' \
  quay.io/deskos/deskos-core@sha256:…
cosign verify-attestation --type cyclonedx \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/deskosproject/deskos-core/' \
  quay.io/deskos/deskos-core@sha256:…
```

Keyless signing needs a GitHub Actions OIDC identity, so it covers the
CentOS Stream 10 CI path. **RHEL builds run on the entitled factory host
with no OIDC**: sign them with a cosign key kept outside this repository
and verify against its public key.

The signing steps are **experimental and have not been dispatched**. The
gates are now unified: `supply-chain.yml` is the single publish path, and it
boots, scans, signs and attests the same image before it is promoted.
`vm-bootcheck.yml` is a pure check.

## Promotion gate (proposed)

A digest moves forward only when, for that digest:

1. `make check` and the boot/session checks pass (see
   [development.md](development.md) and `tests/vm`, `tests/rhel`);
2. the installed SBOM contains every component the declared SBOM lists;
3. the CVE scan is at or below the agreed threshold;
4. the image carries a signature over its digest.

Only the declared SBOM exists today (v0.8.0). Steps 2–4 are the next
stages of [ADR 0010](adr/0010-artifact-evidence-and-supply-chain.md). This
is the "factory" layer around the compiler — it never enters compiler
semantics (see `AGENTS.md`, invariant 11).
