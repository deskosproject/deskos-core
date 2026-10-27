# Identity client fixture (FreeIPA)

Status: **phase 2, automated**. `identity-freeipa.yml` joins a booted VM to a
disposable realm daily and logs in at GDM as the directory user. This is a test
fixture, never a support claim.

## What this proves

The identity client is an **organization profile**, not Core:

- `tests/identity/resources/` adds `ipa-client` and `oddjob-mkhomedir` to a
  CentOS Stream 10 workstation that is otherwise Core plus the example apps.
- The image carries **no realm, principal or password**. Enrollment is a
  runtime act on the machine, never an artifact input (architecture.md,
  “Enrollment”).
- The packages are present, so a person can join a domain without layering RPMs
  on a running image.

The container and the directory in this procedure are throwaway. Do not point
this fixture at a real organization.

## Build the image and disk

```bash
deskosctl render tests/identity/resources \
  --workstation identity-client-centos-stream-10 --output dist/identity-client
sudo podman build -t localhost/identity-client:test dist/identity-client
```

Then build a QCOW2 exactly as [tests/vm/README.md](../vm/README.md) describes,
with `bootc-image-builder --type qcow2 --no-default-kernel-args` and the image
above. The identity workstation is not published.

## Start a disposable FreeIPA

Pin the server image you tested; do not float a tag. The fixture needs `podman`
and a container running systemd. The image does **not** support privileged mode
or host networking, so the script runs it on the bridge network and publishes
the realm ports (80, 88, 389, 443, 464, 636). It also passes `-U` and the realm
and domain to `ipa-server-install`: the image runs the installer interactively
by default, which fails on a detached container.

```bash
export FREEIPA_IMAGE='freeipa/freeipa-server@sha256:…'
export FREEIPA_PASSWORD='a throwaway Directory Manager password'
export IPA_SERVER_HOSTNAME=ipa.lab.test
export IPA_SERVER_DOMAIN=lab.test
export IPA_SERVER_REALM=LAB.TEST

sudo -E tests/identity/freeipa.sh start
```

The domain needs at least two labels — `ipa-server-install` rejects a
single-label domain such as `test` — so this fixture uses `lab.test`, which
cannot collide with a real realm. On the bridge the container resolves its own
name to its own address; the certificate names `ipa.lab.test`, and the guest
reaches it through the published ports. Wait until
`sudo tests/identity/freeipa.sh status` answers, then create the directory user:

```bash
export FREEIPA_USER=qa
export FREEIPA_USER_PASSWORD='a throwaway user password'
sudo -E tests/identity/freeipa.sh user
```

The fixture keeps its data in the `deskos-freeipa-data` volume. `stop` removes
the container and keeps the volume; `purge` removes both.

## Join the guest

Boot the disk with QEMU user networking, which exposes the host as `10.0.2.2`.
Log in as the local installer account (the identity image is not enrolled yet),
copy [join.sh](join.sh) to the guest, and run it as root:

```bash
sudo IPA_DOMAIN=lab.test IPA_REALM=LAB.TEST IPA_SERVER=ipa.lab.test \
  IPA_PRINCIPAL=admin IPA_PASSWORD='…' ./join.sh
```

The script pins `ipa.lab.test` to `10.0.2.2` in the guest's `/etc/hosts`, then runs
`ipa-client-install` with `--force`, a host name inside the realm and
`--mkhomedir --no-ntp --force-join`. The server certificate and the host name
match. The fixture realm has **no integrated DNS** (port 53 belongs to the
host resolver): the client reaches the server through its hosts file, and
`--force` skips the checks that expect a resolvable name. A realm with DNS is a
different setup, not this fixture.

A password in argv is visible to the guest's process list. This directory is
throwaway; never do it on a real realm.

## Log in as the directory user

Log out to GDM and log in as the user created above. Record:

```bash
id
getent passwd qa
sssctl user-checks qa
systemctl status sssd oddjobd
journalctl -b -u sssd --no-pager
```

and a screenshot of the GDM login and the desktop. The home directory is created
by `oddjob-mkhomedir` on first login.

## Evidence to return

- the FreeIPA server image digest and version (`ipa --version` in the container);
- the realm and domain (throwaway values);
- the client image digest and the disk digest;
- the join output, `id`, `sssctl user-checks`, the SSSD journal and the login
  screenshot;
