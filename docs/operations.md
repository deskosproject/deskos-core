# Operating an installed machine

How a DeskOS workstation updates, and how to inspect, apply and roll back
that update with what the image ships today. `deskos`, the planned endpoint
command (`status`, `update`, `rollback`), is not in the image yet; this page
uses `bootc` and `systemctl`.

## How updates work

A DeskOS image **stages** its next version; it never reboots your session on
its own. The image carries two services and their timers, on the schedule
the effective `UpdatePolicy` declares (DeskOS Core: daily, on AC power):

| Unit | Runs | Effect |
|---|---|---|
| `deskos-image-update.timer` → `deskos-image-update.service` | `bootc upgrade --quiet` | downloads the next image and **stages** it |
| `deskos-flatpak-update.timer` → `deskos-flatpak-update.service` | `flatpak update --system --noninteractive --assumeyes` | updates system Flatpak apps and runtimes |

> **Platform scope.** Everything `bootc` here is on every platform. The Flatpak
> half exists only where the image declares a system Flatpak channel — the EL
> family's `PackageSet/rhel-family` and the Fedora family's `PackageSet/fedora`
> provide one, and DeskOS Core adds neither the package nor the unit without one
> of those profiles; an organization can still declare them itself. Ubuntu 26.04
> declares no channel, so a `FlatpakSet` there fails composition rather than
> pretending, and on AlmaLinux the Flatpak preinstall path is **declared**, not
> exercised by the CI example ([platforms.md](platforms.md)).

The timers are `OnCalendar=daily` with a two-hour randomized delay and
`Persistent=true`; the services carry `ConditionACPower=true`. The base
image's own `bootc-fetch-apply-updates.timer` and `.service` are **masked**,
so the image owns updates rather than the base
(see [ADR 0007](adr/0007-updatepolicy.md)).

The staged image **applies at the next reboot**. An organization changes
the schedule, or turns automation off, in its own `UpdatePolicy`
(`image.automatic`, `image.schedule`, `image.requireACPower`, and the
`flatpak` equivalents). With `automatic: false` the service is still
written, but no timer, so an operator (or the future `deskos`) can run it
on demand. The kind is documented in [resources.md](resources.md#updatepolicy).

## Inspect

```bash
bootc status                        # booted and staged deployments, image ref
systemctl list-timers 'deskos-*'    # when the next update runs
systemctl status deskos-image-update.service
journalctl -u deskos-image-update.service --since today
```

On the EL platforms:

```bash
rpm-ostree status                   # the same, in ostree terms
flatpak list --system               # system apps and runtimes
```

`bootc status` shows a `Staged` entry once the next image is downloaded and
waiting for a reboot; `bootc status --verbose` says whether it is in
download-only mode.

## Update now, then apply

```bash
sudo systemctl start deskos-image-update.service     # stage the next image
sudo reboot                                          # apply the staged image
```

To check and stage in one step, without waiting for the timer:

```bash
sudo bootc upgrade
sudo reboot
```

`deskos update` is the same staging step behind a guard: it refuses when this is not a
bootc deployment or when bootc cannot read the deployment, and it reports — without
refusing — an image that updates itself and whatever the plan owns under `/etc`, since
customization there is the machine owner's.

On the EL platforms only, the Flatpak half:

```bash
sudo systemctl start deskos-flatpak-update.service   # update Flatpaks now
```

## Roll back

Every applied image keeps the previous deployment. To boot it:

```bash
sudo bootc rollback
sudo reboot
```

`bootc rollback` reorders the boot entries: the deployment marked `rollback`
becomes the next boot and the current one becomes the rollback, so rolling
back again returns you. Files under `/etc` revert with the deployment; `/var`
(including home directories) is preserved.

## Change where updates come from

A machine tracks the image reference it was installed from. To move it to a
different one (a new registry, a pinned digest):

```bash
sudo bootc switch registry.example.com/deskos/core-rhel10:latest
sudo reboot
```

## On the machine: `deskos`

`deskos` is the runtime companion to `deskosctl` (ADR 0018). The release ships it next
to `deskosctl`, with the same checksum coverage, and **every Core image carries it**:
Core declares a pinned `BinaryArtifact` for the release asset, so `/usr/local/bin/deskos`
is on the machine and nobody has to install a tool to ask what it was built from
(`/usr/local/bin`, because the model lets a `BinaryArtifact` write only there). The pin
lands with the first release that ships the binary.

```bash
deskos report        # what this machine was built from, read from the plan the image carries
deskos check         # compare the plan's declared scope with this machine, and say what it leaves out
deskos update        # update the image, after a guard that refuses only what is unsafe
```

It is not a reconciler: it changes the deployment the machine boots from and nothing
else, it never touches a home, and it does not re-apply the plan. `deskos check` prints
its scope on every run, because it measures what the artifact declares and not
everything the machine has become. `status` and `rollback` are not part of it: the
approved scope is those three commands, and `bootc status` and `bootc rollback` answer
the other two questions in the meantime.
