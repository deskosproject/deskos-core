# Who answers for a secure installation

A machine that boots is not a machine that boots safely, and the parts that make it
safe belong to different owners. This page maps Secure Boot, TPM 2.0, LUKS,
measured boot and factory provisioning to the layer that answers for each, states
the combinations DeskOS does not support, and plans the lab that would turn an
untested combination into a measured one.

## The layers

| Layer | Answers for |
|---|---|
| Firmware | the Secure Boot key hierarchy (PK, KEK, db), the TPM it exposes, the measurements it records |
| Vendor | the signed chain it publishes: shim, GRUB, kernel, and on RHEL the bootc base |
| Hardware | a TPM 2.0 if there is one, the disk, the firmware update path |
| Organization | its own keys and MOK enrolments, the LUKS passphrase and its escrow, the TPM policy, factory provisioning |
| DeskOS Core | the artifact and its records. It does not sign boot components, hold keys, or encrypt disks |

## What is true today

- **Secure Boot is off in every installation.** DeskOS signs no boot component, so
  the firmware verifies the vendor's chain or nothing. RHEL 10 ships a signed
  chain; **CentOS Stream 10 does not boot with Secure Boot on**, because its shim is
  still blocked in Microsoft's review ([roadmap](../roadmap.md#secure-boot-proposed-2026-10-26)).
- **No TPM is exercised.** The VM harness boots without one, so no test in CI
  covers a TPM path. The image carries `tpm2-tools` 5.7, `tpm2-tss` 4.1.3,
  `cryptsetup` 2.8.6, `systemd` 257, `systemd-cryptenroll`, `bootctl` and `mokutil`;
  `tpm2-pkcs11` is **not** installed, so the TPM-backed 802.1X key has nothing to
  talk to ([ADR 0027](../adr/0027-a-network-profile-is-typed-and-its-secrets-are-not.md)).
- **LUKS is the installer's decision, not the image's.** The published ISO leaves
  storage to the person installing; the config-less kickstart deliberately avoids
  `autopart --encrypted`, because a passphrase in an ISO is a passphrase in clear
  text ([installer-iso.md](../installer-iso.md)).
- **Recovery covers boot, not keys.** [recovery.md](../recovery.md) says how to boot
  the ISO and repair a machine; recovering an encrypted disk without its passphrase
  is not a procedure this repository has.
- **Factory provisioning is deferred.** The network install recipe exists
  ([P1-00](../future-backlog.md)) and has not been run against a Satellite and a
  fleet.

## The matrix

| Capability | Firmware | Vendor | Hardware | Organization | Core |
|---|---|---|---|---|---|
| Secure Boot verification | verifies | signs shim, GRUB, kernel | | own keys and MOK | ships nothing signed; documents Secure Boot off |
| Secure Boot key ownership | db, KEK, PK | | | owns and rotates, or enrols a MOK | none |
| TPM 2.0 | exposes it | | provides it | sets the policy (PCRs, PIN) | no use today |
| Measured boot | records measurements | | | defines attestation and what it trusts | nothing |
| LUKS | | installer support | the disk | passphrase, escrow, recovery | never encrypts, never stores a passphrase |
| Factory provisioning | | | | owns PXE, kickstart, registry credentials | publishes the ISO and the OCI artifact |
| Recovery | boot menu | | | key and passphrase recovery | ISO recovery documented; keys are not |

## Unsupported, said plainly

| Combination | State |
|---|---|
| Secure Boot on, CentOS Stream 10 base | does not boot; the shim is not signed by Microsoft yet |
| Secure Boot on, RHEL 10 base | the vendor chain exists, and no DeskOS test has booted it with Secure Boot on |
| LUKS with a bootc upgrade or rollback | unmeasured; `bootc` owns the deployment and this repository has no record either way |
| TPM-backed LUKS unlock | not shipped: no `tpm2-pkcs11`, no TPM in CI, no policy |
| TPM-backed 802.1X key | not shipped, for the same reasons plus the SELinux module ADR 0027 describes |
| Measured boot and attestation | not implemented, and no owner |
| Secure Boot on aarch64 | untested; aarch64 itself is [P3-04](../future-backlog.md) |

## The lab plan

Equipment: QEMU with OVMF and its own Secure Boot variable store, `swtpm` for a
software TPM, an entitled RHEL 10 build, and one physical machine for the firmware
that emulation cannot imitate.

| Test | What it proves |
|---|---|
| Boot with Secure Boot off | the baseline every claim starts from (already green) |
| Boot a RHEL 10 image with Secure Boot on | whether the vendor chain is enough without a DeskOS signature |
| Boot a CentOS Stream 10 image with Secure Boot on | that it fails, so the documentation is right |
| `systemd-cryptenroll` against `swtpm`, then unlock | that a TPM-bound LUKS volume is a path this image can carry |
| LUKS plus a bootc upgrade and a rollback | whether an encrypted machine survives the update the artifact promises |
| Recover an encrypted machine from the ISO | that recovery is a procedure and not a hope |

What the lab cannot prove: the customer's firmware, their key ceremony, their
escrow, and their TPM policy. Those stay with the organization, which is the point
of the matrix.

## What would change this page

- DeskOS signing boot components would need a key, a CA and a review process; Core
  does not hold signing material (AGENTS.md, "Requires maintainer approval").
- An organization enrolling its own key and MOK changes the first row of the
  matrix, not the model.
- A vendor base that boots with Secure Boot on and a lab that measures it turns
  one "untested" into a "validated", which is the only change this page is waiting
  for.
