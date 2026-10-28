# Conforma against a DeskOS image: what it read, and what it wanted

**Run 2026-10-29**, Conforma CLI `v0.10.42` (ECC `v0.1.7`, OPA `v1.15.2`), against
`quay.io/deskos/deskos-core-centos-stream-10@sha256:871e59cc…`, with the policy
library at `github.com/enterprise-contract/ec-policies` (the CLI pinned commit
`cb839dc5`), and the keyless identity of the publish workflow.

```bash
ec validate image \
  --image quay.io/deskos/deskos-core-centos-stream-10@sha256:871e59cc… \
  --policy ./policy.yaml \
  --certificate-identity 'https://github.com/deskosproject/deskos-core/.github/workflows/supply-chain.yml@refs/heads/main' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

where `policy.yaml` is a policy configuration with the `release` rules and the
`lib` they import. No compatibility is claimed beyond what this run shows.

## Result

```
Success: false   Result: FAILURE
Violations: 14   Warnings: 6   Successes: 155
```

### What it did consume

- **The signatures.** It fetched the image's attestations and verified the
  keyless certificate **through Rekor**. It does **not** accept
  `--ignore-rekor` here: without the log it cannot verify a short-lived
  certificate after it expires, and it fails with "failed to verify leaf
  certificate". A consumer must not skip the log.
- **Our SLSA v1 predicate.** The provenance was found and parsed — the
  attestation-type check did not fail for absence. The failures around it are
  about the *build type*, not the predicate type.
- **155 individual checks** of the release set, including the identity and
  signature rules.

### What it wanted that we do not produce

| Violation | Reason it gave | What it means |
|---|---|---|
| `attestation_type.pipelinerun_attestation_found` | "Missing pipelinerun attestation" | the policy expects a Tekton `PipelineRun` attestation |
| `rhtap_multi_ci.attestation_found` | "A SLSA v1 provenance with one of the RHTAP Multi-CI build types was not found" | our `buildType` is `https://deskos.org/build/supply-chain/v1`, not a Tekton/Konflux one |
| `slsa_build_scripted_build.image_built_by_trusted_task` | trusted task data absent | it wants the build to name a trusted Tekton task |
| `slsa_source_correlated.*` (2) | "Expected source code reference was not provided" | it wants a source URI plus digest in the provenance; ours carries the commit in `resolvedDependencies` |
| `cve.cve_results_found` | "CVE scan results were not found" | it expects a vulnerability attestation; ours is evidence in the index |
| `sbom.found` | no SBOM attestation it recognises | ours names the SBOM by hash in a custom predicate type |
| `base_image_registries.base_image_info_found` | "Base images information is missing" | it expects the base in a Tekton-shaped field |
| `source_image.exists` | — | a rule that assumes the image was built from an image (a rebase), not from a plan |

### What it wanted and we simply did not supply

These are **configuration**, not artifacts: `base_image_registries.allowed_registries_provided`,
`rpm_signature.rule_data_provided`, `rpm_repos.rule_data_provided`,
`trusted_task.data`, `external_parameters.pipeline_run_params_provided`. A real
evaluation passes them with `--extra-rule-data` or a policy configuration; this
probe did not, on purpose, to see which ones are mandatory.

### The warnings worth reading

`github_certificate.gh_workflow_extensions` fired five times: the certificate
carries none of the *GitHub Workflow Name / Ref / Repository / SHA / Trigger*
extensions the policy looks for. That is a property of how the keyless
certificate was requested, not of this image, and it is the kind of thing an
organization pinning policy would want to know before relying on those
extensions for anything.

## What an adapter would need

1. **Rule data**: allowed registries, RPM repositories and their keys, a pipeline
   identifier and its parameters. Configuration, not code.
2. **A provenance build type** the policy recognises, or **a custom policy** that
   accepts `https://deskos.org/build/supply-chain/v1` and reads the base image and
   commit from `resolvedDependencies`. Our fields are there; the *names* are ours.
3. **An SBOM and a CVE attestation** in a type the policy knows, or rules that
   read the evidence index. Today both live in the index and are referenced by a
   custom predicate.
4. **Not** a change to the compiler: nothing here asks DeskOS to build with Tekton.

## Conclusion

Conforma **can consume** a DeskOS image's keys and provenance: it verified the
signature, found the SLSA v1 predicate and ran 155 checks. The **release policy
set** is written for Tekton/Konflux evidence, so most of its violations are about
the shape of the evidence rather than its absence. Adopting it means writing a
policy (or rule data) for our build type and evidence layout, and no more than
that is claimed here.
