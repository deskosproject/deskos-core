# Operating an installed machine

```bash
deskos report        # what this machine was built from, read from the plan it carries
deskos check         # compare the plan's declared scope with the machine, and say what it leaves out
deskos etc           # what differs between /etc and the image's baseline
deskos status        # what the host follows, what it booted, what is staged, what the rollback is
sudo deskos update   # update the image, after a guard that refuses only what is unsafe
sudo deskos update --flatpak   # update system Flatpak applications, then the image
sudo deskos update --all       # image, Flatpak applications and a firmware check
sudo deskos rollback # stage the previous deployment for the next boot
sudo deskos etc reset --all   # put /etc back to the image, with a backup
```

Every Core image carries `/usr/bin/deskos` (ADR 0018), the runtime companion to
`deskosctl`, with no controller (ADR 0001); `status` and `rollback` front `bootc`.
`update` runs `bootc upgrade`, so it needs root; `report` and `check` are read-only. `--firmware` lists updates with
`fwupdmgr get-updates` and never applies them; the pin is
`resources/software/deskos.yaml`. `check` measures what the artifact declares and
prints its scope on every run, and `report` says that enforcement by the running
session is not its measurement.

## What changed in /etc

On a deployment that merges `/etc`, `deskos etc` compares the running `/etc` with
the baseline at `/usr/etc` and lists modified, added and removed paths. It is
read-only, needs no root, and refuses without that baseline. Runtime state is not
an edit: NetworkManager connections, machine-id, SSH host keys, printer and
Bluetooth state, sockets and caches are ignored; `/etc/deskos/deskos.json` adds
patterns, and `/etc/deskos/` itself is ignored.

```bash
deskos etc                     # the differences, with a count of what was ignored
deskos etc --ignored           # name the ignored paths too
sudo deskos etc reset /etc/foo # put one path back to the image's version
sudo deskos etc reset --all --dry-run
```

`reset` copies the baseline back and keeps a timestamped copy under
`/var/lib/deskos/etc-backup/`; it never touches `/etc/deskos/` without
`--include-policy`.

## How updates work

Two services and their timers run on the `UpdatePolicy` schedule (DeskOS Core:
daily, on AC power):

| Unit | Runs | Effect |
|---|---|---|
| `deskos-image-update.timer` → `deskos-image-update.service` | `bootc upgrade --quiet` | downloads the next image and **stages** it |
| `deskos-flatpak-update.timer` → `deskos-flatpak-update.service` | `flatpak update --system --noninteractive --assumeyes` | updates system Flatpak apps and runtimes |
| `deskos-image-apply.timer` → `deskos-image-apply.service` (opt-in) | `deskos window`, then `bootc upgrade --apply --quiet` | applies the staged image and **reboots** inside the declared time |

> **Platform scope.** Everything `bootc` here is on every shipped platform. The
> Flatpak half needs a system Flatpak channel: the EL family's
> `PackageSet/rhel-family` provides one, and a platform with no channel fails
> composition for a `FlatpakSet`. The AlmaLinux and Fedora examples are not shipped
> support ([platforms.md](platforms.md)).

The timers are `OnCalendar=daily` with a two-hour randomized delay and
`Persistent=true`; the services carry `ConditionACPower=true`. The base image's
own `bootc-fetch-apply-updates.timer` and `.service` are masked
([ADR 0007](adr/0007-updatepolicy.md)).

An organization sets the schedule or turns automation off in its own `UpdatePolicy`
(`image.automatic`, `image.schedule`, `image.requireACPower`, and the `flatpak`
equivalents); with `automatic: false` the service exists but no timer does, so
`deskos update` runs it on demand ([system.md](resources/system.md#updatepolicy)).

With `image.requireACPower`, a udev rule starts the staging service when AC comes
online, and the service still checks `ConditionACPower`. The service first runs
`deskos window`, which checks the local weekday and `at`/window (and optional IANA
timezone), so a timer that fires after suspend outside the interval does nothing.
An exact `at` allows five minutes of grace, a `window` its full duration; no
active-session check exists, because the window is the contract.

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
| metered connection | not detected: the timer runs as on any other connection ([not supported](runtime.md#updates)) |
| `deskos update` | stages the image only: no reboot, no Flatpaks, no firmware, no rollback, and no reconciliation of `/etc` |

## Inspect

```bash
bootc status                        # booted and staged deployments, image ref
systemctl list-timers 'deskos-*'    # when the next update runs
journalctl -u deskos-image-update.service --since today
rpm-ostree status                   # on the EL platforms, the same in ostree terms
flatpak list --system               # system apps and runtimes, on the EL platforms
```

## Update now, then apply

```bash
sudo systemctl start deskos-image-update.service     # stage the next image
sudo bootc upgrade                                   # or check and stage in one step
sudo reboot                                          # apply the staged image
```

`deskos update` is the same staging step behind a guard: it refuses when this is
not a bootc deployment or bootc cannot read it, and reports, without refusing, an
image that updates itself and what the plan owns under `/etc`.

## Roll back

```bash
sudo bootc rollback
sudo reboot
```

`bootc rollback` reorders the boot entries: the deployment marked `rollback`
becomes the next boot, so rolling back again returns you. Image-owned files under
`/etc` revert; a local edit under `/etc` is kept (the merge is three-way, not a
snapshot), and `/var` and home directories are preserved. The lifecycle check
asserts both `/etc` cases ([design](design/lifecycle.md#shape)).

A rollback is not a backup: data migrated under `/var` is not undone, and the image
ships no backup tool ([recovery.md](recovery.md#rollback-is-not-a-backup)).

## Change where updates come from

```bash
sudo bootc switch registry.example.com/deskos/core-rhel-10:latest
sudo reboot
```
