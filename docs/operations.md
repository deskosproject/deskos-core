# Operating an installed machine

How a DeskOS workstation updates, and how to inspect, apply and roll back that
update with what the image ships. Every Core image carries `/usr/bin/deskos`
(ADR 0018), so start there; `bootc` and `systemctl` are underneath it and remain
the fallback.

```bash
deskos report        # what this machine was built from, read from the plan it carries
deskos check         # compare the plan's declared scope with the machine, and say what it leaves out
sudo deskos update   # update the image, after a guard that refuses only what is unsafe
```

`deskos` is the runtime companion to `deskosctl`: it reads the plan the image
carries, so a machine can be asked what it was built from. It is **not** a
controller and has no reconciliation loop (ADR 0001), and it deliberately has no
`status` or `rollback` subcommand — `bootc` already owns those. The pin is
`resources/software/deskos.yaml`, and it moves like any other pin: a release
publishes the asset, and the pin is bumped to it deliberately.

`update` runs `bootc upgrade` underneath, which changes the deployment the machine
boots from, so it needs root. `report` and `check` only read and need none.

`check` measures **what the artifact declares** — the files it installs and
generates, the dconf defaults it sets — and prints its scope on every run. It is
not an audit of everything the machine has become.

When an install, a first boot or an update does not work, start with
[recovery.md](recovery.md): it lists the symptom, the evidence to collect and
the remedy.

## How updates work

A DeskOS image **stages** its next version. By default it never reboots your
session on its own. An organization can opt in to a scheduled apply and reboot
with `image.apply`; that is an explicit maintenance decision, not the default.
The image carries two base services and their timers, on the schedule the
effective `UpdatePolicy` declares (DeskOS Core: daily, on AC power):

| Unit | Runs | Effect |
|---|---|---|
| `deskos-image-update.timer` → `deskos-image-update.service` | `bootc upgrade --quiet` | downloads the next image and **stages** it |
| `deskos-flatpak-update.timer` → `deskos-flatpak-update.service` | `flatpak update --system --noninteractive --assumeyes` | updates system Flatpak apps and runtimes |
| `deskos-image-apply.timer` → `deskos-image-apply.service` (opt-in) | `deskos window`, then `bootc upgrade --apply --quiet` | applies the staged image and **reboots** inside the declared time |

> **Platform scope.** Everything `bootc` here is on every shipped platform. The
> Flatpak half exists only where the image declares a system Flatpak channel: the
> EL family's `PackageSet/rhel-family` provides one, and DeskOS Core adds neither
> the package nor the unit without a profile that selects it; an organization can
> still declare them itself. A platform that declares no channel fails composition
> for a `FlatpakSet` rather than pretending. The example AlmaLinux and Fedora
> examples are not part of the shipped support statement
> ([platforms.md](platforms.md)).

The timers are `OnCalendar=daily` with a two-hour randomized delay and
`Persistent=true`; the services carry `ConditionACPower=true`. The base
image's own `bootc-fetch-apply-updates.timer` and `.service` are **masked**,
so the image owns updates rather than the base
(see [ADR 0007](adr/0007-updatepolicy.md)).

