# Examples

Seven resource roots, each one a thing DeskOS can be asked to compile. None is a template
to copy wholesale: each shows **one axis**, and all of them reference the profiles and
platforms this repository ships *by name*, so they compose against the same Core a flavor
would.

| Example | Shows |
|---|---|
| [`centos-stream-10/`](centos-stream-10/) | the minimal Core composition on the public, redistributable platform |
| [`rhel-10/`](rhel-10/) | the same on RHEL 10, which is **never** published (`redistributable: false`, ADR 0004) |
| [`almalinux-10/`](almalinux-10/) | the same on AlmaLinux 10, whose vendor base is the vendor's own *experimental* one |
| [`fedora-45/`](fedora-45/) | the same on Fedora 45 (Silverblue), a base that carries no RHEL packages |
| [`organization-and-role/`](organization-and-role/) | the **composition axis**: an organization baseline and a role, on two platforms (`example-devops-rhel-10`, `example-devops-centos-stream-10`) |

## How the names work

- **A directory is named after its platform**, spelled the way
  `resources/platforms/<platform>.yaml` spells it. One spelling per platform, everywhere:
  `centos-stream-10`, `almalinux-10`, `rhel-10`, `fedora-45`.
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

`make check` validates every example. CI validates them too, and builds and **boots**
several of them on real VMs — [docs/platforms.md](../docs/platforms.md) is the table that
says which, with the evidence.
