# Example: an office workstation

A **role** over DeskOS Core: a document suite from Flathub, a PDF viewer from the
platform, the serif families and the favorites an office desk expects.

```bash
deskosctl plan ./examples/office \
  --workstation example-office
```

| Resource | Contents |
|---|---|
| `example-office` (role) | the document role, its packages, its Flatpak and its desktop defaults |
| `example-office` (PackageSet) | `papers`, `gnome-calculator`, Noto Serif and Liberation Serif |
| `example-office-apps` (FlatpakSet) | LibreOffice from Flathub |
| `example-office-desktop` | Papers as the PDF viewer, Gelasio for documents, office favorites |

## What it shows

- **An application the platform does not package is a Flatpak**, not a vendor RPM
  repository. CentOS Stream 10 does not serve LibreOffice; Flathub does, and Core
  already declares the channel and its key.
- **A role can set the document font** without shipping one: Gelasio travels with
  `deskos-gnome`, and the role only names it.
- **The PDF viewer is a default, not a lock.** A person can still open a PDF in a
  browser; the role decides what a double-click does.

## What it does not show

- **Microsoft 365, Teams or OneDrive.** They are not in the archives DeskOS
  supports, and a browser tab is the honest way to reach them.
- **Proprietary fonts.** The image ships families whose licences allow
  redistribution; an organization that owns a licence adds its own
  `BinaryArtifact` (ADR 0013).
- **A shared mailbox or a calendar account.** Those are runtime identity and
  credentials, not image content (ADR 0027).

Swap `platformRef` and the family profile for another platform; the Flatpak and
the role do not change.
