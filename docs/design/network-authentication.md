# How a workstation authenticates to the network, and who supplies what

Two authentications are easy to conflate: the **network** (802.1X, before and
independently of any login) and the **session** (PAM and SSSD, after a person logs
in). They can use the same account and they are still separate decisions, decided
by different servers. This page says what each method needs, how Active Directory
and FreeIPA fit, and which part the image can carry.

## The three methods that matter

| Method | The client proves | The RADIUS server needs | Client certificate |
|---|---|---|---|
| EAP-TLS | a machine or user certificate | the issuing CA | yes |
| PEAP with MSCHAPv2 | a user name and password, inside a TLS tunnel | the NT hash of that password, or a domain controller that can prove the challenge | no |
| EAP-TTLS with PAP | a user name and password, inside a TLS tunnel | an LDAP bind as that user | no |

The first is what a machine uses **before a login**; the other two need a person's
password, so they work after one, or with a secret the fleet wrote down.

## Active Directory

- **The workstation does not need to be joined** for any of the three. 802.1X is
  the supplicant sending credentials to a RADIUS server; nothing in the protocol
  asks whether the machine has a computer account.
- **PEAP does need a domain controller behind the RADIUS server.** Active Directory
  never exposes the NT hash (`unicodePwd` is write-only), so the challenge has to be
  proved by a DC. The node that needs the machine account is the **RADIUS server**
  (Samba `winbind`/`ntlm_auth`, or a Windows NPS), not the workstation.
- **EAP-TTLS with PAP** binds to AD's LDAP as the user, so it needs no NT hash and no
  join anywhere. It is the password method for a directory that cannot prove
  MSCHAPv2.
- **EAP-TLS** needs a certificate issued to the machine (or the user) by the
  organization's PKI. Auto-enrollment usually rides on AD, but the certificate works
  on a machine that never joined.

## FreeIPA, and its trust with AD

- **FreeIPA publishes `ipaNTHash`**, the same MD4 the MSCHAPv2 challenge needs, as an
  ordinary attribute. With an ACI that lets the RADIUS bind account read it,
  FreeRADIUS verifies PEAP **without any domain join**, on the RADIUS node or the
  workstation. The hash is read per login and stored nowhere.
- **A trust with AD does not change that for AD users.** The trust lets AD users
  authenticate to IPA services with Kerberos; it does not copy their NT hashes into
  IPA, so PEAP for an AD account still needs the AD DC to prove the response. For AD
  users, EAP-TTLS/PAP against AD's LDAP, or EAP-TLS, avoids the question.
- **For the session**, an AD trust is what makes "indirect login" work: the
  workstation is enrolled in FreeIPA, not joined to AD, and an AD user logs in
  through the trust with SSSD. That is a login decision and says nothing about how
  the machine got on the network.

## What the image carries, and what the fleet supplies

`NetworkProfile` carries the policy: the EAP method, the CA, the server name, the
inner method, and an identity. It never carries a password
([ADR 0027](../adr/0027-a-network-profile-is-typed-and-its-secrets-are-not.md)).

| Thing | Image | Fleet or provisioning | Why |
|---|---|---|---|
| EAP method, CA, server name, `phase2` | yes | | identical on every machine, and the part that is dangerous to get wrong |
| Shared identity (`identity`) | yes | | not a secret, but shared by every machine that carries the image |
| Password | never | written per machine, or asked at login | a secret in a repository is a published secret |
| Per-machine identity | no | provisioning, or the person at login | it names a machine or a person |
| Machine certificate and key | no | enrolled by the fleet or the PKI | it names the machine |
| Domain or realm enrolment | no | provisioning | it names the machine, and it is not image content |

Three ways the secret or the identity reaches a machine, in the order this
repository prefers them:

1. **EAP-TLS with a machine certificate**, enrolled at provisioning: nothing to ask
   for, works before login. This is the only path that needs no secret on the
   machine.
2. **A per-machine secret written at provisioning** into the keyfile with
   `password-flags=0`, mode `0600`. It makes the keyfile a fleet file, and
   `deskos check` reports it as the difference it is.
3. **A person types it after login**, with `password-flags=1`. It works, and it is
   the only option when nobody is going to provision anything, but the machine is
   offline at the greeter and before the first login.

## What DeskOS does not do

- It does not join a domain or a realm: enrolment is per-machine identity, and the
  research item for it is [P2-06](../future-backlog.md).
- It does not carry a password, a private key or a per-machine identity.
- It does not claim that a profile authenticates. The session check reads whether
  NetworkManager loaded it (`nmcli`), and the lab with a RADIUS server is what would
  prove a login.
