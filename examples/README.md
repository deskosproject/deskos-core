# Examples

The resource roots here are compile-and-render examples, each showing **one use
case**. They are not shipped platform support, and CI does not build them.

| Example | Shows |
|---|---|
| [`centos-stream-10/`](centos-stream-10/) | the minimal Core composition on the public, redistributable platform |
| [`rhel-10/`](rhel-10/) | the same on RHEL 10, which is **never** published (`redistributable: false`, ADR 0004) |
| [`vanilla-gnome/`](vanilla-gnome/) | the **desktop axis**: the Core desktop with GNOME's own dash and no `deskos-dock` |
| [`deskos-blue/`](deskos-blue/) | a Flatpak-first Fedora desktop, inspired by Bluefin |
| [`layers/`](layers/) | the **composition axis**: an organization baseline and a role, on two platforms |
| [`developer/`](developer/) | a role with the compiler and container toolchain |
| [`office/`](office/) | a role with documents, a PDF viewer and the serif families |
| [`kiosk/`](kiosk/) | a locked-down public session, and what the image cannot lock |
| [`identity/`](identity/) | the image-level half of a domain join |
| [`offline/`](offline/) | a site with no network and no automatic updates |
| [`corporate/`](corporate/) | a vendor browser, meetings, chat, passwords and a VPN |

## How the names work

- **An example is named after the use case**, not after a platform.
- **The workstation inside is `example-<use case>`**; shipped ones are
  `deskos-core-<platform>` ([resources/workstations/](../resources/workstations/)).
- **`layers/` is the exception**: its two workstations carry a platform suffix
  because the point is the same profiles on two platforms.

## Running one

```bash
deskosctl validate ./examples/developer
deskosctl plan \
  ./examples/developer \
  --workstation example-developer
deskosctl render \
  ./examples/developer \
  --workstation example-developer \
  --output ./dist/example
```

`make check` validates every example. CI does not plan or render them, and it
does not build the use-case examples. The one exception is `layers/`:
`image-bootcheck.yml` builds and boots its CentOS workstation to prove the four
RPM repositories it declares. See [docs/platforms.md](../docs/platforms.md) for
the shipped platforms and
[docs/example-platforms.md](../docs/example-platforms.md) for what an example is.
