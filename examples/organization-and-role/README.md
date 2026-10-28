# Example: a baseline and a role

A **separate resource root** that composes an organization baseline and a
role on top of DeskOS Core, referencing DeskOS resources by name:

```bash
deskosctl plan ./examples/organization-and-role --workstation example-devops-rhel-10
```

| Resource | Contents |
|---|---|
| `example-organization` (organization) | fonts, wallpaper, window controls, dock and idle/lock defaults for every workstation of the organization, and the organization root CA trusted through `TrustAnchor` |
| `example-devops` (role) | containers, virtualization, Terraform, kubectl, VS Code, Chrome and the OpenShift CLI |
| `example-devops-rhel-10` | the target |
| `example-devops-centos-stream-10` | composes the same profiles on the public platform so it can be built without a subscription |

For a pinned, end-to-end exercise — compiler, Plan and provenance, declared
SBOM, customization, build, UEFI QCOW2, first user, update and rollback — follow
[WALKTHROUGH.md](WALKTHROUGH.md).

The minimal per-platform examples live in the sibling directories
(`centos-stream-10/`, `almalinux-10/`, `rhel-10/`,
`ubuntu-2604/`); this one
shows the *composition* axis (baseline + role) on EL10.

## A desktop tool the platform does not supply

`example-virtualization` installs what both EL10 platforms carry from a supported
repository (`qemu-kvm`, `libvirt`, `virt-install`, `virt-viewer`, Cockpit and its
machines plugin). An organization that wants a desktop application the platform
does not ship — a graphical VM manager, for instance — adds it where it belongs:
a Flatpak from Flathub, or a repository the organization enables and trusts.
Core does not enable an unsupported repository to make an example look complete,
and a package the platform cannot supply fails composition with the platform's
reason ([platform capability gaps](../../docs/architecture.md#platform-capability-gaps)).

## Status

Composition and rendering are **validated for both targets**. The RHEL
10.2 workstation is built on an entitled factory VM, passes
`bootc container lint` with no build-host subscription state, and its
QCOW2 passes the boot and session checks there (retired factory host); this
repository no longer builds RHEL. The CentOS Stream 10 workstation is built
and booted in CI (`.github/workflows/vm-bootcheck.yml`): its boot, session
and first-boot checks pass on an image built with its four declared RPM
sources enabled and the requested packages installed.

`assets/` holds placeholder artwork; see
[`assets/PROVENANCE.md`](assets/PROVENANCE.md).
