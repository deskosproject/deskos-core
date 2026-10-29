# How other desktops test a session

**Surveyed 2026-10-29. Informs [session-e2e.md](session-e2e.md).**

Three families of practice exist for testing a desktop session that no unit test
can reach: boot the artifact, use it, and assert on what a person would see. They
differ in **what they assert on**, not in what they boot.

Project names are left out on purpose. The question this page answers is *which
method fits this repository*, and a method has to stand on its own: a comparison
that borrows another project's reputation is not evidence, and adopting a stack
wholesale would import assumptions that were never ours — a helper inside the
image, a scheduler, a screenshot library.

## Shape 1: compare the screen against stored references

An OS-level tester boots or installs the image in a virtual machine, sends
synthetic input through the VM's keyboard and mouse, and compares the screen with
a stored crop after each step.

- **Proves:** the installer and the desktop, on the artifact as shipped, including
  what a person notices and no API exposes.
- **Cost:** every UI change re-records references, and a person reviews each one.
  Theme, font and locale changes turn a real pass into a false failure.
- **As a gate:** it needs an image library and review discipline; the suite
  becomes a product of its own.
- **Where it is used:** release validation of a whole distribution, and gating
  updates on it.

## Shape 2: drive the accessibility tree

A driver inside the session speaks AT-SPI to the applications the desktop
exposes, finds widgets by role and name, and acts on them. On Wayland, input
injection is bridged through the desktop's **remote-desktop portal** by a
session-side helper.

- **Proves:** the desktop is usable through the same interface a screen reader
  uses — launch, navigate, type, close.
- **Cost:** application trees are imperfect (missing names, wrong roles, scroll
  bars that refuse to move), and the bridge helper is usually expected **inside**
  the image.
- **As a gate:** one suite, reusable across images, with assertions that survive a
  theme or font change.

## Shape 3: boot a digest and exercise the lifecycle

A post-build gate boots the published digest, runs upgrade, rollback and
persistence, and lets a promotion continue only if the run passed.

- **Proves:** the thing that breaks users most, on the artifact people pull.
- **Cost:** low.
- **What it does not do:** look at the desktop.

## Which shape this repository is

Shape 2 for the session, and shape 3 is already in place: `lifecycle.yml` stages a
second deployment, rolls back and recovers a failed candidate, and
`promotion.yml` moves only what the gate accepted.

Shape 1 stays **open on purpose**. `bootcheck.py` asserts a stable graphical frame
and the splash and does **not** match against references: a reference library is a
maintenance product we have not agreed to own, and a false failure in a weekly
gate costs more than it proves.

## What the survey changed

- **Input injection.** The first design said a root helper writing to
  `/dev/uinput` was how to send Super and typing. It is not: that bypasses the
  desktop. The remote-desktop portal is the supported route, and it is what the
  accessibility-driven suites use on Wayland. The open question is how a helper
  reaches an image that must not carry test packages — as a credential injected at
  boot, or as a package the platform already ships.
- **A time budget.** The suites that survive as gates state a full pass in the low
  tens of minutes; one states 10–15. Our session pass is about a minute and the
  boot pass about five, so the budget is a constraint on what we add, not a
  problem to solve.
- **Sharding.** One suite splits its scenarios across parallel jobs to keep the
  number. That matters when the pass stops fitting the budget, and not before.

## What we deliberately do not take

- **A helper inside the image.** The artifact under test must not carry test
  packages; a check that changes the thing it checks is not a check.
- **A scheduler and a web UI.** A database of runs is infrastructure, and the
  evidence index already answers "which digest passed what".
- **A screenshot reference library.** See shape 1.
- **A BDD layer.** The steps are few and the summary is the artifact; a `.feature`
  file adds indirection, not proof.
- **Another suite's scope.** Installer, first boot, lifecycle, session and supply
  chain are separate checks here; importing a foreign suite would duplicate them,
  and the duplication is what rots first.

## Still unverified

- Whether a portal-bridge helper can be delivered without adding it to the image,
  and whether it is packaged for the platform this repository builds today.
- Whether a Settings page and the lock screen expose enough of a tree to assert on
  (scenarios 2 and 3 of the design).

Nothing on this page is a claim about another project's quality. It is a reading
of public documentation in October 2026, written down so that the design can cite
a method instead of a reputation.