The staged image **applies at the next reboot**. An organization changes
the schedule, or turns automation off, in its own `UpdatePolicy`
(`image.automatic`, `image.schedule`, `image.requireACPower`, and the
`flatpak` equivalents). With `automatic: false` the service is still
written, but no timer, so an operator (or `deskos update`) can run it
on demand. The kind is documented in [resources/system.md](resources/system.md#updatepolicy).

An organization that enables `image.apply` accepts scheduled reboots. The
apply timer is not persistent: if the machine is off during the window, it
waits for the next one. The service first runs `deskos window`, which checks
the local weekday and `at`/window (and optional IANA timezone), so a timer
that fires after suspend outside the interval does nothing. An exact `at`
allows five minutes of grace; a `window` allows its full duration. There is no
active-session check in this version: the window is the maintenance contract.
Inspect the apply units with `systemctl list-timers 'deskos-*'` and
`journalctl -u deskos-image-apply.service`.

## What happens in each situation

| Situation | Result |
|---|---|
| automatic on, on AC power, at the daily run | stages the next image; the running session is untouched |
| automatic on, on battery when the timer fires | `ConditionACPower` skips the service; nothing is staged. There is no catch-up when AC power returns: the next timer run does it |
| automatic off | no timer is installed; `deskos update` or `systemctl start deskos-image-update.service` stages on demand |
| staged, `image.apply` off or not configured | the image waits; it applies at the next reboot, whenever the person chooses |
| `image.apply` on, staged, inside the declared window | `deskos window` passes, the service runs `bootc upgrade --apply` and reboots |
| `image.apply` on, outside the window | `deskos window` fails, so the service does not apply and the machine keeps running; the apply timer is not persistent, so the next window is the next chance |
| machine off during the window | no catch-up on boot; the next window applies |
| registry unreachable or no network | `bootc upgrade` fails, the journal records it, and the running deployment is unchanged; the next run retries |
| download interrupted | no staged deployment is recorded; the running image is unchanged |
| metered connection | not detected: the timer runs as on any other connection ([not supported](architecture.md#updates)) |
| Flatpak updates | a separate timer and service, independent of the image update |
| `deskos update` | stages the image only: no reboot, no Flatpaks, no firmware, no rollback, and no reconciliation of `/etc` |

## Inspect

```bash
bootc status                        # booted and staged deployments, image ref
systemctl list-timers 'deskos-*'    # when the next update runs
systemctl status deskos-image-update.service
journalctl -u deskos-image-update.service --since today
```

On the EL platforms:

```bash
rpm-ostree status                   # the same, in ostree terms
flatpak list --system               # system apps and runtimes
```

`bootc status` shows a `Staged` entry once the next image is downloaded and
waiting for a reboot; `bootc status --verbose` says whether it is in
download-only mode.

## Update now, then apply

```bash
sudo systemctl start deskos-image-update.service     # stage the next image
sudo reboot                                          # apply the staged image
```

To check and stage in one step, without waiting for the timer:

```bash
sudo bootc upgrade
sudo reboot
```

`deskos update` is the same staging step behind a guard: it refuses when this is not a
bootc deployment or when bootc cannot read the deployment, and it reports — without
refusing — an image that updates itself and whatever the plan owns under `/etc`, since
customization there is the machine owner's.

On the EL platforms only, the Flatpak half:

```bash
sudo systemctl start deskos-flatpak-update.service   # update Flatpaks now
```

`deskos update` stages the image and nothing else. System Flatpaks and
user-level tools are separate layers, updated by their own commands:

```bash
sudo deskos update                                  # the image, staged
sudo systemctl start deskos-flatpak-update.service  # system Flatpaks
mise upgrade                                        # example user-level tools
```

The last line is an example: a user-level tool manager changes `$HOME` and runs
as the person, never under `sudo`. `deskos update` does not touch a home or
update Flatpaks, user or system.

## Roll back

Every applied image keeps the previous deployment. To boot it:

```bash
sudo bootc rollback
sudo reboot
```

`bootc rollback` reorders the boot entries: the deployment marked `rollback`
becomes the next boot and the current one becomes the rollback, so rolling
back again returns you. Files the image owns under `/etc` revert with the
deployment; a local edit under `/etc` is kept (the merge is three-way, not a
snapshot), and `/var` (including home directories) is preserved. The lifecycle
check asserts both `/etc` cases on every run
([design](design/lifecycle.md#shape)).

A rollback does not undo data that the newer version already migrated under
`/var`, and it is not automatic: choosing the previous deployment is a manual
step (or the firmware entry for it after a candidate that does not boot).
Restore application data from your own backup; the image ships no backup tool.

## Change where updates come from

A machine tracks the image reference it was installed from. To move it to a
different one (a new registry, a pinned digest):

```bash
sudo bootc switch registry.example.com/deskos/core-rhel-10:latest
sudo reboot
```
