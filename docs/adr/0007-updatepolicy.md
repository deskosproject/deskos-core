# ADR 0007: UpdatePolicy as the eleventh public kind

**Status:** accepted (2026-10-04)

## Context

Both supported bases enable `bootc-fetch-apply-updates.timer` from an unowned symlink in
`/usr/lib/systemd/system/default.target.wants/`. Its service runs `bootc upgrade --apply --quiet`, and `--apply`
"currently always reboots the system" (checked on `centos-bootc:stream10`, bootc 1.16.13, and `rhel-bootc:10.2`, bootc
1.16.4), one hour after boot and every 8 hours with a 2-hour random delay. `systemctl disable` does not remove a
`/usr/lib` wants symlink; only a mask stops it. None of the ten kinds can hold the policy: it is not GNOME, boot or
platform configuration, and `Workstation` has no layering.

## Decision

Add `system.deskos.org/v1alpha1` `UpdatePolicy` with layered scalars: `image.automatic`, `image.requireACPower`,
`flatpak.automatic` and `flatpak.requireACPower` (booleans), and `image.schedule` and `flatpak.schedule` (`daily` or
`weekly`).

The highest layer wins, and different values at one layer conflict. An effective `automatic: true` needs a `schedule`.
A `schedule` or `requireACPower` at or above the layer that sets `automatic: false`, or with no `automatic`, is an
error, and below it a masked-setting warning.

- Any effective `image.automatic` masks the platform's `updates.imageUpdateUnits` (`bootc-fetch-apply-updates.timer`
  and `.service`), so DeskOS owns image updates.
- The **service** is written whenever an area has intent, even with `automatic: false` (`deskos-image-update.service`,
  `deskos-flatpak-update.service`); only the **timer** follows `automatic`, and the Flatpak package ships either way.
- `image.automatic: true` adds `deskos-image-update.timer`, running `/usr/bin/bootc upgrade --quiet`;
  `flatpak.automatic: true` adds the platform Flatpak package and `deskos-flatpak-update.timer`, running
  `/usr/bin/flatpak update --system --noninteractive --assumeyes`.
- Timers use `OnCalendar=daily` or `weekly`, `RandomizedDelaySec=2h` (the stock bootc spread) and `Persistent=true`,
  enabled through `timers.target`; `requireACPower` sets `ConditionACPower=true`.
- Commands are fixed by the backend from the IR kind: no field accepts a command, calendar expression or unit name,
  and `--apply`, `--soft-reboot` and `--download-only` are never generated.
- **Metered networks are not supported.** systemd has no condition for them, and NetworkManager exposes `Metered` only
  over D-Bus, which a script would have to compare.

## Endpoint interface

`deskos report`, `deskos check` and `deskos update` (ADR 0018) rely on these names within `v1alpha1`:

- `deskos-image-update.service` and `.timer`; `deskos-flatpak-update.service` and `.timer`; units in
  `/usr/lib/systemd/system/<unit>`; masked platform units in `/etc/systemd/system/<unit>` linked to `/dev/null`;
  `artifact.scheduledUpdates` and `artifact.systemdMasks` in `/usr/share/deskos/plan.json`.

## Consequences

- The public API has **fifteen kinds** today (eleven when this decision was taken); a test enforces the count.
- DeskOS Core ships `UpdatePolicy/deskos-core`: image updates daily, only on AC power, so Core images no longer reboot
  on their own. Since 2026-10-17 the foundation sets no `flatpak` policy; system Flatpak updates are the EL family's
  `rhel-family` policy.
- This is artifact content, not a controller: DeskOS does not observe or enforce the result, and `/etc` may be
  unmasked by an administrator or configuration management.
- A workstation without `UpdatePolicy` image intent keeps the platform behavior, including the stock rebooting timer;
  with `requireACPower`, a run on battery is skipped until the next scheduled time.
