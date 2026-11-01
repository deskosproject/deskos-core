# Example: an identity-managed workstation

An **organization** layer that carries the image-level half of joining a corporate
identity domain: the domain's root CA, the Wi-Fi profile used before login, and the
client packages a Kerberos-backed domain needs.

```bash
deskosctl plan ./examples/identity \
  --workstation example-identity
```

| Resource | Contents |
|---|---|
| `example-identity` (organization) | the CA, the network profile and the client packages |
| `example-identity-ca` (TrustAnchor) | the domain root CA in the platform trust store |
| `example-identity-network` (NetworkProfile) | the corporate Wi-Fi, 802.1X/PEAP |
| `example-identity` (PackageSet) | `oddjob`, `oddjob-mkhomedir`, `krb5-workstation` |

## What it shows

- **Trust is image content.** The root CA lands in the platform's anchors
  directory and one `update-ca-trust` makes every TLS client trust it.
- **The network the machine needs before a login** is a declared profile, not a
  post-install script. It is a Wi-Fi profile because an ethernet profile with
  autoconnect off can leave a machine with no route at all (measured).
- **The client packages are named; the domain is not.** Nothing in the image
  knows the domain name, the realm or a user's credentials.

## What it does not show

Enrollment is a runtime action, and the model deliberately does not express it:

- **`realm join`** needs a domain account and a secret; the image carries the
  client, not the join.
- **SSSD's domain configuration** is written by the enrollment, not by a resource
  kind. A static `sssd.conf` would embed a realm and a server list the image does
  not own.
- **Certificate-based Wi-Fi credentials** are a secret per machine. The profile
  names the outer identity and the server; the credential comes from the fleet.
- **Single sign-on for browsers and desktop apps** beyond the system trust store
  is an application configuration, not a workstation default.

The example is what the image can promise: the trust decision and the client
travel, and the enrollment stays a deliberate, auditable action by the
organization that owns the domain.
