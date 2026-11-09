# One install of the published ISO

The installer the download page offers was installed once, from the bytes the
page names, and the result is recorded here. It boots, installs and reaches
GNOME Initial Setup; it also erases every disk it sees without asking, which
the page did not say.

## The artifact

| Field | Value |
|---|---|
| File | `deskos-centos-stream-10-20261108-60d055d1aafb.iso` |
| SHA-256 | `bf01da7de75690248de121ddd7289a1fc55290b8f2bfcc0989095ae60ab96e7c` |
| Image | `sha256:7e30c662b9798efc2ae3dc22d053724ca5f2d42adb5b6e28c4289a020aa2899a` |
| OCI artifact | `sha256:60d055d1aafb892be5d3f2b9731dc5a842f4ad97ca8154e035520d815d2d63b7` |

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
2. Anaconda goes straight to the installation progress screen, with no
   language, disk or LUKS step. It deploys the embedded image.
3. The machine reboots by itself and reaches GNOME Initial Setup.
4. The setup asks for privacy controls, the account and a password. The account
   is `DeskOS QA` with the user name `deskosqa`; no person is named.
5. The desktop starts with that account.
6. The machine powers off, the ISO is detached, and it boots from the disk
   alone: the greeter lists `DeskOS QA`, the password opens the session.

| Step | Capture |
|---|---|
| Installation progress | [installation-progress.png](images/qa/published-iso/installation-progress.png) |
| First boot, Initial Setup | [initial-setup-welcome.png](images/qa/published-iso/initial-setup-welcome.png) |
| Account creation | [initial-setup-account.png](images/qa/published-iso/initial-setup-account.png) |
| Session after setup | [session.png](images/qa/published-iso/session.png) |
| Greeter after a disk-only boot | [greeter-after-reboot.png](images/qa/published-iso/greeter-after-reboot.png) |
| Session after the reboot | [session-after-reboot.png](images/qa/published-iso/session-after-reboot.png) |

## What it found

The ISO carries bootc-image-builder's default kickstart, not the interactive
one this repository documents:

```
ostreecontainer --url=/run/install/repo/container
rootpw --lock
lang en_US.UTF-8
keyboard us
timezone UTC
clearpart --all
autopart --nohome --type=plain --fstype=xfs
```

`clearpart --all` erases every disk the installer sees. On the test machine
that was one empty disk; on a laptop with a second drive, that drive goes too.
The download page said the installation destination is editable, and
[installer-iso.md](installer-iso.md) describes a shipped ISO that asks for the
disk and LUKS. The published artifact does neither, because the build passes no
`--config` to `bootc-image-builder`.

Until the build passes the interactive configuration and a new ISO is
published, the page must warn that the installer is unattended and
destructive.

## What it does not prove

- LUKS, a second disk, or any physical machine.
- The RHEL flavor, which builds downstream.
- The installer with Secure Boot enabled.
- An upgrade of the installed machine; this record ends at the first session.