- the model, BIOS and kernel of the machine if it is not a VM.

Keep the passwords and the realm private. The fixture is not a production
directory and carries no support statement.

## The other model: a disposable Samba AD DC

`tests/identity/samba/` builds a Samba AD DC and `samba-ad.sh` runs it. It is a
real AD implementation — Kerberos, LDAP, DNS, `samba-tool` — not a mock, so the
AD client path (`realm`, `adcli`, `sssd-ad`) can be exercised without Windows.
It is **not** Windows: Group Policy processing, Windows DC replication and
Microsoft-specific extensions are out of scope and nothing here claims them.

```bash
sudo podman build -t localhost/deskos-ad:test tests/identity/samba
export AD_IMAGE=localhost/deskos-ad:test
export ADMIN_PASSWORD='a throwaway Administrator password'
sudo -E tests/identity/samba-ad.sh start
sudo tests/identity/samba-ad.sh status
export AD_USER=qa AD_USER_PASSWORD='a throwaway user password'
sudo -E tests/identity/samba-ad.sh user
```

Two things the fixture depends on:

- **Run it as root.** Provisioning needs real ownership and `security.*` xattrs;
  a rootless container panics in `set_nt_acl` with `Security context active token
  stack underflow`.
- **The client must use the DC's DNS before it joins.** `identity-ad.yml` points
  NetworkManager at the realm's resolver through `JOIN_DNS`; the DC is published
  on the host loopback and QEMU user networking exposes it as `10.0.2.2`. With a
  trust in front (FreeIPA), the client would point at FreeIPA instead and
  FreeIPA would forward the AD zone.

The domain is provisioned with `--use-rfc2307`, so `uidNumber`, `gidNumber`,
`unixHomeDirectory` and `loginShell` live in the directory and the Unix identity
comes from the realm.

**One machine, one realm.** A disk that has joined FreeIPA keeps `krb5.conf`,
`sssd.conf`, the machine keytab, the default realm and the `authselect` state;
joining it to AD afterwards is a schema conflict, not a change of provider. The
two checks therefore never share a VM, a disk, a workspace or an image tag:
`identity-freeipa.yml` works in `/mnt/deskos-ipa-work` and
`identity-ad.yml` in `/mnt/deskos-ad-work`, each building its own QCOW2 from
its own image. Reproducing one by hand on a disk left over from the other is
not a supported path.

## The daily check

`.github/workflows/identity-freeipa.yml` runs daily and on demand, never as a
pull-request gate. It starts the pinned server, builds the identity image and
disk, boots the VM and runs the instrumented session. The join unit runs before
GDM with the realm, server and admin password injected at runtime, and the
session asserts the join completed, SSSD is active and the directory user
resolves through it, and then logs in at GDM as that user. Verified on
2026-10-27: `join_ok=yes`, `join_sssd=active`,
`join_user=qa:…:/home/qa:/bin/sh`, `session_user=qa`, `oddjobd=active`, and the
home was `missing` before the login and owned by the uid SSSD resolved after it
— that is `oddjob-mkhomedir` creating it at login, not a directory prepared in
advance.

The login is **autologin**: PAM's session phase, the greeter and the home policy
run, but no password is typed. An interactive GDM login is still unproven.

Three details that cost runs and are worth keeping:

- `ipa-server-install` rejects a single-label domain, so the fixture uses
  `lab.test`, and it must be told `-U` or it prompts and dies on a detached
  container;
- the join unit starts in `init_t`, so it runs the script through `/bin/bash`:
  a file under `/var` is `var_lib_t`, which `init_t` may read but not execute;
- a directory user's home does not exist before its first login, so the session
  probes live in `/var/lib/deskos-qa` and start from `/etc/xdg/autostart`, and
  the initial-setup marker travels in `/etc/skel` so `oddjob-mkhomedir` still
  creates the home.

## Still unproven

- an interactive GDM login: a person typing the directory password at the
  greeter, rather than autologin;
- AD instead of FreeIPA, and a realm with its own DNS (this fixture has none);
- the server image digest and realm are recorded in the evidence, and the
  container and VM are destroyed afterwards, but the realm is throwaway and
  says nothing about a production directory.

The container is a fixture; the test says nothing about AD, production FreeIPA
or a corporate fleet.
