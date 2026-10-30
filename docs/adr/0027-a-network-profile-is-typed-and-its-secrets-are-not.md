# ADR 0027: a network profile is a typed resource, and its secrets are not

**Status:** accepted (2026-10-30)

## Context

An organization that adopts a desktop asks about the corporate network before it
asks about anything else. Wired and Wi-Fi networks that require 802.1X are the
norm in government, banking and large enterprise, and a machine that cannot join
one is not deployable, however good its desktop is.

DeskOS carries the CA side already: a `TrustAnchor` places a certificate in the
platform's trust store. It carries nothing for the connection itself, so today the
profile is typed by hand with `nmcli`, or pushed later by the fleet, and the image
cannot promise the machine reaches the network it was built for.

The shape of the problem is in NetworkManager's own files. A connection profile is
a keyfile in `/etc/NetworkManager/system-connections/`, owned by root and mode
`0600`, with a `[connection]` block and one block per setting (`[802-1x]`,
`[wifi]`, `[vpn]`, ...). Every secret property has a companion `-flags` key:

| Flag | Meaning |
|---|---|
| `0` | NetworkManager owns and stores the secret |
| `1` | agent-owned: a user-session agent supplies it when needed |
| `2` | not saved; asked each time |
| `4` | not required |

And 802.1X is a small, closed set of properties: `eap` (`leap`, `md5`, `tls`,
`peap`, `ttls`, `sim`, `fast`, `pwd`), `identity`, `anonymous-identity`,
`ca-cert`, `client-cert`, `private-key`, `domain-suffix-match`,
`system-ca-certs`, plus the secret and its flags. NetworkManager's own manual
ships a WPA-EAP example that sets `autoconnect=false` and asks for the secret.

## Decision

**A fifteenth kind, `system.deskos.org/v1alpha1 NetworkProfile`, with a typed
subset of an 802.1X profile and no secret in it.**

| Field | Meaning |
|---|---|
| `name`, `ssid` | the connection id, and the network for a Wi-Fi profile |
| `type` | `ethernet` or `wifi`, the two the first slice covers |
| `eap` | one of NetworkManager's methods |
| `caCert` | an asset in the resource root, pinned by digest, placed for NetworkManager to read |
| `serverName` | `domain-suffix-match`: the certificate name the server must present |
| `autoconnect`, `priority` | whether the machine joins it on its own, and how it ranks |

The compiler generates the keyfile at
`/etc/NetworkManager/system-connections/<name>.nmconnection`, mode `0600`, and
`deskos check` reads its digest like any other generated file.

**No password, no private key, no identity.** The generated file sets
`password-flags=1` (agent-owned) and never writes a secret, so the machine asks for
one at connect time; the fleet supplies it through NetworkManager's agent or its
own mechanism. Identity is deliberately absent for the same reason: it names a
machine or a person, and a value shared by every image of a fleet is a value that
should not be in the image (architecture.md, "What each layer owns").

That has a consequence worth stating plainly: a profile that pins the CA and the
server name but carries no identity **cannot authenticate until the fleet supplies
one**. What the image buys is the part that must be identical everywhere and that
is dangerous to get wrong, which is the trust decision: which CA, which server
name, which method, and whether the machine joins on its own.

## Alternatives rejected

- **A raw keyfile as a `File` or a `BinaryArtifact`.** It would carry any key and
  any secret, and the schema would be a text field: the escape hatch invariant 5
  refuses, with the added risk that a password in a public repository is a
  password leaked.
- **The fleet only, through Ansible.** Then the image cannot say what network it
  is for, the first boot of a freshly installed machine has no profile, and the
  trust decision lives in a playbook nobody reviews with the image.
- **A `GnomeProfile` block.** NetworkManager is not dconf; its settings are files
  owned by root, read before any session exists.

## Consequences

- The kind count test goes from fourteen to fifteen. That is the cost, and the
  reason is a deployment path that does not exist otherwise.
- Secrets stay out of resources by construction, not by review: there is no field
  to put one in.
- VPN is a second slice, and the same shape: `service-type` plus the plugin's own
  fields, with the plugin's `vpn.data` refused as a free-form map. OpenVPN and
  WireGuard differ enough that they are separate decisions.
- The session check gains a read that the profile exists and is well formed
  (`nmcli connection show <name>`), not that it authenticated: authentication
  needs a network that accepts the machine.
- Until the fleet supplies an identity, the profile is inert. An organization that
  wants a shared service account in the image has to ask for a field that carries
  one, and the ADR that adds it has to say why the value is the same everywhere.

## Open questions

1. **Identity**: leave it to the fleet (this ADR), or allow a machine-account
   identity that the fleet overrides? The first is honest, the second is
   convenient, and only one of them keeps the image free of identity.
2. **EAP-TLS**: a client certificate and its private key are per machine. The
   profile can name paths; whether the image may place the certificate (public)
   and refuse the key is the same question as identity.
3. **Scope of the first slice**: ethernet and Wi-Fi 802.1X only, or also a plain
   Wi-Fi PSK profile, which is a secret by definition and would have to be
   agent-owned too.
