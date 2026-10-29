# ADR 0023: Scheduled image apply is an explicit, time-bound opt-in

**Status:** accepted

## Context

ADR 0007 made DeskOS stage image updates and never reboot on its own, which is the right default for a workstation:
a published image must not interrupt a user's session. Some organizations run unattended kiosks or lab stations where
a reboot is part of the maintenance contract, and a staged image must not wait indefinitely. `bootc upgrade --apply`
applies the update and reboots, so it cannot be a safe default: it must be an explicit organization decision, limited
to a local time or a window, and a missed calendar event after suspend must not become a surprise reboot.

## Decision

Extend `UpdatePolicy.image` with an optional `apply` object:

```yaml
image:
  automatic: true
  schedule: daily
  requireACPower: true
  apply:
    enabled: true
    days: [sunday]
    at: "03:00"                 # exact time
    timezone: Europe/Madrid     # optional; machine local time when omitted
```

The window form replaces `at` with a `window` of `start` (`HH:MM`) and `duration`. `apply.enabled: true` requires
`image.automatic: true`; the staging service remains, and a separate `deskos-image-apply.service` and `.timer` are
added. Rules:
- `days` is a non-empty list of lowercase weekdays, and exactly one of `at` or `window` is required;
  `window.duration` is a positive whole-minute duration that ends on the same day.
- `timezone` is an optional IANA name, validated with Go's embedded `time/tzdata`, never the build host's zoneinfo;
  the generated `OnCalendar=` carries it, and omitting it means the machine's local zone.
- `apply.enabled: false` disables a lower layer's apply policy, and other apply fields below it are reported as
  masked. The apply timer is `Persistent=false`, so a machine powered off during the window waits for the next one
  instead of rebooting at the next boot.
- The generated service runs `deskos window` as an `ExecCondition` before `bootc upgrade --apply --quiet`; it exits 1
  outside the permitted interval, so a calendar event that elapses during suspend cannot reboot a user who wakes the
  machine later. The exact mode allows five minutes of grace, the window mode checks the full window, and
  `AccuracySec=1min` is written for both.

## Consequences

- ADR 0007's "no generated unit reboots" remains true for every image without an explicit `apply` block; the test
  that rejects `--apply` in rendered files must keep covering the Core and example images. Core does not set `apply`,
  which stays a foundation default of stage-only.
- The service does not silently kill an active session: `skipIfActiveSession` is deliberately not part of this
  decision, and the window is the contract. Notification is a separate decision, and this adds no public kind; the
  count remains fourteen.

## Endpoint interface

| Item | Name |
|---|---|
| apply service and timer | `deskos-image-apply.service`, `deskos-image-apply.timer` |
| time guard | `/usr/bin/deskos window --days … --at … --grace …` or `--start … --duration …` |
| effective plan | `artifact.scheduledUpdates` with kind `image-apply` |
