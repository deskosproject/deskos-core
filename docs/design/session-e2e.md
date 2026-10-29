# Session E2E through accessibility

**Status: scenario 1 implemented 2026-10-29 (run `40795106343`, both images);
scenarios 2–4 designed.** Drive a GNOME session the way a person does, and fail the
gate where a user would be stuck.

## What exists, and what it does not prove

`tests/vm/sessioncheck.py` boots a throwaway overlay of the tested disk, injects a
test user and a script as **systemd credentials through SMBIOS**, autologins, and
reads the GNOME state back over the serial port. It asserts every dconf default the
Plan declares, the extensions that must load, the favorites, the wallpaper, and
that Firefox starts headless against a marker page.

It proves the state is what the Plan says. Since 2026-10-29 it also lists the
applications the accessibility registry knows and launches one from its desktop
entry, asserting that a new window appears. It does not prove a person can use the
session: the login is an autologin, no menu is clicked, and a shell that draws
correctly but ignores input would pass every step but the launch.

## The shape

The same transport, a different script: the guest runs an **accessibility driver**
inside the session, writes one JSON line per step to the serial port, and the host
asserts on those lines plus QMP frames taken between steps.

The driver speaks **AT-SPI through `python3-pyatspi`**, which the published image
already carries. Checked against the installed SBOM of `sha256:871e59cc…`:
`at-spi2-core 2.56.1`, `at-spi2-atk 2.56.1`, `python3-pyatspi 2.46.1`, `gjs 1.80.2`
and `gnome-shell 49.5`.

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

1. **Launch an application and see its window.** The declared favorite is started
   through its desktop entry with `gio launch` — the path its icon takes — and the
   run asserts that a **new** application with at least one toplevel appears in the
   tree. Implemented 2026-10-29 with `org.gnome.Calculator.desktop`.

   *Keyboard injection is deferred.* Mutter on Wayland rejects synthetic key events
   from an ordinary client, and a root helper writing to `/dev/uinput` would go
   around the desktop — the **wrong shape**. The supported route is the
   **remote-desktop portal**, which a session-side helper uses to send input. Whether
   that helper is packaged for EL10 is the open question, and it is not a
   prerequisite for this assertion.
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
`bootcheck.py` does — a frame assertion — and the login logo is checked as a file.

## Determinism and cost

- Every step **waits for a state** with a timeout, then asserts; nothing sleeps a
  fixed time. A timeout fails with the last tree state, so the report says whether
  the app was absent or merely slow.
- Steps are independent: a failure is reported with its step name and the run
  continues where safe.
- Screenshots are taken **between** steps and only when the screen changed, the
  same rule `bootcheck.py` uses for frames.
- Cost: a few seconds per step on a booted session, after the session check in the
  same job.

## What the first run showed

The probe lists what is **already running** when the autostart script fires: the
shell, its helper daemons and the portal. Scenario 1 therefore cannot pass by
finding an application that was there all along — it must make one appear, and
assert on a window in the tree rather than on the application's presence.

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
- [x] A `pyatspi` call listing the applications the registry knows, over the serial
      transport. First run on 2026-10-29 (`deskos-core-centos-stream-10`): **nine**
      applications, all belonging to the session itself, so the next step has to
      make one appear.
- [x] Scenario 1, asserted in the harness and reported in the summary: launch
      through the desktop entry, a new application with a toplevel, then terminate
      it. The close *through the tree* stays scenario 4.
- [ ] Then 2 to 4, one at a time, each with its failure message.
- [ ] Wire the job into the gate, and only then into promotion.
