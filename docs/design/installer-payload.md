# The installer payload

**Status: proposal, 2026-10-26. Nothing here is implemented; it exists to
decide whether the installer's own tree is pinned or recorded as floating.**

The ISO is a delivery vehicle. What gets installed is an image digest; the
installer decides how that image reaches a disk and what a person is asked on
the way. That asymmetry is the whole question: pinning the image does not pin
the installer, and today the installer floats.

## What floats

- **Anaconda and lorax**, pulled from the distribution repositories when
  `bootc-image-builder` builds the ISO. They are not covered by the image pin.
- **The kickstart** is ours, and it is versioned with the image.
- **The payload image** is pinned: `bootc-image-builder` installs the digest the
  plan names, not `:latest`.

So a rebuild days later can ship the same image with a different installer, and
the same kickstart can behave differently. That is not hypothetical: a kickstart
with `graphical --non-interactive` installed on one RHEL 10.2 build and aborted
on another at the progress spoke (`User interaction required on standalone spoke
ProgressSpoke`), then rebooted — which reads as a hardware failure to the person
holding the laptop.

## Options

| Option | What it buys | What it costs |
|---|---|---|
| **Pin the installer tree** | the same ISO build inputs produce the same installer behaviour | a mirrored, dated compose of Anaconda/lorax per platform; the RHEL side needs entitlement to that snapshot, and the mirror becomes another thing to keep and retire |
| **Record that it floats, guard with a test install** | no new mirror; a change that breaks the install shows up before a person meets it | the ISO is not reproducible; a test-only ISO is not the published one, so an interactive-only regression can still escape |
| **Vendor a thin installer from a pinned image** | the installer is the image, so the pin covers it | a different install path (live image + `bootc install`), and the interactive storage story has to be rebuilt |

## Recommendation

Record that it floats for now, and make the guard explicit:

1. the test-only ISO installs unattended in CI (`installer-test.yml`), which is
   the check that would have caught the ProgressSpoke abort;
2. the ISO's README and the release note say the installer tree floats, so a
   difference between two builds of the same image is expected rather than
   surprising;
3. revisit the vendored-installer option if a second installer regression
   reaches a person.

This is a smaller promise than pinning: it says the **installed system** is the
digest we tested, and the **installer** is whatever the vendor shipped that day,
checked by an unattended install.

## What this does not decide

- Whether an organization's RHEL ISO follows the same rule. Its entitlement and
  mirroring are the organization's, not Core's.
- Whether `installer-test.yml` becomes a gate. It takes about 45 minutes and
  needs KVM; today it is dispatch-only.
- Whether the published, interactive ISO is ever installed by a check. Its
  storage is interactive, so only a person completes it.
