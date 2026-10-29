# Recovering a workstation

What to do when an install, a first boot, an update or a rollback does not go as
expected, and what evidence to collect before changing anything. Normal operation
is in [operations.md](operations.md); the installer is in
[installer-iso.md](installer-iso.md). Nothing on this page phones home.

## The installer does not finish

| Symptom | Likely cause | What to do |
|---|---|---|
| Anaconda exits at the progress screen and the machine reboots | The kickstart used `graphical --non-interactive`, which RHEL 10.2's Anaconda treats as an error (`User interaction required on standalone spoke ProgressSpoke`) | Use an ISO whose kickstart does not pass the flag; the published kickstarts in this repository do not. The failure is visible in the installer console before the reboot |
| The summary says the kickstart is insufficient and **Begin Installation** stays disabled | The published ISO leaves storage to you; that is not a failure | Choose the disk and, if wanted, LUKS, on **Installation Destination**, then begin |
| The machine does not boot the ISO | Secure Boot is on, the medium is damaged, or the machine is not booting UEFI | Verify the published checksum, write the ISO again, boot UEFI with Secure Boot off |
| An unattended test install stops on the summary | The test-only ISO was not used | Build and boot the ISO from [tests/installer](../tests/installer/README.md); do not add its answers to the published ISO |

The installer asks for language, keyboard, timezone, network, the disk and LUKS.
It erases the disk you select. Back up first and disconnect other disks.

## First boot

| Symptom | Likely cause | What to do |
|---|---|---|
| A text login instead of GNOME | The installed system's default target is `multi-user.target`, or GDM did not start | `systemctl get-default` should say `graphical.target`; if it does not, the kickstart is missing `xconfig --startxonboot`. Check `systemctl status gdm` for a failed start |
| GNOME Initial Setup appears | Expected on an image with `rootpw --lock` and no user | Create the first account there. There is no root password to guess |
| A unit failed during boot | A service or timer did not reach its target | `systemctl --failed`, then `journalctl -u <unit> -b` |

## An update fails or waits

An update **stages** a new deployment; the running system is untouched until a
reboot, so a failed download does not take the machine down.

| Symptom | Likely cause | What to do |
|---|---|---|
| `deskos update` or the timer fails | Registry unreachable, DNS, credentials, or no free space | A manual run prints the error; for the timer read `journalctl -u deskos-image-update.service -b`. `bootc status` shows whether anything was staged; the previous deployment keeps running. Retry when the cause is fixed |
| The update is staged but nothing happens | No reboot yet, or `image.apply` is off | `bootc status` shows the staged deployment. Reboot when convenient, or let the configured apply window do it |
| A scheduled apply did not reboot | Outside the window, on battery, or the machine was off | The apply timer is not persistent and the service checks AC power and the window. Inspect `journalctl -u deskos-image-apply.service -b`; the next window is the next chance |
| The download fills the disk | Image layers plus the staged deployment need space | Free space under `/var`, then retry. `bootc status` says whether a deployment is staged |

A metered connection is not detected; the timer behaves as on any other
connection.

## The update did not boot

The previous deployment is still on the disk. Recovery is manual: DeskOS does not
switch back on its own.

1. In the boot menu, choose the previous entry. The menu lists the deployments
   the machine keeps.
2. If the machine boots but the new deployment is current, run
   `sudo bootc rollback` and reboot. That queues the previous deployment.
3. If no entry boots, boot a rescue environment (for example the installer ISO's
   rescue entry, if it has one), mount the installation, and use your backup.
   There is no automatic rescue.

After recovering, keep the evidence from the failed boot: `journalctl -b -1` from
the working deployment, `bootc status`, and the boot menu entries. A candidate
that does not boot is exactly the case the lifecycle check does not cover yet
([design](design/lifecycle.md)).

## Rollback is not a backup

`bootc rollback` reverts image-owned files under `/etc`; local `/etc` edits are
kept because the merge is three-way, not a snapshot. `/var`, home directories,
system Flatpaks and application data are **not** reverted. Data a newer version
already migrated is not undone by rolling back: restore it from your own backup.
The image ships no backup tool.

## Collect the evidence before you change state

```bash
deskos report                    # the plan the image carries
deskos check                     # declared scope versus the machine, and its limits
bootc status                     # booted, staged and rollback deployments
systemctl --failed               # units that did not reach their target
journalctl -b                    # this boot
journalctl -b -1                 # the previous boot, after a failed update
journalctl -u deskos-image-update.service -b
journalctl -u deskos-image-apply.service -b
```

Reports and feedback are user-owned and opt-in; see
[AGENTS.md](../AGENTS.md#agentic-contribution).
