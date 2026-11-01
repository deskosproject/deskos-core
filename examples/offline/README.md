# Example: an offline workstation

An **organization** layer for a site with no network: the applications are RPMs
the image already carries, automatic image updates are off, and an update is a
deliberate act.

```bash
deskosctl plan ./examples/offline \
  --workstation example-offline
```

| Resource | Contents |
|---|---|
| `example-offline` (organization) | the package set, the update policy and the desktop default |
| `example-offline` (PackageSet) | Papers, the text editor and the calculator, all RPMs |
| `example-offline` (UpdatePolicy) | `image.automatic: false`, overriding Core's daily staging |
| `example-offline-desktop` | `software.updates: manual` |

## What it shows

- **Offline is a policy, not a compiler mode.** The image is the same artifact; the
  organization turns off automatic staging and names only content the image
  already carries.
- **RPMs, not Flatpaks.** The browser, the document viewer and the editor are in
  the platform archive. A Flatpak would need a remote at build time and at update
  time, which is exactly what this site does not have.
- **A higher layer can override a foundation scalar.** Core says daily; this
  organization says never automatically, and the plan records which resource won.

## What it does not show

- **An update from removable media.** `bootc upgrade` needs an image reference; a
  site that updates from a USB stick imports the image into a local registry
  first. That is a procedure, not a resource, and [install.md](../../docs/install.md)
  covers the reference.
- **A local mirror or an air-gapped registry.** Both are infrastructure outside the
  compiler; an organization that runs one records it in its own pipeline.
- **Offline activation of a licensed application.** A licence server is runtime
  infrastructure, not image content.
- **A guarantee that nothing reaches the network.** The image carries a browser
  and a network stack; keeping a machine offline is a network control, not a
  property of this Plan.

This example is what the image can say: it stages nothing automatically, and every
application it names is already in the artifact.
