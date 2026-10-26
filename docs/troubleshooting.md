# Troubleshooting

Read before changing anything, and collect [what a report needs](#what-to-send)
first. Every remedy below is one a check or a field validation exercised; a
symptom that is not here belongs in an issue, not in a guess.

## The update guard refuses to run

```text
[REFUSED] this machine's deployment can be read: bootc could not read the
running deployment (bootc status: exit status 1); an update would run blind
```

`deskos update` reads the deployment through `bootc`, which needs root. Run it
with `sudo`:

```bash
sudo deskos update --dry-run
sudo deskos update --apply
```

An unprivileged run cannot see the deployment, and the guard refuses instead of
updating blind.

## "No update available" right after a release

The image updates itself on a timer (`update-policy`), so the update may already
be applied and waiting for a reboot, or already booted. Check what is running
and compare its digest with the published one:

```bash
sudo bootc status
```

`Booted image` names the digest the machine is on; `Rollback image` is the
previous deployment. If the booted digest is the one you expected, there is
nothing to do. If it is older, apply and reboot:

```bash
sudo deskos update --apply
```

## A Flatpak install fails while offline

```text
error: No se puede cargar el resumen del remoto flathub: While fetching
https://dl.flathub.org/repo/summary.idx: [6] Could not resolve hostname
```

That is the expected failure: a system Flatpak needs the remote. Reconnect and
retry:

```bash
flatpak update --appstream
flatpak install --system flathub org.example.App
```

## The first-boot Flatpak unit failed

```bash
systemctl status deskos-flatpak-preinstall.service
journalctl -u deskos-flatpak-preinstall.service -b
```

The unit refreshes the appstream data and then installs the declared Flatpaks;
it retries on failure. A stale Flathub summary is the usual cause. Refresh it
by hand and restart the unit:

```bash
sudo flatpak update --appstream
sudo systemctl restart deskos-flatpak-preinstall.service
```

The Flatpaks themselves live in `/var/lib/flatpak`, so they survive an image
update.

## The clock is hours off

```bash
timedatectl
```

`NTP service: active` and `System clock synchronized: yes` are what a healthy
machine shows. If `RTC in local TZ: yes` appears and the machine does not boot
Windows, store the hardware clock in UTC:

```bash
sudo timedatectl set-local-rtc 0
```

A clock that is hours off writes misleading evidence and breaks certificate
checks.

## A system unit failed

```bash
systemctl --failed
journalctl -b -u <unit> --no-pager
```

On AMD guests without `edac_mce_amd`, `mcelog.service` fails and is known and
non-blocking; the session check records it as such. Any other failed unit is
worth reading before rebooting.

## The machine does not boot after an update

`bootc` keeps the previous deployment. If the boot menu appears, choose the
older entry; if the machine boots but the session is unusable, roll back from a
working deployment and reboot:

```bash
sudo bootc rollback
sudo reboot
```

Nothing in `/var`, including Flatpaks and home directories, is rolled back with
the image.

## An application is missing from the grid

Declared favorites are part of the image; a store application is not. Check
whether it is a system Flatpak:

```bash
flatpak list --system --app
```

If it is not there, install it from Bazaar or with `flatpak install`. An
application that must exist before login, or for every user, belongs in the
image, not in a user install.

## What to send

```bash
deskos report
deskos check
sudo bootc status
journalctl -b -p err --no-pager
systemctl --failed --no-pager
```

Include the exact message, the booted digest and the machine model. Serial
numbers, screenshots of a session and registry credentials stay private; see
[field-validation.md](field-validation.md) for what a field report collects.
