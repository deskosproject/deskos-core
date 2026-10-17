# Example: DeskOS Core on Ubuntu 26.04

DeskOS Core on Ubuntu 26.04 LTS, the platform whose base its vendor does not
publish: it is owned by the Project DeskOS and pinned by digest in
`resources/platforms/ubuntu-2604.yaml`
([ADR 0014](../../docs/adr/0014-platform-base-ownership.md)). It is a
**separate resource root** that references the shipped `deskos-core` profile
and the `ubuntu-2604` Platform by name:

```bash
deskosctl plan ./examples/core-ubuntu-2604 \
  --workstation example-core-ubuntu-2604
```

| Resource | Contents |
|---|---|
| `example-core-ubuntu-2604` | the target: `ubuntu-2604` + the `deskos-core` profile |
| `example-ubuntu-vendor` | the same Core plus an organization profile that installs `code` from a declared, signed APT source |

An Ubuntu workstation carries no `rhel-family` profile: it must not ship
firewalld, an RPM-named browser or an RPM application channel.

`example-ubuntu-vendor` is what proves a declared `AptRepository` on this
platform: the build writes the deb822 source and its key under `/etc/apt/`,
`apt-get update` verifies the signature, and the image installs `code`. The CI
job that builds and boots it is in
`.github/workflows/vm-bootcheck.yml`. Both workstations boot to GNOME Initial
Setup, which `tests/vm/firstbootcheck.py` asserts by asking the guest.

There is no bootc base to hand to `bootc-image-builder` on this platform, so an
Ubuntu disk is written by `bootc install to-disk`; see
[docs/install.md](../../docs/install.md#ubuntu-2604).
