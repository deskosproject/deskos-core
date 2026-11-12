# Examples

The resource roots under [examples/](../examples/) are compile-and-render
examples. They are not shipped platform support, and they are not built as
images: `checks.yml` validates them, and each README says which use case it
shows and which half of that use case the model cannot express.

The exception is `examples/layers/`, which `image-bootcheck.yml` builds and
boots because it exercises four declared RPM repositories.

The shipped platforms, their verification and their publication are in
[Platforms](platforms.md). An example can reference their profiles by name; it
does not copy a platform or promise support for one.

| Example | Axis |
|---|---|
| [`centos-stream-10/`](../examples/centos-stream-10/) | the minimal Core composition on the public platform |
| [`rhel-10/`](../examples/rhel-10/) | the same on RHEL 10, which is never published |
| [`vanilla-gnome/`](../examples/vanilla-gnome/) | the desktop axis: Core without the dock |
| [`deskos-blue/`](../examples/deskos-blue/) | a Flatpak-first Fedora desktop |
| [`layers/`](../examples/layers/) | the composition axis: an organization baseline and a role |
| [`developer/`](../examples/developer/) | a role with the compiler and container toolchain |
| [`office/`](../examples/office/) | a role with documents, a PDF viewer and the serif families |
| [`kiosk/`](../examples/kiosk/) | a locked-down public session |
| [`identity/`](../examples/identity/) | the image-level half of a domain join |
| [`offline/`](../examples/offline/) | a site with no network and no automatic updates |
| [`corporate/`](../examples/corporate/) | a vendor browser, meetings, chat, passwords and a VPN |

Each README follows the same shape: what it shows, what it does not show, and
which part is runtime work rather than image content. An example that cannot
honestly express a use case says so instead of inventing a field (ADR 0005).

A workstation in an example is named `example-<use case>`; the two in `layers/`
carry a platform suffix because that example composes the same profiles on two
platforms. Shipped workstations are `deskos-core-<platform>`
([resources/workstations/](../resources/workstations/)).
