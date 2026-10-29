# Example: DeskOS Core on Fedora 45

Minimal Core on Fedora 45, on the Fedora Project's bootc base Silverblue: a
**self-contained root**, `fedora-45` in [`platforms/`](platforms/).

```bash
deskosctl plan ./examples/fedora-45 \
  --workstation example-fedora-45
```

| Resource | Contents |
|---|---|
| `fedora-45` (platform) | `platforms/fedora-45.yaml`: Fedora facts on `quay.io/fedora/fedora-silverblue:45` |
| `example-fedora-45` | the target: `fedora-45` + the `deskos-core` and `deskos-gnome` profiles |

Fedora 45 is **not a shipped platform**: Silverblue's `:45` tag moves daily and
its old digest is garbage-collected upstream, so a published Fedora platform
needs a mirrored base. Fedora is the path to the newest GNOME; it composes and
renders in CI, and a weekly job builds and boots it without publishing.

The example uses Silverblue, not the minimal `quay.io/fedora/fedora-bootc` base:
Silverblue carries GNOME, GDM, Flatpak, Plymouth and the vendor desktop. Software
choices are in `Profile/fedora`; the example keeps Fedora's own wallpaper and
splash.

To build it locally, including a base digest refresh and a QCOW2, follow
[docs/example-platforms.md](../../docs/example-platforms.md); do not publish it.
