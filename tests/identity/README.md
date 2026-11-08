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
export IPA_SERVER_HOSTNAME=idm1.idm.example.com
export IPA_SERVER_DOMAIN=idm.example.com
export IPA_SERVER_REALM=IDM.EXAMPLE.COM
sudo -E tests/identity/freeipa.sh start

export FREEIPA_USER=qa
export FREEIPA_USER_PASSWORD='a throwaway user password'
sudo -E tests/identity/freeipa.sh user
```

The domain needs at least two labels (`ipa-server-install` rejects `test`), so
the fixture uses `idm.example.com`, which cannot collide with a real realm.

**Join the guest.** Boot the disk with QEMU user networking (host `10.0.2.2`),
log in as the local installer, copy [join.sh](join.sh) and run it as root:

```bash
sudo IPA_DOMAIN=idm.example.com IPA_REALM=IDM.EXAMPLE.COM IPA_SERVER=idm1.idm.example.com \
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

Samba on Fedora 45 (`samba/Dockerfile`), built in the run and never published.
It is the only DC in the fixture: the Debian build was removed because a
harness that tests Fedora and CentOS clients should not depend on a third
distribution. Fedora's Samba completes the cross-realm service ticket the
Debian build rejected (`HANDLE_AUTHDATA`, run 42386803763 against 42416547551).

The direct join is exercised from the Fedora 45 client as well: with a Debian
DC and a CentOS Stream 10 client it worked, with Fedora's Samba and the same
client the machine-password exchange is refused (`gensec_unwrap failed -
NT_STATUS_ACCESS_DENIED`, run 42430711881), so both sides are now the same
family.

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
