# Example: a baseline and a role

A **separate resource root** that composes an organization baseline and a
role on top of DeskOS Core, referencing DeskOS resources by name:

```bash
deskosctl plan ./examples/baseline-and-role --workstation example-devops-rhel10
```

| Resource | Contents |
|---|---|
| `example-baseline` (organization) | fonts, wallpaper, window controls, dock and idle/lock defaults for every workstation of the organization, and the organization root CA trusted through `TrustAnchor` |
| `example-devops` (role) | containers, virtualization, Terraform, kubectl, VS Code, Chrome and the OpenShift CLI |
| `example-devops-rhel10` | the target |
| `example-devops-centos10` | composes the same profiles on the public platform so it can be built without a subscription |

The minimal per-platform examples live in the sibling directories
(`core-centos-stream-10/`, `core-almalinux-10/`, `core-rhel-10/`); this one
shows the *composition* axis (baseline + role) on EL10.

## Known gap: virt-manager

The DevOps role requires `virt-manager`. On both EL10 platforms it is only
available from the CodeReady Linux Builder repository (unsupported on
RHEL 10, disabled by default on CentOS Stream 10), so the platforms list it
under `unavailablePackages` and **the requirement is not met** by these
workstations.

`PackageSet/example-virt-manager` records it; including it makes
composition fail with the platform's reason. `cockpit-machines` is
installed as an additional tool and *is not an equivalent*. See
[Platform capability gaps](../../docs/architecture.md#platform-capability-gaps).

## Status

Composition and rendering are **validated for both targets**. The RHEL
10.2 workstation is built on an entitled factory VM, passes
`bootc container lint` with no build-host subscription state, and its
QCOW2 passes the boot and session checks there, for each head of `main`
(`tests/rhel/factory.py`). An earlier revision of the CentOS Stream 10
workstation was built with Podman; the current one is not built. Neither
target has an E2E test.

`assets/` holds placeholder artwork; see
[`assets/PROVENANCE.md`](assets/PROVENANCE.md).
