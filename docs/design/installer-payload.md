# The installer payload

**Status: accepted, 2026-10-26.** The installer tree **floats**, and
`installer-test.yml` is the guard: it installs the test-only ISO unattended and
fails if Anaconda does not reach "Install finished". The pinned image is the
installed system; the ISO is delivery only, a smaller promise than pinning, and
the one we can keep today.

## What floats

- **Anaconda and lorax**, pulled from the distribution repositories when
  `bootc-image-builder` builds the ISO, outside the image pin.
- **The kickstart** is ours, and it is versioned with the image.
- **The payload image** is pinned: `bootc-image-builder` installs the digest the
  plan names, not `:latest`.

A rebuild days later can ship the same image with a different installer, and the
same kickstart can behave differently. A kickstart with `graphical
--non-interactive` installed on one RHEL 10.2 build and aborted on another at the
progress spoke (`User interaction required on standalone spoke
ProgressSpoke`), then rebooted, which reads as a hardware failure to the person
holding the laptop.

## Options

| Option | What it buys | What it costs |
|---|---|---|
| **Pin the installer tree** | the same ISO build inputs produce the same installer behaviour | a mirrored, dated compose of Anaconda/lorax per platform; the RHEL side needs entitlement to that snapshot, and the mirror becomes another thing to keep and retire |
| **Record that it floats, guard with a test install** | no new mirror; a change that breaks the install shows up before a person meets it | the ISO is not reproducible; a test-only ISO is not the published one, so an interactive-only regression can still escape |
| **Vendor a thin installer from a pinned image** | the installer is the image, so the pin covers it | a different install path (live image + `bootc install`), and the interactive storage story has to be rebuilt |

## Decision

Record that it floats, and make the guard explicit:

1. the test-only ISO installs unattended in CI (`installer-test.yml`), the check
   that would have caught the ProgressSpoke abort;
2. the ISO's README and the release note say the installer tree floats, so a
   difference between two builds of the same image is expected, not surprising;
3. revisit the vendored-installer option if a second installer regression arrives.

The **installed system** is the digest we tested; the **installer** is whatever
the vendor shipped that day, checked by an unattended install.

## What this does not decide

- Whether an organization's RHEL ISO follows the same rule. Its entitlement and
  mirroring are the organization's, not Core's.
- Whether `installer-test.yml` becomes a weekly check. It takes about 45 minutes
  and needs KVM; today it is dispatch-only.
- Whether the published, interactive ISO is ever installed by a check. Its
  storage is interactive, so only a person completes it.
