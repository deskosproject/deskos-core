# Identity client fixtures

A test fixture, never a support claim. `identity-freeipa.yml` and
`identity-ad.yml` run daily and on demand, join a booted VM to a disposable
realm, and log in at GDM as the directory user.

## What this proves

The identity client is an **organization profile**, not Core:
`tests/identity/resources/` adds `ipa-client` and `oddjob-mkhomedir` to a CentOS
Stream 10 workstation that is otherwise Core. The image carries **no realm,
principal or password**: enrolment is a runtime act, never an artifact input
([runtime.md](../../docs/runtime.md#provisioning-and-enrollment)). The
directories here are throwaway; never point them at a real organization.

## Build the image and disk

```bash
deskosctl render tests/identity/resources \
  --workstation identity-client-centos-stream-10 --output dist/identity-client
sudo podman build -t localhost/identity-client:test dist/identity-client
```

Then build a QCOW2 as [tests/vm/README.md](../vm/README.md) describes; the
identity workstation is not published.

## FreeIPA

Pin the server image you tested; do not float a tag. It runs systemd and
supports neither privileged mode nor host networking, so the script uses the
bridge network, publishes the realm ports (80, 88, 389, 443, 464, 636) and
passes `-U`, because `ipa-server-install` prompts by default.

```bash
export FREEIPA_IMAGE='freeipa/freeipa-server@sha256:…'
export FREEIPA_PASSWORD='a throwaway Directory Manager password'
export IPA_SERVER_HOSTNAME=ipa.lab.test
export IPA_SERVER_DOMAIN=lab.test
export IPA_SERVER_REALM=LAB.TEST
sudo -E tests/identity/freeipa.sh start

export FREEIPA_USER=qa
export FREEIPA_USER_PASSWORD='a throwaway user password'
sudo -E tests/identity/freeipa.sh user
```

The domain needs at least two labels (`ipa-server-install` rejects `test`), so
the fixture uses `lab.test`, which cannot collide with a real realm.

**Join the guest.** Boot the disk with QEMU user networking (host `10.0.2.2`),
log in as the local installer, copy [join.sh](join.sh) and run it as root:

```bash
sudo IPA_DOMAIN=lab.test IPA_REALM=LAB.TEST IPA_SERVER=ipa.lab.test \
  IPA_PRINCIPAL=admin IPA_PASSWORD='…' ./join.sh
```

The script pins the server to `10.0.2.2` in `/etc/hosts`, then runs
`ipa-client-install --force --mkhomedir --no-ntp --force-join`. The realm has
**no integrated DNS** (port 53 belongs to the host resolver), so `--force`
skips the name checks.

**Log in as the directory user** at GDM and record:

```bash
id
getent passwd qa
sssctl user-checks qa
systemctl status sssd oddjobd
journalctl -b -u sssd --no-pager
```

**Records to return:** the server image digest and `ipa --version`; the realm
and domain (throwaway values); the client and disk digests; the join output,
`id`, `sssctl user-checks`, the SSSD journal and the login screenshot; the
model, BIOS and kernel if not a VM.

## Samba AD DC

`tests/identity/samba/` builds a real AD implementation with Kerberos, LDAP, DNS
and `samba-tool`, so the client path (`realm`, `adcli`, `sssd-ad`) runs without
Windows. It is **equivalent to a Windows AD domain**: the same code path. It is
**not the same product**: no Group Policy, no replication, no support statement.

```bash
sudo podman build -t localhost/deskos-ad:test tests/identity/samba
export AD_IMAGE=localhost/deskos-ad:test ADMIN_PASSWORD='a throwaway Administrator password'
sudo -E tests/identity/samba-ad.sh start
sudo tests/identity/samba-ad.sh status
export AD_USER=qa AD_USER_PASSWORD='a throwaway user password'
sudo -E tests/identity/samba-ad.sh user
```

Two dependencies: **run as root** (provisioning needs real ownership and
`security.*` xattrs; rootless panics in `set_nt_acl`), and **the client must use
the DC's DNS before it joins**, which `identity-ad.yml` does through `JOIN_DNS`.

The domain uses `--use-rfc2307`, so `uidNumber`, `gidNumber`,
`unixHomeDirectory` and `loginShell` live in the directory. Verified 2026-10-27:
the daily check joins the VM, GDM logs in as `qa@ad.lab.test`, and `oddjobd` is
active.

Four things easy to miss:
- **The realm's DNS is the source of truth** for the DC's address; pointing the
  DC elsewhere breaks the join with `Insufficient permissions`.
- **`access_provider = ad` with no rule denies every account**, and `realm permit
  --all` exits zero without writing one, so the fixture sets `permit`.
- **sssd starts before the join points the guest at the realm DNS**: the check
  restarts it and waits for the domain.
- **A serving DC is not an authenticating one.** Samba answers the domain level
  and its own SRV while it is still starting; a join that fails there asks for the
  password a second time with stdin already drained, so the unit waits on a prompt
  and reports nothing (#312). The fixture now requires two consecutive `status`
  answers ten seconds apart, and the guest waits for a `kinit` that succeeds before
  joining, with a deadline on the wait and on the join.

**One machine, one realm.** A disk that joined FreeIPA keeps `krb5.conf`,
`sssd.conf`, the keytab, the default realm and the `authselect` state; joining
it to AD is a schema conflict. The checks never share a VM, disk, workspace or
tag: `identity-freeipa.yml` uses `/mnt/deskos-ipa-work`, `identity-ad.yml`
`/mnt/deskos-ad-work`.

## The daily check

It runs daily and on demand, never on a pull request: it starts the pinned
server, builds the image and disk, boots the VM and runs the instrumented
session. The join unit runs before GDM, and the session asserts the join, SSSD
and the directory user. Verified 2026-10-27: `join_ok=yes`, `join_sssd=active`,
`session_user=qa`, `oddjobd=active`. The login is **autologin**: PAM's session
phase, the greeter and the home policy run, but no password is typed.

Three details that cost runs:
- `ipa-server-install` rejects a single-label domain and needs `-U` on a detached
  container;
- the join unit starts in `init_t`, so it runs the script through `/bin/bash`,
  since a `/var` file is `var_lib_t`;
- a directory user's home does not exist before first login, so the probes live
  in `/var/lib/deskos-qa` and start from `/etc/xdg/autostart`.

## Still unproven

- An interactive GDM login, with a person typing the directory password.
- A FreeIPA realm with integrated DNS; only the AD check has one.
- A production directory: the throwaway realm covers none.
