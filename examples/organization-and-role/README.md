# Example: a baseline and a role

A **separate resource root** that composes an organization baseline and a role
on top of DeskOS Core, referencing DeskOS resources by name:

```bash
deskosctl plan ./examples/organization-and-role --workstation example-devops-rhel-10
```

| Resource | Contents |
|---|---|
| `example-organization` (organization) | fonts, wallpaper, window controls, dock and idle/lock defaults for every workstation of the organization, and the organization root CA trusted through `TrustAnchor` |
| `example-devops` (role) | containers, virtualization, Terraform, kubectl, VS Code, Chrome and the OpenShift CLI |
| `example-devops-rhel-10` | the target |
| `example-devops-centos-stream-10` | composes the same profiles on the public platform so it can be built without a subscription |

For a pinned compiler-to-rollback exercise, follow
[WALKTHROUGH.md](WALKTHROUGH.md).

Minimal examples live in the sibling directories (`centos-stream-10/`,
`almalinux-10/`, `rhel-10/`); this one shows composition.

## A desktop tool the platform does not supply

`example-virtualization` installs the EL10 virtualization stack from a supported
repository (`qemu-kvm`, `libvirt`, `virt-install`, `virt-viewer`, Cockpit and its
machines plugin). An application the platform does not ship belongs in a Flatpak
from Flathub or a trusted organization repository. Core does not enable an
unsupported repository; a missing package fails composition with the platform's
reason ([platform capability gaps](../../docs/architecture.md#platform-capability-gaps)).

## Status

Composition and rendering are **validated for both targets**. The RHEL 10.2
workstation is built on an entitled factory VM, passes `bootc container lint`,
and its QCOW2 passes the boot and session checks there (retired factory host);
this repository no longer builds RHEL. The CentOS Stream 10 workstation is built
and booted in CI (`.github/workflows/vm-bootcheck.yml`), and its boot, session
and first-boot checks pass with its four declared RPM sources enabled.

`assets/` holds placeholder art ([`assets/PROVENANCE.md`](assets/PROVENANCE.md)).
