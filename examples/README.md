# Examples

Four resource roots, each a thing DeskOS can compile, each showing **one axis**.
The AlmaLinux root carries its own example Platform resource.

| Example | Shows |
|---|---|
| [`centos-stream-10/`](centos-stream-10/) | the minimal Core composition on the public, redistributable platform |
| [`rhel-10/`](rhel-10/) | the same on RHEL 10, which is **never** published (`redistributable: false`, ADR 0004) |
| [`almalinux-10/`](almalinux-10/) | an example platform on AlmaLinux 10, whose vendor base is the vendor's own *experimental* one |
| [`organization-and-role/`](organization-and-role/) | the **composition axis**: an organization baseline and a role, on two platforms (`example-devops-rhel-10`, `example-devops-centos-stream-10`) |

## How the names work

- **A directory is named after its platform**, spelled as its Platform resource
  spells it: `centos-stream-10`, `rhel-10`, `almalinux-10`.
- **The workstation inside is `example-<platform>`**; shipped ones are
  `deskos-core-<platform>` ([resources/workstations/](../resources/workstations/)).
- **`organization-and-role/` is the exception**: its name is about composition.

## Running one

```bash
deskosctl validate ./examples/centos-stream-10
deskosctl plan     ./examples/centos-stream-10 --workstation example-centos-stream-10
deskosctl render   ./examples/centos-stream-10 --workstation example-centos-stream-10 --output ./dist/example
```

`make check` validates every example and CI renders them. The weekly
[`examples-weekly.yml`](../.github/workflows/examples-weekly.yml) builds and
boots the AlmaLinux and Fedora examples without publishing. See
[docs/platforms.md](../docs/platforms.md) and
[docs/example-platforms.md](../docs/example-platforms.md).
