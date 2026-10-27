# Identity client fixture (FreeIPA)

Status: **phase 1**. The image side exists; the automated join and GDM login are
phase 2. This is a test fixture, never a support claim.

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

Pin the server image you tested; do not float a tag. The fixture needs
`podman`, a privileged container with systemd, and host networking. The script
passes `-U` and the realm and domain to `ipa-server-install`: the image runs the
installer interactively by default, which fails on a detached container.

```bash
export FREEIPA_IMAGE='freeipa/freeipa-server@sha256:…'
export FREEIPA_PASSWORD='a throwaway Directory Manager password'
export IPA_SERVER_HOSTNAME=ipa.test
export IPA_SERVER_DOMAIN=test
export IPA_SERVER_REALM=TEST

sudo -E tests/identity/freeipa.sh start
```

The host must resolve the server's own name, because its certificate names it:

```text
127.0.0.1 ipa.test
```

in `/etc/hosts` on the host. Wait until `sudo tests/identity/freeipa.sh status`
answers, then create the directory user:

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
sudo IPA_DOMAIN=test IPA_REALM=TEST IPA_SERVER=ipa.test \
  IPA_PRINCIPAL=admin IPA_PASSWORD='…' ./join.sh
```

The script pins `ipa.test` to `10.0.2.2` in the guest's `/etc/hosts`, then runs
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

## Phase 2

Automate the same path so it can run nightly or on demand, never on every pull
request:

- pass the join inputs as systemd credentials and run `ipa-client-install` in a
  first-boot unit;
- configure GDM autologin for the directory user (or drive the GDM password
  prompt) and run the instrumented session check;
- assert the session is the directory user, the home was created by oddjob, and
  SSSD is healthy;
- record the server image digest, realm and client digest in the evidence, and
  destroy the container and the VM afterwards.

The container is a fixture; the test says nothing about AD, production FreeIPA
or a corporate fleet.
