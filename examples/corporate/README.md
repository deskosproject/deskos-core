# Example: a corporate workstation

An **organization** baseline for an office on Fedora 45: Chrome from Google's
repository, Zoom, Slack and 1Password from Flathub, and Tailscale as the VPN.

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
| `example-corporate-vpn` (PackageSet) | `tailscale` from Tailscale's RPM repository |
| `example-corporate-apps` (FlatpakSet) | Zoom, Slack and 1Password from Flathub |
| `example-corporate-desktop` | Chrome as the browser, Papers for PDFs, and the corporate favorites |

## What it shows

- **A vendor RPM repository with a local signing key.** Chrome is not a Flatpak;
  the organization that builds the image accepts Google's terms.
- **A public VPN repository.** Tailscale is the example; the same shape works for
  a vendor that publishes an RPM repository.
- **Userland applications as Flatpaks.** Zoom, Slack and 1Password arrive from
  Flathub at first boot, the way a managed desktop installs its apps.
- **A desktop default, not a lock.** The browser, the PDF viewer and the
  favorites are defaults; a person can change them.

## What it does not show

- **Core does not build or publish this example.** Chrome, Zoom, Slack and
  1Password are third-party software under their own terms. The image is for the
  organization that builds it, in its own registry.
- **A real corporate VPN.** Tailscale is a public example; a vendor that ships a
  private client needs its own repository and key, and the organization owns that
  decision.
- **Enrollment or SSO.** A domain join, a certificate and a user's credentials are
  runtime state, not image content ([identity](../identity/)).
- **A Kiosk or a locked-down session.** This is an ordinary office desktop; the
  locks belong to a kiosk ([kiosk](../kiosk/)).

`validate` and `render` compile the intent; they do not install Chrome, Zoom,
Slack, 1Password or Tailscale, and CI does not build the image.
