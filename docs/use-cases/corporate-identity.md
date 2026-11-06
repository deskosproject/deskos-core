# A corporate desktop with FreeIPA and an AD trust

**Status:** use case and evidence, 2026-11-06. The trust runs in the
container fixture (`identity-ad-trust-container.yml`, PR #441); the client
login runs in `identity-ad.yml` (PR #396 and #444), with `identity-freeipa.yml`
as the FreeIPA-only control.

The organization has Active Directory and Linux workstations. The account
lives in AD; the machine is enrolled in FreeIPA; SSSD resolves the AD account
through a one-way, cross-forest trust. This is the corporate desktop case
DeskOS tests. It is not a product feature: DeskOS compiles the client image and
does not enrol machines, hold a realm or run a join.

## What the image carries, and what it does not

The client image carries `ipa-client`, `realmd`, `adcli`, `sssd-ad`,
`samba-common-tools`, `krb5-workstation` and `oddjob-mkhomedir`
(`tests/identity/resources`). It carries no realm, principal, keytab or
password. The join is a runtime act. In the fixture the harness passes the
credentials as systemd credentials and writes a root-only `join.env` at boot,
so `deskosctl render` never sees them and the image is shareable.

## The shape

```
AD DC (adtrust.test)            FreeIPA server (lab.test)
        \                          /
         \   trust, DNS, Kerberos /
          \                      /
           DeskOS client (FreeIPA-enrolled, GDM)
```

- The DC forwards `lab.test` to the IPA server; the IPA server forwards
  `adtrust.test` to the DC with a conditional zone. Both directions are
  asserted before the trust is created.
- Both directories share a podman network with pinned addresses
  (`10.89.0.2` and `10.89.0.3`). A restart used to move the DC's address and
  every check after it talked to a dead host; the static address is the fix.
- The disposable `.test` zones are unsigned, so the IPA DNS runs with
  `dnssec-validation no` in `/etc/named/ipa-options-ext.conf`, the file Red Hat
  documents in solution 1400113.
- The trust is one-way: AD trusts IPA. `ipa-adtrust-install`, `trust-add`,
  `getent passwd qa@adtrust.test` and a cross-realm `kinit` prove it. The
  negative controls ask the same IPA server before the trust and for a name
  outside it; both must fail.

## The greeter

The user list is disabled, so the greeter asks for a name. The test types the
AD user and a generated password through the hypervisor's VNC keyboard, which
is the auth phase, not only the session phase.

![The AD user at the GDM greeter, with the user list disabled](../images/identity/ad-gdm-username.png)

*The greeter asks for `user@ad.lab.test`; the frame is from the instrumented
boot check, not a mock.*

![The password page for the AD user](../images/identity/ad-gdm-password.png)

*PAM and SSSD authenticate the AD account through the trust. The password is
generated per run and never enters the image.*

## The session

After the login the probe runs as the directory user and records the session.
This frame is a FreeIPA session from the same fixture; the first-run dialog is
in front and Settings is open behind it.

![A directory-user GNOME session with the Settings app and the DeskOS dock](../images/identity/freeipa-session.png)

*The session belongs to the directory user, `oddjob-mkhomedir` created the
home, and `/var/home` is labeled `home_root_t` by the image's tmpfiles rule.*

## What is verified

- **The trust in containers**: `identity-ad-trust-container` provisions the
  DC and the IPA server, proves DNS in both directions and the trust ports,
  establishes the trust, resolves the user, obtains a Kerberos ticket and keeps
  the two negative controls. It builds no image and boots no VM.
- **The client login**: `identity-ad` and `identity-freeipa` build the client
  image, join the realm at boot, log in at GDM and assert `join_ok`,
  `join_sssd`, `session_user`, the home owner and `oddjobd`.
- **The label**: the identity check reads `home_root_label` from `/var/home`
  and requires `home_root_t`; the RHEL 10 bases create the directory unlabeled,
  which is why the image carries the tmpfiles rule (ADR 0030).
- **No secrets**: the artifact carries the client packages only; the realm,
  principal and password are runtime inputs.

## What it does not

- **Not Microsoft AD.** The Samba DC runs the same Kerberos, LDAP, DNS and
  `sssd-ad` path as a Windows domain for the client, but it has no Group
  Policy and no Windows replication. It is a stand-in for testing.
- **Not a production directory.** One account, a throwaway realm, no
  hardening, generated passwords.
- **Not enrolment.** A real machine needs a credential for that machine alone,
  forward and reverse DNS, and a place to run the join; that is the
  organization's configuration management.
- **Not every trust.** DNS delegation, firewall policy, the organization's PKI
  and the AD side's governance stay with the organization.

## Evidence

- [The identity validation design](../design/identity-validation.md) records
  the client checks, the credential path and the watchdog.
- The workflows: `identity-ad-trust-container.yml`, `identity-ad.yml` and
  `identity-freeipa.yml`. The container trust is the cheap half; the VM login
  is the expensive one.
- The desktop configuration in this image is typed: `GnomeProfile` carries the
  greeter, the session and the locks, and
  [the GNOME coverage](../gnome-coverage.md) records what is modelled.
