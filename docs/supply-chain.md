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
deskosctl sbom ./resources ./examples/example-org \
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
and `cves.json` with Grype, fails on the chosen severity, compares the two
bills with `tests/supply-chain/compare.py`, and uploads all evidence.

`compare.py` gates **RPM packages**: every declared `deskos:source=rpm`
component must appear, by name, in the installed SBOM. Binaries, RPM
files, Flatpaks and trust anchors are reported but not gated there — a
scanner does not enumerate all of them, and `generated-manifest.json` and
the boot/session checks cover those.

The job is **experimental and has not been dispatched yet**. The first run
establishes the real CVE baseline, and the Syft/Grype versions must be
pinned before this gate blocks a release.

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
