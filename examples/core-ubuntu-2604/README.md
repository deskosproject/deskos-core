# Example: DeskOS Core on Ubuntu 26.04

DeskOS Core on Ubuntu 26.04 LTS, the platform whose vendor publishes no bootc
base: the base is owned by the DeskOS project and pinned by digest in
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

This example does not include the `rhel-family` profile: its workstation is
Core's foundation plus the organization profile, without firewalld, an
RPM-named browser or an RPM application channel.

`example-ubuntu-vendor` declares a signed `AptRepository` and a `PackageSet`
that names it. The build writes the deb822 source and its key under `/etc/apt/`,
and `apt-get update` verifies the signature before the packages install; the
`PackageSet` does not bind a package to one source, so this does not claim which
source each installed package came from. The CI job that builds and boots it is
in `.github/workflows/vm-bootcheck.yml`. Both workstations boot to GNOME Initial
Setup, which `tests/vm/firstbootcheck.py` asserts by asking the guest.

DeskOS implements no ISO path for this platform; what is validated is a disk
written by `bootc install to-disk`; see
[docs/install.md](../../docs/install.md#ubuntu-2604).
