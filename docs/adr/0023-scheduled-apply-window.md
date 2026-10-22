# ADR 0023: Scheduled image apply is an explicit, time-bound opt-in

**Status:** accepted

## Context

ADR 0007 made DeskOS stage image updates and never reboot on its own. That is
the right default for a workstation: a published image must not interrupt a
user's session. Some organizations, however, run unattended kiosks or lab
stations where a reboot is part of the maintenance contract and a staged image
must not wait indefinitely.

`bootc upgrade --apply` currently applies the update and reboots. It is not a
safe default. It must be an explicit organization decision, limited to a local
time or a window, and it must not turn a missed calendar event after suspend
into a surprise reboot.

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

or:

```yaml
  apply:
    enabled: true
    days: [sunday, wednesday]
    window:
      start: "03:00"
      duration: 2h
    timezone: Europe/Madrid
```

Rules:

- `apply.enabled: true` requires `image.automatic: true`; the staging service
  remains, and a separate `deskos-image-apply.service` and `.timer` are added.
- `days` is a non-empty list of lowercase weekdays.
- Exactly one of `at` or `window` is required.
- `at` is `HH:MM`; `window.start` is `HH:MM` and `window.duration` is a positive
  whole-minute duration that ends on the same day.
- `timezone` is an optional IANA name. It is validated with Go's embedded
  `time/tzdata`, not the build host's zoneinfo. The generated `OnCalendar=`
  carries the same zone; omitting it means the machine's local zone.
- `apply.enabled: false` disables a lower layer's apply policy; other apply
  fields below that layer are reported as masked, like other layered settings.
- The apply timer is `Persistent=false`. A machine that was powered off during
  the window waits for the next one instead of rebooting at the next boot.
- The generated service runs `deskos window` as an `ExecCondition` before
  `bootc upgrade --apply --quiet`. It exits 1 when the current local day/time is
  outside the permitted interval, so a calendar event that elapses during
  suspend cannot reboot a user who wakes the machine later.
- The exact mode allows five minutes of grace; the window mode checks the full
  window. `AccuracySec=1min` is written for both.

## Consequences

- ADR 0007's "no generated unit reboots" remains true for every image without
  an explicit `apply` block. The test that rejects `--apply` in rendered files
  must continue to cover the Core and example images.
- Core does not set `apply`; it stays a foundation default of stage-only.
- The apply decision is separate from the stage schedule: daily staging plus a
  weekly apply window is the recommended shape.
- The service does not silently kill an active session. `skipIfActiveSession`
  is deliberately not part of this decision; an organization that enables
  scheduled apply accepts that the window is the contract.
- The notification/reminder is a separate decision. It is not part of the
  apply unit and does not translate strings.
- This does not add a public kind. The count remains fourteen.

## Endpoint interface

| Item | Name |
|---|---|
| apply service and timer | `deskos-image-apply.service`, `deskos-image-apply.timer` |
| time guard | `/usr/bin/deskos window --days … --at … --grace …` or `--start … --duration …` |
| effective plan | `artifact.scheduledUpdates` with kind `image-apply` |
