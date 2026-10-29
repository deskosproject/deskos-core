# How other desktops test a session

**Surveyed 2026-10-29. Informs [session-e2e.md](session-e2e.md).**

Three families of practice exist for testing a session no unit test can reach:
boot the artifact, use it, assert on what a person would see. They differ in
**what they assert on**, not in what they boot. Project names are left out: a
method has to stand on its own, and a comparison that borrows another project's
reputation proves nothing.

## Shape 1: compare the screen against stored references

An OS-level tester boots or installs the image in a virtual machine, sends
synthetic input through the VM's keyboard and mouse, and compares the screen with
a stored crop after each step.

- **Proves:** the installer and the desktop, on the artifact as shipped,
  including what a person notices and no API exposes.
- **Cost:** every UI change re-records references, and a person reviews each one;
  theme, font and locale changes turn a real pass into a false failure.
- **As a check:** it needs an image library and review discipline, so the suite
  becomes a product of its own.
- **Where it is used:** release validation of a whole distribution.

## Shape 2: drive the accessibility tree

A driver inside the session speaks AT-SPI to the applications the desktop
exposes, finds widgets by role and name, and acts on them. On Wayland, input
injection is bridged through the desktop's **remote-desktop portal** by a
session-side helper.

- **Proves:** the desktop is usable through the interface a screen reader uses:
  launch, navigate, type, close.
- **Cost:** application trees are imperfect (missing names, wrong roles, scroll
  bars that refuse to move), and the bridge helper is usually expected **inside**
  the image.
- **As a check:** one suite, reusable across images, with assertions that survive
  a theme or font change.

## Shape 3: boot a digest and exercise the lifecycle

A post-build check boots the published digest, runs upgrade, rollback and
persistence, and lets a promotion continue only if the run passed.

- **Proves:** the thing that breaks users most, on the artifact people pull.
- **Cost:** low.
- **What it does not do:** look at the desktop.

## Which shape this repository is

Shape 2 for the session; shape 3 is already in place: `lifecycle.yml` stages a
second deployment, rolls back and recovers a failed candidate, and
`promotion.yml` moves only what the check accepted.

Shape 1 stays **open on purpose**: `bootcheck.py` asserts a stable graphical frame
and the splash and does **not** match references, because a reference library is a
maintenance product we have not agreed to own.

## What the survey changed

- **Input injection.** A root helper writing to `/dev/uinput` bypasses the
  desktop; the remote-desktop portal is the supported route on Wayland. The open
  question is how a helper reaches an image that must not carry test packages: as
  a credential injected at boot, or as a package the platform already ships.
- **A time budget.** Suites that survive as checks state a full pass in the low
  tens of minutes, one at 10-15. Our session pass is about a minute and the boot
  pass about five, so the budget constrains what we add.
- **Sharding.** One suite splits scenarios across parallel jobs. That matters when
  the pass stops fitting the budget, not before.

## What we deliberately do not take

- **A helper inside the image.** The artifact under test must not carry test
  packages; a check that changes the thing it checks is not a check.
- **A scheduler and a web UI.** A database of runs is infrastructure, and the
  record index already answers "which digest passed what".
- **A screenshot reference library.** See shape 1.
- **A BDD layer.** The steps are few and the summary is the artifact; a `.feature`
  file adds indirection, not proof.
- **Another suite's scope.** Installer, first boot, lifecycle, session and supply
  chain are separate checks here; a foreign suite would duplicate them.

## Still unverified

- Whether a portal-bridge helper can be delivered without adding it to the image,
  and whether it is packaged for the platform this repository builds today.
- Whether a Settings page and the lock screen expose enough of a tree to assert on
  (scenarios 2 and 3 of the design).
