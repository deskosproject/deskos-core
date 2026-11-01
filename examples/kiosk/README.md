# Example: a locked-down kiosk

A **role** that turns the desktop into a public session: one favorite, no command
line, no printing, no saving to disk and no user administration. The settings are
locks, so a person cannot turn them back on from the session.

```bash
deskosctl plan ./examples/kiosk \
  --workstation example-kiosk
```

| Resource | Contents |
|---|---|
| `example-kiosk` (role) | the kiosk desktop profile |
| `example-kiosk-desktop` | lockdown booleans, their locks and one favorite |
| `example-kiosk` (workstation) | CentOS Stream 10 + the Core desktop + this role |

## What it shows

- **Locks are image-level.** The values are written as dconf locks, so they beat a
  user's own settings and the `local`/`site` databases (invariant 8). Core never
  locks; this example does, deliberately.
- **A public session still boots the full desktop.** The dock, the browser and
  the DeskOS look are the same resources as any other workstation.
- **The lockdown is the part the image can carry.** It is enforced by GNOME's own
  settings, with no agent and no runtime policy engine.

## What it does not show

A production kiosk also needs things this model deliberately does not express:

- **Autologin.** A display manager autologin is runtime configuration, not image
  content, and ADR 0005 keeps it out.
- **A single full-screen application.** Launching one app and hiding the shell is
  a session policy, not a GNOME default this model names.
- **A remote reset or a watchdog.** That is fleet management, which DeskOS does
  not do (ADR 0001).
- **A browser profile that clears state.** The kiosk browser's home page and
  storage policy live under the user's home, which the image does not manage.

The example is therefore the honest image-level half: it composes and renders, and
it says which half is missing instead of pretending a lock is a kiosk.
