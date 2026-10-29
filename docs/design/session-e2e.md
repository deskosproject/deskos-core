# Session E2E through accessibility

**Status: designed 2026-10-29, not implemented.** This is the plan for Milestone 4:
drive a GNOME session the way a person does, and fail the gate where a user would
be stuck.

## What exists, and what it does not prove

`tests/vm/sessioncheck.py` boots a throwaway overlay of the tested disk, injects a
test user and a script as **systemd credentials through SMBIOS**, autologins, and
reads the GNOME state back over the serial port. It asserts every dconf default the
Plan declares, the extensions that must load, the favorites, the wallpaper, and
that Firefox starts headless against a marker page.

It proves the state is what the Plan says. It does **not** prove a person can use
the session: the login is an autologin, no window is ever opened by a user, no menu
is clicked, and a shell that draws correctly but ignores input would pass.

## The shape

The same transport, a different script: the guest runs an **accessibility driver**
inside the session, writes one JSON line per step to the serial port, and the host
asserts on those lines plus QMP frames taken between steps.

The driver speaks **AT-SPI through `python3-pyatspi`**, which the published image
already carries. Checked against the installed SBOM of `sha256:871e59cc…`:
`at-spi2-core 2.56.1`, `at-spi2-atk 2.56.1`, `python3-pyatspi 2.46.1`, `gjs 1.80.2`
and `gnome-shell 49.5`. An earlier version of this page said the binding was absent;
that was the base image, not the artifact.

`gdbus` (in `glib2`, always present) stays as the fallback if an image ever ships
without the binding, and the driver must fail with that reason rather than a
traceback. A test still may not add a package to the artifact it tests.

```
host                          guest (autologin session)
────                          ──────────────────────────
inject script (SMBIOS)   →    gdbus → org.a11y.Bus → registry
read serial JSON lines   ←    {"step":"open_overview","ok":true,...}
QMP screendump per step  ←    the screen a person saw
```

## The first four scenarios

Each is a small, bounded interaction with an assertion on the accessibility tree,
not on pixels:

1. **Open the overview and launch an application.** Super, type the app name, Enter,
   and assert the app's window appears in the tree with the expected name. This is
   the cheapest proof that keyboard input reaches the shell.
2. **Reach a page in Settings.** Open Settings, navigate to a known panel, assert
   the panel's title is exposed. A shell that launches apps but cannot navigate is
   still broken.
3. **Lock and unlock.** Lock from the shell, assert the lock screen is up, type the
   password, assert the session returns. This exercises PAM and the lock screen
   without the greeter's separate session.
4. **Open a file manager window and close it.** Assert the window appears and that
   closing it returns to the previous state — a basic window-lifecycle check that
   the session check cannot see.

The **GDM greeter** stays out of the driver: it runs in its own session and bus, and
reaching it from the autologin user is not something to fake. It remains what
`bootcheck.py` does today — a frame assertion — and the login logo is already
checked as a file. If a greeter scenario is wanted later, it is a separate design.

## Determinism and cost

- Every step **waits for a state** with a timeout, then asserts; nothing sleeps a
  fixed time and hopes. A timeout fails with the last tree state in the report, so
  the failure says whether the app was absent or merely slow.
- Steps are independent: a failure is reported with its step name and the run
  continues where safe, so one broken interaction does not hide the others.
- Screenshots are taken **between** steps and only when the screen changed, the
  same rule `bootcheck.py` uses for frames.
- Cost: a few seconds per step on a session that is already booted. It runs after
  the session check on the same instrumented boot, in the same job.

## What it does not claim

- Not an accessibility conformance test: it uses AT-SPI to drive, and says nothing
  about whether an app is usable with a screen reader.
- Not every application: four scenarios, extended when a real defect escapes.
- Not a substitute for the organization's field validation on hardware.

## What adopting it changes

`promotion.yml` would move a digest only after the E2E job passed on that digest, so
a session regression becomes a red gate instead of a user report. That is the
milestone's second line, and it needs the first one implemented before it is worth
promising.

## First checklist

- [x] Confirm the AT-SPI stack is in the published image: done against the installed
      SBOM of `sha256:871e59cc…` (`at-spi2-core`, `at-spi2-atk`, `python3-pyatspi`).
- [ ] A `pyatspi` call that lists the applications the registry knows, from the
      injected script, over the serial transport.
- [ ] Scenario 1 only, asserted in the harness and reported in the summary.
- [ ] Then 2 to 4, one at a time, each with its failure message.
- [ ] Wire the job into the gate, and only then into promotion.
