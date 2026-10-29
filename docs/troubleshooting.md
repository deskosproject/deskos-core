# Troubleshooting

Symptom, then the check that distinguishes it, then the remedy. Collect
[what a report needs](#what-to-send) before changing anything.

## The update guard refuses to run

```text
[REFUSED] this machine's deployment can be read: bootc could not read the
running deployment (bootc status: exit status 1); an update would run blind
```

`deskos update` reads the deployment through `bootc`, which needs root:

```bash
sudo deskos update --dry-run
sudo deskos update --apply
```

## "No update available" right after a release

The timer may have applied it already; it waits for a reboot:

```bash
sudo bootc status
```

`Booted image` names the digest. If it is older than the published one:

```bash
sudo deskos update --apply
```

## A Flatpak install fails while offline

```text
error: No se puede cargar el resumen del remoto flathub: While fetching
https://dl.flathub.org/repo/summary.idx: [6] Could not resolve hostname
```

A system Flatpak needs the remote. Reconnect and retry:

```bash
flatpak update --appstream
flatpak install --system flathub org.example.App
```

## The first-boot Flatpak unit failed

```bash
systemctl status deskos-flatpak-preinstall.service
journalctl -u deskos-flatpak-preinstall.service -b
```

The unit refreshes the appstream data, installs the declared Flatpaks and retries
on failure. A stale Flathub summary is the usual cause:

```bash
sudo flatpak update --appstream
sudo systemctl restart deskos-flatpak-preinstall.service
```

## The clock is hours off

```bash
timedatectl
```

`NTP service: active` and `System clock synchronized: yes` are healthy. If
`RTC in local TZ: yes` appears and the machine does not boot Windows, store the
hardware clock in UTC:

```bash
sudo timedatectl set-local-rtc 0
```

## A system unit failed

```bash
systemctl --failed
journalctl -b -u <unit> --no-pager
```

On AMD guests without `edac_mce_amd`, `mcelog.service` fails and is known and
non-blocking; the session check records it. Read any other failed unit before
rebooting.

## The machine does not boot after an update

`bootc` keeps the previous deployment: choose the older boot-menu entry, or roll
back and reboot if the session is unusable:

```bash
sudo bootc rollback
sudo reboot
```

Nothing in `/var` is rolled back with the image: Flatpaks and home directories stay.

## An application is missing from the grid

Declared favorites are part of the image; a store application is not:

```bash
flatpak list --system --app
```

Install it from Bazaar or with `flatpak install`. One that must exist before
login, or for every user, belongs in the image.

## An AD-joined machine cannot log in

Three things cause it, in this order:

1. **The machine is not using the realm's DNS.** An AD client finds its domain
   controller through DNS; if `/etc/resolv.conf` points elsewhere, the join fails
   or the domain stays offline. `resolvectl status` must show the realm's resolver.

2. **`realm join` leaves `access_provider = ad` with no rule, which denies
   everyone.** `getent passwd user@domain` resolves, and `sssctl user-checks`
   answers `pam_acct_mgmt: Permission denied`. Admit accounts explicitly:

   ```bash
   sudo realm permit --all          # or: realm permit --groups 'linux-users'
   ```

3. **`sssd` started before the machine knew the realm.** It comes up marked
   offline and does not rediscover the controller. Restart it after the network is
   configured:

   ```bash
   sudo systemctl restart sssd
   sssctl domain-status ad.example.com   # "Online status: Online"
   ```

## What to send

```bash
deskos report
deskos check
sudo bootc status
journalctl -b -p err --no-pager
systemctl --failed --no-pager
```

Include the exact message, the booted digest and the machine model. Serial
numbers, session screenshots and registry credentials stay private. The field
report belongs to the organization that owns the machine.
