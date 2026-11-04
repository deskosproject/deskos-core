# Browsers: RPM or Flatpak

Question: for a browser on a managed fleet, is the vendor's RPM or a Flatpak the
better channel?

The vendor's RPM, when the vendor publishes and supports one: it is the channel
that carries enterprise policy, native messaging and smartcards. A Flatpak is the
right answer when the vendor publishes it and the fleet can authenticate to the
remote, or when isolating the browser matters more than integrating it.

## What each channel carries

| | Vendor RPM | Flatpak |
|---|---|---|
| Who supports it | The vendor, through its signed repository | The vendor if the package is verified; otherwise a third party |
| Updates | Through the image or the package manager, the same version fleet-wide | Independent of the image, on the packager's schedule |
| Enterprise policy | A file in `/etc`, baked into the image | Inside the sandbox, delivered by the package or a wrapper |
| Native messaging (password managers, security agents) | Host binaries, direct | Needs filesystem permissions; often breaks |
| Smartcards and client certificates | PC/SC and the host drivers, direct | Needs the socket and libraries inside the sandbox |
| Kerberos SSO | The host's ticket cache | Its own environment; may not see the ticket |
| System proxy, printing, downloads | Direct | Through portals, with more permissions and dialogs |
| Video decode | The host stack | Depends on the runtime |
| Isolation from the user's files | None | The sandbox |
| Trust chain | The vendor's signature | Whoever packaged it |

## Chrome has no official Flatpak

Flathub carries `com.google.Chrome`, and its own page says:

> This community-provided package is not verified by, affiliated with, or
> supported by Google.

The community package downloads Google's binary and wraps it. Google publishes
and supports the RPM repository instead. For a fleet that needs policies,
certificates or security agents, the Flatpak trades vendor support for a sandbox
and independent updates, and adds integration work.

## Firefox has both, and the difference is entitlement

Red Hat ships Firefox and Thunderbird as Flatpaks from `flatpaks.redhat.io`
since RHEL 10.2, and keeps the RPMs in AppStream. The Flatpak needs an active
subscription on the installed system to reach the registry; the RPM needs only
the subscription that built the image.

On a fleet that does not register its machines, the RPM is the channel that
works, and it is the one the vendor keeps for exactly that case.

## What this rejects

- **"Flatpak always wins because it is sandboxed."** Isolation is real, and it is
  the reason to choose a Flatpak when the vendor publishes one and the fleet can
  authenticate. It does not replace policy, native messaging or smartcards.
- **"RPM always wins because it is native."** A Flatpak that the vendor publishes,
  verifies and updates is a better channel than an RPM that ships with the image
  and lags behind.

## What is unverified

- Whether Chrome's enterprise policies can be delivered to the Flathub package at
  fleet scale, and whether smartcard authentication works inside it. Not measured
  here; an organization that needs either should test before choosing the Flatpak.
- Whether a Flatpak's independent updates can be pinned to the same version across
  a fleet without running its own mirror.

The rule this page supports: **choose the channel the vendor supports, then check
that the fleet can authenticate to it.** For Chrome that is the RPM. For Firefox
it is the RPM on a fleet without runtime entitlement, and the Red Hat Flatpak on a
registered one.
