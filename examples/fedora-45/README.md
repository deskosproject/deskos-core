# Example: DeskOS Core on Fedora 45

The minimal DeskOS Core composition on Fedora 45, on the Fedora Project's own
desktop bootc base, Silverblue. It is a **self-contained resource root**: the
`fedora-45` Platform lives in [`platforms/`](platforms/), next to the example
workstation.

```bash
deskosctl plan ./examples/fedora-45 \
  --workstation example-fedora-45
```

| Resource | Contents |
|---|---|
| `fedora-45` (platform) | `platforms/fedora-45.yaml`: Fedora facts on `quay.io/fedora/fedora-silverblue:45` |
| `example-fedora-45` | the target: `fedora-45` + the `deskos-core` and `deskos-gnome` profiles |

Fedora 45 is **not a shipped platform**. Silverblue's `:45` tag moves daily and
its old digest is garbage-collected upstream, so a published Fedora platform
would need a mirrored base first. The example is kept because Fedora is the path
to the newest GNOME; it composes and renders in CI, and a weekly job builds and
boots the current Fedora base without publishing.

The example deliberately uses Silverblue rather than the minimal
`quay.io/fedora/fedora-bootc` base: Silverblue carries GNOME, GDM, Flatpak,
Plymouth and the vendor desktop, which is what the example tests. The Fedora
family's software choices are in `Profile/fedora`; the example keeps Fedora's
own wallpaper and splash.

To build it locally, including refreshing the base digest and writing a QCOW2,
follow [docs/example-platforms.md](../../docs/example-platforms.md). Do not
publish the resulting image.
