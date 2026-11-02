# Example: DeskOS Blue

A Flatpak-first Fedora desktop, inspired by Bluefin. The GNOME Shell, GDM,
Nautilus and Settings stay RPMs from the Silverblue base; the applications are
Flatpaks installed at first boot, and VSCode and virt-manager are userland
Flatpaks rather than RPMs.

```bash
deskosctl validate ./examples/deskos-blue
deskosctl render ./examples/deskos-blue \
  --workstation deskos-blue \
  --output ./dist/deskos-blue
```

| Resource | Contents |
|---|---|
| `fedora-45-flatpak` (Platform) | the shipped Fedora 45 pin, with the GNOME applications removed from the RPM set |
| `deskos-blue` (organization) | the VPN, the Flatpak set and the desktop defaults |
| `deskos-blue-apps` (FlatpakSet) | the Bluefin-style application set, including Chrome, VSCode, virt-manager, Podman Desktop and Builder |
| `deskos-blue-desktop` | Firefox, Papers, the developer favorites and blur-my-shell |
| `deskos-blue-vpn` | Tailscale from its RPM repository, the one system daemon |

## What it shows

- **The shell is the platform's; the applications are userland.** The base keeps
  GNOME Shell, GDM, Nautilus and Settings, and the example removes the RPMs for
  the applications that the Flatpak set replaces.
- **A long preinstall list.** `flatpak preinstall` runs at first boot, so the
  image stays small and the applications arrive from Flathub.
- **An extension the shipped catalog does not carry.** `blur-my-shell` is a
  pinned source on the example Platform, enabled by the profile.
- **A userland development desktop.** Chrome, VSCode, virt-manager, Podman
  Desktop, Builder and DevToolbox are Flatpaks. Tailscale is the one system
  daemon; a VPN client has no Flatpak.

## What it does not show

- **Homebrew.** DeskOS has no script or `$HOME` field (ADR 0005); installing
  Homebrew would need a new kind. This example does not pretend to have it.
- **A GNOME Shell in Flatpak.** The base's Shell stays an RPM, as on Bluefin.
- **The full Bluefin desktop list.** The set is the default system and developer
  list; the optional `full-desktop` list is not preinstalled.
- **Bluefin's own tooling.** There is no `ujust`; `deskos` is the runtime command.

The example compiles and renders; it is not built by CI.
