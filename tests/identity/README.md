# Identity client fixtures

A test fixture, never a support claim. `identity-freeipa.yml` and `identity-ad.yml`
run daily and on demand, join a booted VM to a disposable realm, and log in at GDM
as the directory user.

## What this proves

The identity client is an **organization profile**, not Core:
`tests/identity/resources/` adds `ipa-client` and `oddjob-mkhomedir` to a CentOS
Stream 10 workstation that is otherwise Core plus the example apps. The image
carries **no realm, principal or password**: enrolment is a runtime act on the
machine, never an artifact input ([runtime.md](../../docs/runtime.md#provisioning-and-enrollment)).

The containers and directories here are throwaway. Never point them at a real
organization.

## Build the image and disk

```bash
deskosctl render tests/identity/resources \
  --workstation identity-client-centos-stream-10 --output dist/identity-client
sudo podman build -t localhost/identity-client:test dist/identity-client
```

Then build a QCOW2 exactly as [tests/vm/README.md](../vm/README.md) describes. The
identity workstation is not published.

## FreeIPA

Pin the server image you tested; do not float a tag. The image runs systemd, and it
supports neither privileged mode nor host networking, so the script uses the bridge
network, publishes the realm ports (80, 88, 389, 443, 464, 636) and passes `-U`
with the realm and domain, because `ipa-server-install` prompts by default and a
detached container dies on the prompt.

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

The domain needs at least two labels (`ipa-server-install` rejects `test`), so the
fixture uses `lab.test`, which cannot collide with a real realm. On the bridge the
container resolves its own name, the certificate names `ipa.lab.test`, and the
guest reaches it through the published ports. Data lives in the
`deskos-freeipa-data` volume: `stop` removes the container and keeps the volume,
`purge` removes both.

**Join the guest.** Boot the disk with QEMU user networking (the host is
`10.0.2.2`), log in as the local installer account, copy [join.sh](join.sh) and run
it as root:

```bash
sudo IPA_DOMAIN=lab.test IPA_REALM=LAB.TEST IPA_SERVER=ipa.lab.test \
  IPA_PRINCIPAL=admin IPA_PASSWORD='…' ./join.sh
```

The script pins the server to `10.0.2.2` in `/etc/hosts`, then runs
`ipa-client-install --force --mkhomedir --no-ntp --force-join` with a host name
inside the realm. This realm has **no integrated DNS** (port 53 belongs to the
host resolver), so the client reaches it through the hosts file and `--force` skips
the checks that expect a resolvable name. A realm with DNS is a different setup. A
password in argv is visible in the guest's process list: this directory is
throwaway.

**Log in as the directory user** at GDM and record:

```bash
id
getent passwd qa
sssctl user-checks qa
systemctl status sssd oddjobd
journalctl -b -u sssd --no-pager
```

`oddjob-mkhomedir` creates the home at first login.

**Evidence to return:** the server image digest and `ipa --version`; the realm and
domain (throwaway values); the client and disk digests; the join output, `id`,
`sssctl user-checks`, the SSSD journal and the login screenshot; the model, BIOS
and kernel if it is not a VM. Keep passwords and the realm private.

## Samba AD DC

`tests/identity/samba/` builds a real AD implementation — Kerberos, LDAP, DNS,
`samba-tool` — not a mock, so the client path (`realm`, `adcli`, `sssd-ad`) runs
without Windows. For the client it is **equivalent to a Windows AD domain**: the
same code path, which is what makes the check meaningful. It is **not the same
product**: no Group Policy, no Windows replication, no support statement.

```bash
sudo podman build -t localhost/deskos-ad:test tests/identity/samba
export AD_IMAGE=localhost/deskos-ad:test ADMIN_PASSWORD='a throwaway Administrator password'
sudo -E tests/identity/samba-ad.sh start
sudo tests/identity/samba-ad.sh status
export AD_USER=qa AD_USER_PASSWORD='a throwaway user password'
sudo -E tests/identity/samba-ad.sh user
```

Two dependencies: **run as root** (provisioning needs real ownership and
`security.*` xattrs; rootless panics in `set_nt_acl`), and **the client must use the
DC's DNS before it joins** — `identity-ad.yml` points NetworkManager at the realm's
resolver through `JOIN_DNS`; the DC is published on the host loopback and appears
as `10.0.2.2`. With a trust in front, the client would point at FreeIPA and FreeIPA
would forward the AD zone.

The domain is provisioned with `--use-rfc2307`, so `uidNumber`, `gidNumber`,
`unixHomeDirectory` and `loginShell` live in the directory. Verified 2026-10-27: the
daily check joins the VM, GDM logs in as `qa@ad.lab.test`, the home is created then,
and `oddjobd` is active.

Three things easy to miss:
- **The realm's DNS is the source of truth** for the DC's address; pointing the DC
  elsewhere breaks the join with `Insufficient permissions`.
- **`access_provider = ad` with no rule denies every account**, and `realm permit
  --all` exits zero without writing one: the fixture sets `access_provider =
  permit`, and a real organization narrows it to groups.
- **sssd starts before the join points the guest at the realm DNS**, so it can come
  up offline: the check restarts it, pins `ad_server` and waits for the domain to
  report online.

**One machine, one realm.** A disk that joined FreeIPA keeps `krb5.conf`,
`sssd.conf`, the keytab, the default realm and the `authselect` state; joining it to
AD afterwards is a schema conflict. The checks therefore never share a VM, disk,
workspace or image tag: `identity-freeipa.yml` uses `/mnt/deskos-ipa-work`,
`identity-ad.yml` `/mnt/deskos-ad-work`.

## The daily check

It runs daily and on demand, never as a pull-request gate: it starts the pinned
server, builds the image and disk, boots the VM and runs the instrumented session.
The join unit runs before GDM with realm, server and password injected at runtime,
and the session asserts the join, SSSD and the directory user, then logs in at GDM.
Verified 2026-10-27: `join_ok=yes`, `join_sssd=active`,
`join_user=qa:…:/home/qa:/bin/sh`, `session_user=qa`, `oddjobd=active`, and the home
was missing before the login and owned by the uid SSSD resolved after it.

The login is **autologin**: PAM's session phase, the greeter and the home policy
run, but no password is typed.

Three details that cost runs:
- `ipa-server-install` rejects a single-label domain and needs `-U` on a detached
  container;
- the join unit starts in `init_t`, so it runs the script through `/bin/bash`: a
  file under `/var` is `var_lib_t`, readable but not executable by `init_t`;
- a directory user's home does not exist before first login, so the probes live in
  `/var/lib/deskos-qa` and start from `/etc/xdg/autostart`, and the initial-setup
  marker travels in `/etc/skel`.

## Still unproven

- An interactive GDM login, with a person typing the directory password.
- A FreeIPA realm with its own integrated DNS; the AD check has one, the FreeIPA
  fixture does not.
- Anything about a production directory: the realm is throwaway and carries no
  support statement.
