# Example: a corporate workstation

An **organization** baseline for an office on Fedora 45: Chrome from Google's
repository, Zoom, Slack and 1Password from Flathub, and the corporate VPN
through NetworkManager OpenConnect.

```bash
deskosctl validate ./examples/corporate
deskosctl render ./examples/corporate \
  --workstation example-corporate \
  --output ./dist/corporate
```

| Resource | Contents |
|---|---|
| `example-corporate` (organization) | the browser, VPN, Flatpaks and desktop defaults |
| `example-corporate-browser` (PackageSet) | `google-chrome-stable` from Google's RPM repository |
| `example-corporate-vpn` (PackageSet) | `NetworkManager-openconnect` and its GNOME front end, from Fedora |
| `example-corporate-apps` (FlatpakSet) | Zoom, Slack and 1Password from Flathub |
| `example-corporate-desktop` | Chrome as the browser, Papers for PDFs, and the corporate favorites |

## What it shows

- **A vendor RPM repository with a local signing key.** Chrome is not a Flatpak;
  the organization that builds the image accepts Google's terms.
- **A VPN through NetworkManager.** OpenConnect replaces the unmaintained
  `NetworkManager-fortisslvpn` plugin and speaks the Fortinet SSL VPN protocol;
  Fedora 45 carries the plugin and its GNOME front end.
- **Userland applications as Flatpaks.** Zoom, Slack and 1Password arrive from
  Flathub at first boot, the way a managed desktop installs its apps.
- **A desktop default, not a lock.** The browser, the PDF viewer and the
  favorites are defaults; a person can change them.

## What it does not show

- **Core does not build or publish this example.** Chrome, Zoom, Slack and
  1Password are third-party software under their own terms. The image is for the
  organization that builds it, in its own registry.
- **A real corporate VPN.** The gateway, the profile and the credentials are
  runtime state; the image only carries the NetworkManager plugin.
- **Enrollment or SSO.** A domain join, a certificate and a user's credentials are
  runtime state, not image content ([identity](../identity/)).
- **A Kiosk or a locked-down session.** This is an ordinary office desktop; the
  locks belong to a kiosk ([kiosk](../kiosk/)).

`validate` and `render` compile the intent; they do not install Chrome, Zoom,
Slack, 1Password or the VPN plugin, and CI does not build the image.
