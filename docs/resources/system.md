# System resources

Part of the [resource model](../resources.md): the kinds that describe boot, updates and trust.

## `BootProfile`

**Boot appearance**: graphical splash, quiet boot and the splash watermark.

Core ships two: `deskos-boot` (the boot behaviour, no artwork) and
`deskos-splash` (the DeskOS lockup), which needs `deskos-boot` for a graphical
splash to sit on.

```yaml
apiVersion: system.deskos.org/v1alpha1
kind: BootProfile
metadata:
  name: deskos-boot
spec:
  splash: graphical
  quiet: true
---
apiVersion: system.deskos.org/v1alpha1
kind: BootProfile
metadata:
  name: deskos-splash
spec:
  watermark: ../assets/deskos/deskos-splash-watermark.png
```

## `UpdatePolicy`

**Unattended updates** of the image and of system Flatpaks. By default the
image update downloads and stages the next image; it applies at the next
reboot the user makes. An organization can opt in to a scheduled apply and
reboot inside a local time window.

```yaml
apiVersion: system.deskos.org/v1alpha1
kind: UpdatePolicy
metadata:
  name: deskos-core
spec:
  image:
    automatic: true
    schedule: daily
    requireACPower: true
```

Core sets the **image** policy only. `flatpak:` is deliberately absent from it
([`resources/system/deskos-core-updates.yaml`](../../resources/system/deskos-core-updates.yaml)):
declaring it in the foundation would impose an application channel on every
platform. An organization that wants unattended Flatpak updates adds `flatpak:`
at its own layer.

`schedule` is `daily` or `weekly`. Every field is a layered scalar, so an
organization changes one field and keeps the rest of Core's policy, for
example `image: {schedule: weekly}`, or turns image updates off with
`image: {automatic: false}` when another tool updates its machines.

## Scheduled apply

`apply` is the explicit opt-in to run `bootc upgrade --apply` and reboot. It
requires `automatic: true`, and it is independent of the stage schedule:
daily staging plus a weekly apply window is the recommended shape.

```yaml
spec:
  image:
    automatic: true
    schedule: daily
    requireACPower: true
    apply:
      enabled: true
      days: [sunday]
      at: "03:00"                  # exact local time, or:
      # window:
      #   start: "03:00"
      #   duration: 2h
      timezone: Europe/Madrid      # optional; machine local zone when omitted
```

- `days` is a non-empty list of lowercase weekdays.
- Exactly one of `at` or `window` is required.
- `at` is `HH:MM`; the apply may start within five minutes after it.
- `window.start` is `HH:MM`, `window.duration` is a positive whole-minute
  duration, and the window must end on the same day.
- `timezone` is an IANA name validated with the embedded tzdata; omit it for
  the machine's local zone.
- The apply timer is `Persistent=false`; a missed window waits for the next
  one instead of rebooting at the next boot. The generated
  `deskos window` `ExecCondition` also rejects a timer that fires after
  suspend.
- `apply.enabled: false` disables a lower layer's apply policy.

An organization that only wants staging never sets `apply`. The generated units
are described in [architecture.md](../architecture.md#updates) and
[ADR 0023](../adr/0023-scheduled-apply-window.md); how to operate them is in
[operations.md](../operations.md).

## `TrustAnchor`

**CA certificates the image trusts or refuses to trust** in the platform
trust store:

```yaml
apiVersion: system.deskos.org/v1alpha1
kind: TrustAnchor
metadata:
  name: example-root-ca
spec:
  anchors:
    - name: example-root-ca
      file: ../assets/example-root-ca.crt
```

Each `file` is a PEM X.509 certificate inside the resource root and `name`
is the file stem in the trust store. Anchors are **keyed** by `name`: the
same certificate under one name deduplicates, and a different certificate
under the same name conflicts at any layer. The Platform supplies the
anchors directory and the update command; on the EL platforms that is
`/etc/pki/ca-trust/source/anchors/` and `update-ca-trust`. The private key
is neither needed nor accepted. See
[architecture.md](../architecture.md#trust-store).

A certificate can also be **distrusted** instead of trusted:

```yaml
apiVersion: system.deskos.org/v1alpha1
kind: TrustAnchor
metadata:
  name: my-workstation-distrust
spec:
  blocklist:
    - name: compromised-root-ca
      file: ../assets/compromised-root-ca.pem
```

`blocklist` entries have the same `name`/`file` shape, keying and asset
rules as `anchors`, and one name cannot be in both. They lower to the
platform's `trust.blocklistDir` (on the EL platforms
`/etc/pki/ca-trust/source/blocklist/`), which p11-kit reads as distrusted:
`update-ca-trust` leaves the certificate out of every extracted bundle, so
a chain that ends at it is rejected. Trust in that CA is removed; TLS is
not weakened. A platform that declares anchors but no `blocklistDir`
fails composition with that gap instead of ignoring the entry.

The rescue path for a compromised CA is a resource, not a script, and it
composes at any layer including the workstation layer. Core ships
`deskos-trust` with the October 2026 eMudhra roots (RHSB-2026-012); an
organization adds its own through its profiles. Drop an entry once the
platform's `ca-certificates` no longer ships that root.
