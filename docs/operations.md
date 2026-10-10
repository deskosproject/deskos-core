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
rpm-ostree status                   # the same, in ostree terms
systemctl list-timers 'deskos-*'    # when the next update runs
systemctl status deskos-image-update.service
journalctl -u deskos-image-update.service --since today
flatpak list --system               # system apps and runtimes
```

`bootc status` shows a `Staged` entry once the next image is downloaded and
waiting for a reboot; `bootc status --verbose` says whether it is in
download-only mode.

## Update now, then apply

```bash
sudo systemctl start deskos-image-update.service     # stage the next image
sudo systemctl start deskos-flatpak-update.service   # update Flatpaks now
sudo reboot                                          # apply the staged image
```

To check and stage in one step, without waiting for the timer:

```bash
sudo bootc upgrade
sudo reboot
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

## When `deskos` lands

The endpoint command (roadmap, Milestone 7) wraps these units: `deskos
status` for what is staged, `deskos update` to run the image-update service
on demand, `deskos rollback`. Until it ships, the commands above are the
supported path.
