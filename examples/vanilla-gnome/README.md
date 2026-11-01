# Example: vanilla GNOME on CentOS Stream 10

The Core desktop without Dash to Dock. Every other profile is the reference
composition; the difference is one line: `deskos-dock` is absent.

```bash
deskosctl plan ./examples/vanilla-gnome \
  --workstation example-vanilla-gnome
```

| Resource | Contents |
|---|---|
| `example-vanilla-gnome` | `centos-stream-10` + `deskos-core`, `deskos-gnome`, `rhel-family`, `firefox`; **no `deskos-dock`** |

The dock is a profile, not a compiler default: the platform catalog names the
dash-to-dock artifact, and only a profile that selects `deskos-dock` installs and
enables it (ADR 0017). Without that profile the image has GNOME's own dash, and
none of the dock's dconf keys are written.

`deskos-look`, `deskos-reference-apps` and any organization layer are separate
choices and stay out of this example, so what changes is only the dock.
