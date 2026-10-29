# Validating identity the way a fleet needs it

**Status:** design note, 2026-10-27. The FreeIPA check landed in
[#236](https://github.com/deskosproject/deskos-core/pull/236) and the GDM login
in [#237](https://github.com/deskosproject/deskos-core/pull/237); the Samba AD
check is in [#238](https://github.com/deskosproject/deskos-core/pull/238).
FreeIPA run `40450471210` and AD run `40539508281` are the evidence quoted
below.

**Purpose: prove that it can be done, automatically, over and over.** This is a
test rig, not a product capability. DeskOS does not enrol machines: *Enrollment
is reserved in the Plan and not implemented*
([architecture.md](../runtime.md#provisioning-and-enrollment)), and keeping
running machines in a state belongs to configuration management. The test does
the join itself, stands the machine up and checks the result on a schedule.
Nothing here ships in an image, and no DeskOS command joins a machine.

Any distribution can join a directory once, by hand, with a script. A fleet needs
more, and this is what the repository validates:

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
| `identity-freeipa.yml` | FreeIPA (`lab.test`) | daily 04:23 UTC, dispatch, identity-only paths |
| `identity-ad.yml` | Samba AD (`ad.lab.test`) | daily 04:47 UTC, dispatch, identity-only paths |

Neither is a pull-request gate: each builds a container, a bootc image and a
QCOW2, and boots a VM; a newer push cancels the run in flight.

Each owns its workspace, image tag and disk (`/mnt/deskos-ipa-work`,
`/mnt/deskos-ad-work`). A disk that joined one realm keeps `krb5.conf`,
`sssd.conf`, the machine keytab, the default realm and `authselect` state, so a
second realm is a schema conflict; the driver refuses it if `realm list` is not
empty.

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

The session runs a system-wide autostart probe as whoever logged in, and a root
unit copies its results and the journal to the serial port:

- `join_ok=yes`, `join_sssd=active`, and the user appears in `getent passwd`;
- `session_user` **is the directory user** — for AD, `qa@ad.lab.test`;
- the home did not exist before the login (`join_home_before=missing`) and is
  owned by the uid the directory resolved afterwards — `oddjob-mkhomedir`;
- `oddjobd` is active.

FreeIPA run `40450471210` and AD run `40539508281` report exactly that. The AD
run also shows `sssctl domain-status` online with `dc.ad.lab.test` as domain
controller and global catalog, and the `DESKOS-QA$` and `host/DESKOS-QA`
principals in the keytab.

When no session appears, a watchdog timer writes the join status, the state of
`gdm` and the join unit, and the failed units to the serial before powering off.
It turned a silent fifteen-minute hang into the evidence that found the
`access_provider` mistake.

## Why this is not a unit test

DNS, Kerberos, `sssd` discovery, PAM's session phase, the greeter and the home
policy only exist in a running machine. A unit test can tell you the Plan is
well-formed, not that the account resolves, that PAM admits it, or that the home
appears. The fixture is small: throwaway realm, one user, one machine, no
published artifact, no support claim.

## Out of scope: how a machine is enrolled

Enrolment is the organization's business — a configuration-management run, a
provisioning template, a first-boot hook, a per-host one-time credential. None is
shown or recommended here: Core compiles a baseline and never runs shell.

A real enrolment needs a **credential for that machine alone** (never a shared
secret in an image or ISO), a **name with forward and reverse DNS**, and a place
to run the join. The fixture uses a throwaway realm, a job-generated admin
password and `--force` because its FreeIPA has no DNS.

## What this does not prove

- **Not Windows AD.** Joining the Samba DC is equivalent to joining a Microsoft
  Windows AD domain for the client: the same Kerberos, LDAP, DNS and `sssd-ad`
  path runs. It is not the same product — no Group Policy, no Windows
  replication — and it is a stand-in **for testing only**.
- **A check, not a recommendation.** Which directory a fleet runs is the
  organization's decision; FreeIPA fits Linux better because it models Unix
  identity, host-based access and `sudo` policy natively. AD is supported because
  organizations already have it.
- **Not a trust yet.** FreeIPA ↔ AD needs both realms reachable at once, with DNS
  delegation; that is the next step.
- **Not an interactive password login.** The check autologs the directory user,
  exercising PAM's session phase and the home policy; no password is typed.
- **Not a production directory.** Throwaway realm, one account, no hardening.
- **Not enrolment automation.** The check proves an image can be joined and used;
  how an organization joins its machines is its own tooling.

## Reusing the pattern

The same shape — disposable service in a container, booted image, credentials at
runtime, serial report, watchdog — is what the next fleet properties need: an AD
trust with DNS delegation, a realm outage with cached and emergency accounts, and
signature enforcement against a local registry. None changes the compiler.
