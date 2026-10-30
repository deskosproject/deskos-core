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
| `identity` | the account the network authenticates. Required for a password method, which NetworkManager refuses without one; a TLS profile does not need it |
| `clientCert`, `privateKey` | where the fleet places this machine's certificate and key, for EAP-TLS: `/etc/pki/tls/certs/` and `/etc/pki/tls/private/` |
| `secret` | `agent` (a session agent supplies it) or `none` (EAP-TLS needs no secret) |
| `autoconnect`, `priority` | whether the machine joins it on its own, and how it ranks |

The compiler generates the keyfile at
`/etc/NetworkManager/system-connections/<name>.nmconnection`, mode `0600`, and
`deskos check` reads its digest like any other generated file.

**No password, no private key.** The generated file sets `password-flags=1`
(agent-owned) and never writes a secret, so the machine asks for one at connect
time; the fleet supplies it through NetworkManager's agent or its own mechanism.

**Identity is a field, and a measured one.** The first version of this decision
left it out for the same reason as the password: it names a machine or a person.
Measured on NetworkManager 1.52 on EL10, that does not work: a password profile
without a non-empty identity is refused at load time (`802-1x.identity: property is
missing`), so the keyfile would never appear in `nmcli` and the check would fail
for the wrong reason. The field is therefore required for a password method and
absent for EAP-TLS, which loads without it. It is not a secret, but a shared
identity is shared by every machine of the image, so the text says so, and the
fleet that needs per-machine identities rewrites the keyfile.

That has a consequence worth stating plainly: a profile that pins the CA and the
server name but carries no identity **cannot authenticate until the fleet supplies
one**. What the image buys is the part that must be identical everywhere and that
is dangerous to get wrong, which is the trust decision: which CA, which server
name, which method, and whether the machine joins on its own.

### The first secret

"The fleet supplies it" is circular on a network that requires 802.1X to reach the
fleet, and agent-owned is worse than it sounds: NetworkManager asks a **session**
agent, and before anyone logs in there is none, so the machine is offline at boot
and at the greeter. Three paths actually work, and the image has to make room for
them instead of pretending one of them is automatic:

1. **EAP-TLS with a machine certificate.** The CA is public and travels in the
   image; the client certificate and its key are per machine, placed by
   provisioning or the fleet. There is no secret to ask for, so the machine joins
   before login. This is why `secret: none` and the two paths exist: the image says
   *where* the machine's credentials live, and the fleet puts them there. The paths
   are `/etc/pki/tls/certs/` and `/etc/pki/tls/private/`, the platform's PKI tree,
   because its files carry `cert_t` and NetworkManager may read them; a directory of
   our own would be `etc_t` and would need a policy we ship (measured on EL10 with
   `matchpathcon`).

   The key can live in the TPM instead of a file, and that is a real path but not a
   free one. `tpm2-pkcs11` keeps the key inside the TPM and exposes it as a PKCS#11
   token; NetworkManager accepts a PKCS#11 URI in `802-1x.private-key`
   (`pkcs11:...;type=private`), and `wpa_supplicant` needs
   `TPM2_PKCS11_STORE=/etc/tpm2_pkcs11`. It also needs an SELinux module that lets
   `NetworkManager_t` write the token database and open `tpm_device_t`, and a PIN
   decision: the published recipe puts `pin-value` in the URI, which is a secret in
   the keyfile, and the alternative is a PIN the fleet supplies. DeskOS does not
   model it yet; a profile names a key file, and an organization that wants the TPM
   has those three pieces to solve first.
2. **An onboarding VLAN or MAC-based admission.** The switch admits an unknown
   machine to a quarantine network, the fleet configures it there, and the port
   moves to production. DeskOS does not control the switch; what it can do is say
   that the first boot has to happen there, and the deployment page must say it.
3. **A per-machine secret written at provisioning.** A kickstart `%post` or a fleet
   run writes the password into the keyfile with `password-flags=0`, mode `0600`.
   It is fleet state, not image content, and it makes the keyfile a file the fleet
   owns: `deskos check` will report it as a difference from the declared file,
   because it is one.

Password methods stay `agent` in the image, which is right for a laptop a person
logs into and wrong for a desktop that must be on the network at boot. That is the
honest split, and the ADR does not hide it behind "the fleet supplies it".

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
