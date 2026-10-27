# Validating identity the way a fleet needs it

**Status:** design note, 2026-10-27. The FreeIPA check landed in
[#236](https://github.com/deskosproject/deskos-core/pull/236) and the GDM login
in [#237](https://github.com/deskosproject/deskos-core/pull/237); the Samba AD
check is in [#238](https://github.com/deskosproject/deskos-core/pull/238).
FreeIPA run `40450471210` and AD run `40539508281` are the evidence quoted
below.

**This is a test rig, not a product capability.** DeskOS does not enrol
machines: *Enrollment is reserved in the Plan and not implemented*
([architecture.md](../architecture.md#provisioning-and-enrollment)), and keeping
running machines in a state belongs to configuration management, not to the
artifact. What follows is how the **test** proves that a DeskOS image can be
joined and used, by doing the enrolment a person or a management plane would do
otherwise. Nothing here ships in an image, and no DeskOS command joins a
machine.

Joining a Linux machine to a directory is not interesting. Any distribution can
do it once, by hand, on a machine someone is sitting in front of — and any
distribution can ship a script that does it. What a managed fleet needs is
different, and it is the part this repository validates:

- the machine joins **without a person at the console**, before the greeter
  appears;
- the credentials are **not in the artifact**, so an image cannot leak a realm;
- the result is **asserted end to end**, not assumed from an exit code;
- the proof is **repeatable and attached to a run**, not to somebody's memory;
- a machine belongs to **one realm**, and a check cannot quietly reuse another
  realm's disk.

That is the difference between a desktop distribution and a fleet artifact, and
it is what the two identity workflows exist to demonstrate.

## The shape

Two workflows, one per realm, never one for both:

| workflow | realm | cadence |
|---|---|---|
| `identity-freeipa.yml` | FreeIPA (`lab.test`) | daily 04:23 UTC, dispatch, identity-only paths |
| `identity-ad.yml` | Samba AD (`ad.lab.test`) | daily 04:47 UTC, dispatch, identity-only paths |

Neither is a pull-request gate: each builds a container, a bootc image and a
QCOW2, and boots a VM. A newer push cancels the run in flight, because these are
nightly checks and a stale twenty-minute run behind a fix is wasted time.

Each owns its workspace, image tag and disk (`/mnt/deskos-ipa-work`,
`/mnt/deskos-ad-work`). A disk that joined one realm keeps `krb5.conf`,
`sssd.conf`, the machine keytab, the default realm and its `authselect` state;
joining it to a second realm is a schema conflict, not a change of provider. The
join driver refuses it outright if `realm list` is not empty.

## Nothing secret enters the image

The image carries the client packages and nothing else. At boot the harness
passes systemd credentials through SMBIOS:

```
type=11,value=io.systemd.credential.binary:<name>=<base64>
```

Those become `tmpfiles.d` entries, extra units and the join's `EnvironmentFile`.
The admin password exists only as a root-only `join.env` written at boot from a
value generated in the job and masked in the log. The realm, the principal and
the password are **runtime inputs**; `deskosctl render` never sees them, which is
what keeps the compiler deterministic and the artifact shareable.

## Ordering is the whole trick

The join unit is `Before=gdm.service` and `WantedBy=graphical.target`: the
account has to exist when the greeter starts, because the check logs in as that
account, not as a local stand-in.

It is **not** ordered against `network-online.target`. That target can fail to
be reached, and then the join never starts, GDM waits for the join, and the
machine sits at Plymouth with nothing to time out. The wait for the link happens
inside the script (`nm-online -t 80`), bounded, and the unit carries
`TimeoutStartSec=420`.

For AD the client also has to be brought up in the right order: `sssd` starts at
boot, before the guest knows the realm, and comes up marked offline. The check
restarts it once the realm DNS is in place, pins `ad_server`, and waits for
`sssctl domain-status` to say `Online` before it looks the user up.

## What is asserted, and how

The session runs a system-wide autostart probe as whoever logged in, and a root
unit copies its results and the journal to the serial port. The checks that
matter for identity:

- `join_ok=yes`, `join_sssd=active`, and the user appears in `getent passwd`;
- `session_user` **is the directory user** — for AD, `qa@ad.lab.test`;
- the home did not exist before the login (`join_home_before=missing`) and is
  owned by the uid the directory resolved afterwards, which is
  `oddjob-mkhomedir` doing its job rather than a directory prepared in advance;
- `oddjobd` is active.

FreeIPA run `40450471210` and AD run `40539508281` report exactly that. The AD
run also shows `sssctl domain-status` online with `dc.ad.lab.test` as domain
controller and global catalog, and the `DESKOS-QA$` and `host/DESKOS-QA`
principals in the machine keytab.

When no session ever appears, a watchdog timer writes the join status, the state
of `gdm` and of the join unit, and the failed units to the serial before powering
off. It turned a silent fifteen-minute hang into the evidence that found the
`access_provider` mistake; without it, a stalled greeter and a broken join look
identical from outside.

## Why this is not a unit test

DNS, Kerberos, `sssd` discovery, PAM's session phase, the greeter and the home
policy only exist in a running machine. A unit test can tell you the Plan is
well-formed; it cannot tell you that the account resolves, that PAM admits it,
or that the home appears on login. The fixture exists for exactly the gap
between a correct artifact and a usable workstation.

It is also deliberately small: throwaway realm, one user, one machine, no
published artifact, no support claim. The container is a fixture.

## What real enrolment needs

The harness takes the shortcut a test may take: a throwaway realm, an
administrator password generated in the job, and `--force` because the fixture's
FreeIPA has no DNS. A real fleet cannot take any of those shortcuts, and none of
the following lives in Core:

- **A per-host credential, never a shared secret in the image or the ISO.** FreeIPA
  issues one with `ipa host-add --random` and the client consumes it with
  `ipa-client-install -w <otp>`; AD uses a credential from a service account or an
  offline-join blob. The provisioning system (Satellite, Foreman, Ansible, an MDM)
  delivers it to that machine, once.
- **A resolvable name and DNS.** The machine needs an FQDN before it joins, and the
  realm needs an A and a **PTR** record for it: Kerberos and `sssd` expect the
  name to resolve forward and back. With integrated DNS the client updates its own
  record after joining (`ipa-client-install --enable-dns-updates`); without it,
  somebody owns those records. `--force` exists for fixtures without DNS, not for
  fleets.
- **A place to run the join.** The installer's kickstart `%post`, a first-boot
  unit shipped by the organization, or configuration management after provisioning.
  Core compiles a baseline and never runs shell, so the hook belongs to the
  organization that owns the realm.

Getting those three right is the difference between "the image can be joined" —
what this check proves — and "the machine joined itself", which DeskOS does not
do.

## What this does not prove

- **Not Windows.** Samba AD is a real AD implementation, but Group Policy
  processing, Windows DC replication and Microsoft-specific extensions are
  untested and unclaimed.
- **Not a trust yet.** FreeIPA ↔ AD needs both realms reachable at once, with DNS
  delegation; it is the next step, not a claim.
- **Not an interactive password login.** The check autologs the directory user,
  which exercises PAM's session phase, the greeter and the home policy, but no
  password is typed. Driving QEMU input would cover it.
- **Not a production directory.** Throwaway realm, one account, no hardening.
- **Not enrolment automation.** The check joins the machine itself; how a real
  machine receives a per-host credential, a name and DNS records is the
  organization's provisioning step, not this artifact's.

## Reusing the pattern

The same shape — disposable service in a container, booted image, credentials at
runtime, serial report, watchdog — is what the next fleet properties need:
an AD trust with DNS delegation, a realm outage with cached and emergency
accounts, and signature enforcement against a local registry. None of them
changes the compiler; all of them are about what a machine can do after it boots.
