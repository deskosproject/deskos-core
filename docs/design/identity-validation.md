# Validating identity the way a fleet needs it

**Status:** design note, updated 2026-11-06. The FreeIPA check landed in
[#236](https://github.com/deskosproject/deskos-core/pull/236), the GDM login in
[#237](https://github.com/deskosproject/deskos-core/pull/237) and the Samba AD
check in [#238](https://github.com/deskosproject/deskos-core/pull/238). The
interactive login that replaced autologin landed in
[#396](https://github.com/deskosproject/deskos-core/pull/396), the realm's
own DNS in [#397](https://github.com/deskosproject/deskos-core/pull/397), and
the cross-forest trust in [#441](https://github.com/deskosproject/deskos-core/pull/441).
FreeIPA run `41631837345` and AD run `41631837362` (2026-11-03) are the client
runs quoted below; the trust has its own container workflow. The corporate
case, with screenshots, is in
[corporate-identity.md](../use-cases/corporate-identity.md).

This is a test rig, not a product capability: it proves a machine can join a
directory automatically and repeatedly. DeskOS does not enrol machines.

*Enrollment is reserved in the Plan and not implemented*
([architecture.md](../runtime.md#provisioning-and-enrollment)), and keeping
running machines in a state belongs to configuration management. Nothing here
ships in an image, and no DeskOS command joins a machine.

Any distribution can join a directory once, by hand, with a script. A fleet needs
more, and this is what the repository checks:

- the machine joins **without a person at the console**, before the greeter;
- credentials are **not in the artifact**, so an image cannot leak a realm;
- the result is **asserted end to end**, not assumed from an exit code;
- the proof is **repeatable and attached to a run**;
- a machine belongs to **one realm**, and a check cannot reuse another realm's
  disk.

## The shape

Two workflows, one per realm, never one for both:

| workflow | realm | cadence |
|---|---|---|
| `identity-freeipa.yml` | FreeIPA (`idm.example.com`) | daily 04:23 UTC, dispatch, identity-only paths |
| `identity-ad.yml` | Samba AD (`ad.example.com`) | daily 04:47 UTC, dispatch, identity-only paths |

Neither is a required pull-request check: each builds a container, a bootc image
and a QCOW2, and boots a VM, so a newer push cancels the run in flight. Both run
on pull requests whose paths touch the fixture, and on the daily schedule. Each
owns its workspace, image tag and disk (`/mnt/deskos-ipa-work`,
`/mnt/deskos-ad-work`).

A disk that joined one realm keeps `krb5.conf`, `sssd.conf`, the machine keytab,
the default realm and `authselect` state, so a second realm is a schema conflict;
the driver refuses it if `realm list` is not empty.

## Nothing secret enters the image

The image carries the client packages and nothing else. At boot the harness passes
systemd credentials through SMBIOS:

```
type=11,value=io.systemd.credential.binary:<name>=<base64>
```

Those become `tmpfiles.d` entries, extra units and the join's `EnvironmentFile`.
The admin password exists only as a root-only `join.env` written at boot, from a
value generated in the job and masked in the log. Realm, principal and password
are **runtime inputs**; `deskosctl render` never sees them, which keeps the
compiler deterministic and the artifact shareable.

## Ordering is the whole trick

The join unit is `Before=gdm.service` and `WantedBy=graphical.target`: the
account has to exist when the greeter starts, because the check logs in as that
account, not a local stand-in.

It is **not** ordered against `network-online.target`: that target can fail, the
join never starts, GDM waits and the machine sits at Plymouth. The wait for the
link happens inside the script (`nm-online -t 80`), bounded, and the unit carries
`TimeoutStartSec=420`.

For AD, `sssd` starts before the guest knows the realm and comes up offline. The
check restarts it once realm DNS is in place, pins `ad_server`, and waits for
`sssctl domain-status` to say `Online`.

## What is asserted, and how

The greeter asks for a username (the user list is disabled), a root unit says it
is up on the serial port, and the harness types the directory user and its
generated password on the hypervisor's VNC keyboard. The session then runs a
system-wide autostart probe as whoever logged in, and a root unit copies its
results and the journal to the serial port:

- the typed password reaches PAM: this is the **auth** phase, not only the
  session phase;
- `join_ok=yes`, `join_sssd=active`, and the user appears in `getent passwd`;
- `session_user` **is the directory user**, for AD `qa@ad.example.com`;
- the home did not exist before the login (`join_home_before=missing`) and is
  owned by the uid the directory resolved afterwards, through `oddjob-mkhomedir`;
- `oddjobd` is active.

FreeIPA run `41631837345` and AD run `41631837362` report exactly that, with a
password typed at the greeter. The AD run also shows `sssctl domain-status`
online with `dc1.ad.example.com` as domain controller and global catalog, and the
`DESKOS-QA$` and `host/DESKOS-QA` principals in the keytab.

A watchdog timer writes the join status, the state of `gdm` and the join unit,
and the failed units to the serial before powering off when no session appears.
That report found the `access_provider` mistake behind a silent fifteen-minute
hang.

## Why this is not a unit test

DNS, Kerberos, `sssd` discovery, PAM's session phase, the greeter and the home
policy exist only in a running machine. A unit test can tell you the Plan is
well-formed, not that the account resolves, that PAM admits it, or that the home
appears. The fixture is small: a throwaway realm, one user, one machine, no
published artifact, no support claim.

## Out of scope: how a machine is enrolled

Enrolment is the organization's business: a configuration-management run, a
provisioning template, a first-boot hook or a per-host one-time credential. Core
compiles a baseline and never runs shell, so none is shown or recommended here.

A real enrolment needs a **credential for that machine alone** (never a shared
secret in an image or ISO), a **name with forward and reverse DNS**, and a place
to run the join. The fixture uses a throwaway realm, a job-generated admin
password and `--force`; the realm serves its own DNS, so no test depends on the
runner's resolver for the zone.

## What this does not prove

- **Not Windows AD.** Joining the Samba DC runs the same Kerberos, LDAP, DNS and
  `sssd-ad` path as a Microsoft Windows AD domain for the client. It is not the
  same product (no Group Policy, no Windows replication) and is a stand-in **for
  testing only**.
- **A check, not a recommendation.** Which directory a fleet runs is the
  organization's decision. FreeIPA fits Linux better because it models Unix
  identity, host-based access and `sudo` policy natively; AD is supported because
  organizations already have it.
- **The trust is a separate fixture.** `identity-ad-trust-container.yml` runs
  two jobs. The first provisions the DC (Samba on Fedora 45) and the IPA server
  (CentOS Stream 10) on one network, with DNS in both directions and the trust,
  and fails if the user resolves before it; it also asserts the guide's own
  check, `kinit qa@ad.example.com` then `kvno -S host
  idm1.idm.example.com`, so the cross-realm ticket is a record and not a
  guess. The second repeats the fixture, boots the identity client, joins it to
  FreeIPA and logs in at GDM as `qa@ad.example.com`, the account that lives in
  AD, through the trust.
- **Not a real user's password.** The fixture generates one per run, and the
  disposable DC turns off its complexity rule so the keyboard path stays
  lowercase; it is a test credential, not a directory policy.
- **Not a production directory.** Throwaway realm, one account, no hardening.

## Reusing the pattern

The same shape (disposable service in a container, booted image, credentials at
runtime, serial report, watchdog) covers the trust and the client login. The
next fleet properties are a realm outage with cached and emergency accounts,
and signature enforcement against a local registry. None changes the compiler.
