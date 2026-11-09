# Installing the published ISO

The installer the download page offers was installed from the bytes the page
names, and the result is here. Since v0.25.2 it asks for the language and the
disk, installs, creates the first account at GNOME Initial Setup and boots the
installed system on its own.

## The artifact

| Field | Value |
|---|---|
| File | `deskos-centos-stream-10-20261109-8b73cad52c91.iso` |
| SHA-256 | `32051bc4efea42458a5c7bdea577c476f363548fd3e0af63bb162a49d2a6634e` |
| Image | `sha256:65ff69215ce0dee70ec93664a71cd54cef4fc18c82d593d3824c73b4959fe3e3` |
| OCI artifact | `sha256:8b73cad52c91fa222634bbe3312285adfeb51af11669f22fdc6e7a96db8abfa3` |

Pulled on 2026-11-09 from
`quay.io/deskos/deskos-core-centos-stream-10-installer`, the signed artifact
`:stable` names; `sha256sum` matches the file. The mirror host was not
reachable from the machine that ran the install, and the OCI artifact is the
same bytes the mirror copies.

## The machine

QEMU 11.1.3 on Fedora 45, `q35` with UEFI through
`/usr/share/edk2/ovmf/OVMF_CODE.fd`, 4 GiB of RAM, 4 vCPU, one 40 GiB virtio
disk and one CD-ROM, user-mode network. No physical hardware.

## What happened

1. The GRUB menu boots `Install CentOS Stream 10`, whose command line carries
   `inst.ks=hd:LABEL=...:/osbuild.ks`.
2. Anaconda asks for the language, then shows the summary. The keyboard, time
   zone and installation destination start incomplete.
3. The destination spoke lists the 40 GiB virtio disk, offers automatic or
   custom partitioning and the encryption checkbox. The disk is only touched
   after **Begin Installation**.
4. The installer deploys the embedded image, reboots, and reaches GNOME Initial
   Setup. The account is `DeskOS QA` with the user name `deskosqa`; no person is
   named.
5. The desktop starts with that account. The machine powers off, the ISO is
   detached, and it boots from the disk alone: the greeter lists `DeskOS QA`,
   the password opens the session.

| Step | Capture |
|---|---|
| Language | [installation-language.png](images/qa/published-iso/installation-language.png) |
| Summary with an incomplete destination | [installation-summary.png](images/qa/published-iso/installation-summary.png) |
| Destination and encryption | [installation-destination.png](images/qa/published-iso/installation-destination.png) |
| Progress | [installation-progress-v0252.png](images/qa/published-iso/installation-progress-v0252.png) |
| Initial Setup | [initial-setup-v0252.png](images/qa/published-iso/initial-setup-v0252.png) |
| Session | [session-v0252.png](images/qa/published-iso/session-v0252.png) |
| Greeter after a disk-only boot | [greeter-v0252.png](images/qa/published-iso/greeter-v0252.png) |

## Secure Boot

The same ISO was installed again in QEMU/OVMF with UEFI Secure Boot enabled:
the secboot firmware, q35 with SMM, the pflash device marked secure, and the
enrolled variable store (Ubuntu's `.ms.fd`, which carries the PK/KEK/db and
`SecureBootEnable=1`). The firmware accepted the installer's signed chain, the
install completed, GNOME Initial Setup and the session ran, and a disk-only boot
with the same firmware reached the greeter and logged in.

The chain is the vendor's: shim signed by Microsoft's UEFI CAs, GRUB and kernel
by CentOS. The RHEL flavor builds downstream and was not tested in this mode.

| Step | Capture |
|---|---|
| Language under Secure Boot | [secure-boot-language.png](images/qa/published-iso/secure-boot-language.png) |
| Session under Secure Boot | [secure-boot-session.png](images/qa/published-iso/secure-boot-session.png) |
| Greeter after a disk-only boot | [secure-boot-greeter.png](images/qa/published-iso/secure-boot-greeter.png) |

## What the first install found

v0.25.1 carried bootc-image-builder's default kickstart, because
`installer-build.yml` passed no `--config`. That kickstart ran
`clearpart --all` and `autopart`, asked nothing and erased every disk the
installer saw. The page and [installer-iso.md](installer-iso.md) described an
interactive installer, so the install found a mismatch between the artifact and
its documentation.

v0.25.2 builds the ISO with [installer/config.toml](../installer/config.toml):
`graphical`, `rootpw --lock`, `reboot`, and the Localization and Timezone
modules. The published ISO's `/osbuild.ks` now carries those three lines and no
storage answer, and the screens above are the result.

The v0.25.1 captures stay as the record of the finding:
[installation-progress.png](images/qa/published-iso/installation-progress.png)
and [initial-setup-welcome.png](images/qa/published-iso/initial-setup-welcome.png).

## What it does not prove

- LUKS, a second disk, or any physical machine.
- The RHEL flavor, which builds downstream.
- An upgrade of the installed machine; this record ends at the first session.
