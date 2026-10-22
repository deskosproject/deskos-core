# Examples

Seven resource roots, each one a thing DeskOS can be asked to compile. None is a template
to copy wholesale: each shows **one axis**, and all of them reference the profiles this
repository ships *by name*, so they compose against the same Core a flavor would. The
AlmaLinux and Fedora roots carry their own Platform resources: they are parked examples,
not shipped platforms.

| Example | Shows |
|---|---|
| [`centos-stream-10/`](centos-stream-10/) | the minimal Core composition on the public, redistributable platform |
| [`rhel-10/`](rhel-10/) | the same on RHEL 10, which is **never** published (`redistributable: false`, ADR 0004) |
| [`almalinux-10/`](almalinux-10/) | a parked example on AlmaLinux 10, whose vendor base is the vendor's own *experimental* one |
| [`fedora-45/`](fedora-45/) | a parked example on Fedora 45 (Silverblue), a base that carries no RHEL packages |
| [`organization-and-role/`](organization-and-role/) | the **composition axis**: an organization baseline and a role, on two platforms (`example-devops-rhel-10`, `example-devops-centos-stream-10`) |

## How the names work

- **A directory is named after its platform**, spelled the way its Platform resource spells
  it. One spelling per platform, everywhere: `centos-stream-10`, `rhel-10`, and the parked
  `almalinux-10` and `fedora-45` examples.
- **The workstation inside is `example-<platform>`**, and a variant says what it adds
  (`deskos-ubuntu-org-packages` is the one in the Ubuntu repository). The workstations this repository *ships* are
  `deskos-core-<platform>` instead: those are the ones a reader is meant to take
  ([resources/workstations/](../resources/workstations/)).
- **`organization-and-role/` is the exception**, and keeps a name about composition because it
  is not about one platform.

## Running one

```bash
deskosctl validate ./examples/centos-stream-10
deskosctl plan     ./examples/centos-stream-10 --workstation example-centos-stream-10
deskosctl render   ./examples/centos-stream-10 --workstation example-centos-stream-10 --output ./dist/example
```

`make check` validates every example and CI renders them. The weekly
[`examples-weekly.yml`](../.github/workflows/examples-weekly.yml) builds and boots the
parked AlmaLinux and Fedora examples without publishing. The CentOS example is the one
the PR gate builds on a VM. See [docs/platforms.md](../docs/platforms.md) for the shipped
platforms and [docs/parked-platforms.md](../docs/parked-platforms.md) for the examples.
