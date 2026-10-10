# ADR 0007: UpdatePolicy as the eleventh public kind

**Status:** accepted (2026-10-04)

## Context

Both supported base images enable `bootc-fetch-apply-updates.timer`
through a symlink in `/usr/lib/systemd/system/default.target.wants/`
that no package owns (checked on `centos-bootc:stream10`, bootc 1.16.13,
and `rhel-bootc:10.2`, bootc 1.16.4). Its service runs
`bootc upgrade --apply --quiet`, and `bootc-upgrade(8)` says `--apply`
"currently always reboots the system". The timer fires one hour after
boot and every 8 hours after the last run, with a 2-hour random delay. A
workstation built on either base therefore reboots on its own whenever a
new image is published, possibly during a user's session.
`systemctl disable` does not remove a `/usr/lib` wants symlink; only a
mask stops it.

Plain `bootc upgrade` downloads the new image and queues it as a staged
deployment; `ostree-finalize-staged.service` applies it at the next
ordinary shutdown or reboot. `--download-only` instead locks the staged
deployment so a reboot discards it, and `--check` only fetches metadata.
Bluefin's `uupd` runs `bootc upgrade --quiet` (no `--apply`) and
`flatpak update -y --noninteractive`, from a timer with `Persistent=true`
and a random delay, with `ConditionACPower=true` in a drop-in; Bluefin LTS,
also built on CentOS Stream 10, masks both bootc units.

None of the ten kinds can hold this intent: it is not GNOME, boot or
platform configuration, and `Workstation` has no layering.

## Decision

Add `system.deskos.org/v1alpha1` `UpdatePolicy` with layered scalars:

| Field | Value |
|---|---|
| `image.automatic` | boolean |
| `image.schedule` | `daily` or `weekly` |
| `image.requireACPower` | boolean |
| `flatpak.automatic` | boolean |
| `flatpak.schedule` | `daily` or `weekly` |
| `flatpak.requireACPower` | boolean |

Each field is its own scalar: the highest layer wins and different
values at one layer conflict. An effective `automatic: true` needs a
`schedule`; `schedule` or `requireACPower` above or at the layer that
sets `automatic: false`, or with no `automatic`, is an error, and below
it is a masked-setting warning.

Lowering:

- Any effective `image.automatic` masks the platform's
  `updates.imageUpdateUnits` (`bootc-fetch-apply-updates.timer` and
  `.service` on both platforms), so DeskOS owns image updates.
- The **service** is written whenever an area has intent, even with
  `automatic: false`: `deskos-image-update.service` or
  `deskos-flatpak-update.service`. Only the **timer** follows `automatic`; a
  service with no timer runs on demand, from an endpoint command, without
  re-enabling automation. The Flatpak package is installed either way, because
  the service needs it.
- `image.automatic: true` adds `deskos-image-update.service`, which runs
  `/usr/bin/bootc upgrade --quiet`, and `deskos-image-update.timer`.
- `flatpak.automatic: true` adds the platform Flatpak package,
  `deskos-flatpak-update.service`, which runs
  `/usr/bin/flatpak update --system --noninteractive --assumeyes`, and
  `deskos-flatpak-update.timer`.
- Timers use `OnCalendar=daily` or `weekly`, `RandomizedDelaySec=2h`
  (the stock bootc spread) and `Persistent=true`, and are enabled through
  `timers.target`. `requireACPower` sets `ConditionACPower=true` on the
  service, which systemd checks when the timer fires.

Own units are chosen over a drop-in on the stock service because the
stock unit name and timer say "apply", its schedule is monotonic (no
`Persistent=`), and a drop-in that later lost its target unit would
silently bring back the rebooting behavior. Masking also covers
`automatic: false`, which a drop-in cannot express.

The commands are fixed by the backend from the IR kind (`image`,
`flatpak`); no field accepts a command, calendar expression or unit
name. `--apply`, `--soft-reboot` and `--download-only` are never
generated.

**Metered networks are not supported.** systemd has no condition for
them; NetworkManager exposes `Metered` only over D-Bus, and checking it
needs a program that compares the value, which would be a script.

## Endpoint interface

A future endpoint command (`deskos status`, `deskos update`,
`deskos rollback`) relies on these names and files, which do not change
within `v1alpha1`:

| Item | Name |
|---|---|
| image update service and timer | `deskos-image-update.service`, `deskos-image-update.timer` |
| Flatpak update service and timer | `deskos-flatpak-update.service`, `deskos-flatpak-update.timer` |
| unit files | `/usr/lib/systemd/system/<unit>` |
| masked platform units | `/etc/systemd/system/<unit>` linked to `/dev/null` |
| effective policy | `artifact.scheduledUpdates` and `artifact.systemdMasks` in `/usr/share/deskos/plan.json` |

## Consequences

- The public API has **exactly eleven kinds**; a test enforces the count.
- DeskOS Core ships `UpdatePolicy/deskos-core`: image and Flatpak
  updates daily, only on AC power. Core images no longer reboot on their
  own; a staged image applies at the user's next reboot.
- This is artifact content (units in `/usr`, masks in `/etc`), not a
  controller: DeskOS does not observe or enforce the result. The masks
  live in mutable `/etc`; an administrator or configuration management
  may unmask them.
- A workstation without `UpdatePolicy` image intent keeps the platform
  behavior, including the stock rebooting timer.
- With `requireACPower`, a run that finds the machine on battery is
  skipped until the next scheduled time; there is no catch-up on
  plugging in.
