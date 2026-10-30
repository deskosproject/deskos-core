# System resources

Part of the [resource model](../resources.md): the kinds that describe boot, updates and trust.

## `BootProfile`

Boot appearance: graphical splash, quiet boot and the splash watermark.

| Field | Type | Required | Notes |
|---|---|---|---|
| `splash`, `quiet` | `graphical` or `text`; boolean | no | `text` adds no splash argument and no initramfs rebuild; `quiet: false` adds no `quiet` |
| `watermark` | asset inside the resource root | no | a PNG, at most 1024x1024; needs an effective `splash: graphical` |

Core ships `deskos-boot` (boot behaviour, no artwork) and `deskos-splash` (the
DeskOS lockup), which needs `deskos-boot` for a splash to sit on.

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

**Unattended updates** of the image and of system Flatpaks. By default the image
update stages the next image and applies at the next reboot
([operations.md](../operations.md#how-updates-work)).

| Field | Type | Required | Notes |
|---|---|---|---|
| `image.automatic`, `.schedule`, `.requireACPower`, `flatpak.automatic`, `.schedule`, `.requireACPower` | boolean; `daily` or `weekly`; boolean | no | image updates; system Flatpaks |

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

`requireACPower` also installs a udev rule that starts the staging service when AC
comes online; the daily timer remains the schedule. Core sets the **image** policy
only: `flatpak:` is deliberately absent
([`resources/system/deskos-core-updates.yaml`](../../resources/system/deskos-core-updates.yaml)),
because the foundation must not impose an application channel on every platform.
Units and operation: [runtime.md](../runtime.md#updates),
[operations.md](../operations.md).

## Scheduled apply

`apply` is the explicit opt-in to run `bootc upgrade --apply` and reboot. It
requires `automatic: true`; daily staging plus a weekly apply window is the
recommended shape.

| Field | Type | Required | Notes |
|---|---|---|---|
| `image.apply.enabled`, `.days` | boolean; non-empty list of lowercase weekdays | `enabled` yes within `apply` | `false` disables a lower layer's apply policy |
| `image.apply.at` / `.window.start`, `.duration`, `.timezone` | `HH:MM`; `HH:MM`; positive whole-minute duration (`2h`); IANA name validated with the embedded tzdata | `at` or `window`, not both; `timezone` no; the window ends the same day | the apply may start within five minutes after `at`; machine local zone when `timezone` is omitted |

```yaml
spec:
  image:
    apply:
      enabled: true
      days: [sunday]
      at: "03:00"                  # exact local time
      timezone: Europe/Madrid      # optional; machine local zone when omitted
```

The apply timer is `Persistent=false`, and the `deskos window` `ExecCondition`
rejects a timer that fires after suspend: a missed window waits for the next one
([operations.md](../operations.md#how-updates-work)). An organization that only
wants staging never sets `apply`
([ADR 0023](../adr/0023-scheduled-apply-window.md)).

## `TrustAnchor`

**CA certificates the image trusts or refuses to trust** in the platform trust
store; a platform with anchors but no `blocklistDir` fails composition.

| Field | Type | Required | Notes |
|---|---|---|---|
| `anchors[].name`, `.file`, `blocklist[].name`, `.file` | file stem; PEM X.509 certificate inside the resource root | yes | keyed by `name`; one name cannot be in both |

```yaml
spec:
  anchors:
    - name: example-root-ca
      file: ../assets/example-root-ca.crt
```

Anchors are keyed by `name`: one certificate deduplicates, a different one under
the same name conflicts at any layer. The Platform supplies the directory and the
update command: `/etc/pki/ca-trust/source/anchors/` and `update-ca-trust` on the EL
platforms. The private key is neither needed nor accepted. A `blocklist` entry such
as `{name: compromised-root-ca, file: ../assets/compromised-root-ca.pem}` lowers to
`trust.blocklistDir` (`/etc/pki/ca-trust/source/blocklist/` on EL), which p11-kit
reads as distrusted, so a chain ending at it is rejected
([runtime.md](../runtime.md#trust-store)).

The rescue path for a compromised CA is a resource, not a script, and composes at
any layer. Core ships `deskos-trust` with the October 2026 eMudhra roots
(RHSB-2026-012); an organization adds its own through its profiles and drops an
entry once the platform's `ca-certificates` no longer ships that root.

## `NetworkProfile`

An 802.1X connection: the trust decision and the policy, with no secret in it
([ADR 0027](../adr/0027-a-network-profile-is-typed-and-its-secrets-are-not.md)).

| Field | Type | Required | Notes |
|---|---|---|---|
| `type` | `ethernet` or `wifi` | yes | the device the profile applies to |
| `ssid` | one line, 1 to 32 characters | wifi only | refused on ethernet |
| `eap` | `leap`, `md5`, `tls`, `peap`, `ttls`, `sim`, `fast`, `pwd` | yes | NetworkManager's methods |
| `caCert` | PEM certificate inside the resource root | no | omitted, the system trust store is used |
| `clientCert`, `privateKey` | paths under `/etc/deskos/network/` | EAP-TLS | where the fleet places this machine's certificate and key |
| `secret` | `agent` or `none` | no | `agent` asks a session agent after login; `none` is EAP-TLS, which needs nothing |
| `serverName` | host name | no | the name the server's certificate must present |
| `autoconnect`, `priority` | boolean; -999 to 999 | no | default `true` and `0` |

```yaml
spec:
  type: wifi
  ssid: CorpWLAN
  eap: peap
  caCert: ../assets/corp-root-ca.pem
  serverName: radius.example.com
```

The compiler writes `/etc/NetworkManager/system-connections/<name>.nmconnection`,
mode `0600`, with no password and no identity: a secret in a resource is a secret
published, and an identity names a machine or a person the image would share
across the fleet.

**A password method is `secret: agent`**, so NetworkManager asks a session agent
after a login. It does not connect at boot or at the greeter, because before a
login there is no agent.

**EAP-TLS is `secret: none`**: the CA travels in the image, and the machine's
certificate and key are paths the fleet fills under `/etc/pki/tls/certs/` and
`/etc/pki/tls/private/`, so the machine joins before anyone logs in. The image says
where they live; it never carries them. Those directories are the platform's PKI
tree and their files carry `cert_t`, which NetworkManager may read; a directory of
our own would be `etc_t`.

The key can live in the TPM rather than a file: `tpm2-pkcs11` holds it and
NetworkManager takes a PKCS#11 URI, but that needs `TPM2_PKCS11_STORE` for
`wpa_supplicant`, an SELinux module for NetworkManager's TPM access, and a PIN
decision. The profile models the file case; the TPM case is a later decision.

A machine whose network requires 802.1X before the fleet can reach it has three
ways in, and the image cannot choose for it: EAP-TLS with a machine certificate, an
onboarding VLAN or MAC-based admission where the fleet configures it, or a
per-machine secret written at provisioning with `password-flags=0`, which makes the
keyfile a file the fleet owns and `deskos check` reports it as a difference
([ADR 0027](../adr/0027-a-network-profile-is-typed-and-its-secrets-are-not.md)).
