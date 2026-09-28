# Example organization

An example organization overlay. It is a separate resource root that
references DeskOS resources by name:

    deskosctl plan ./resources ./examples/example-org --workstation example-devops-rhel10

- `example-baseline` (organization): fonts, wallpaper, window controls,
  dock and idle/lock defaults for every workstation of the organization.
- `example-devops` (role): containers, virtualization, Terraform, kubectl,
  VS Code, Chrome and the OpenShift CLI.
- `example-devops-rhel10` is the target; `example-devops-centos10` composes
  the same profiles on the public platform so it can be built without a
  subscription.

## Known gap: virt-manager

The DevOps role requires `virt-manager`. On both EL10 platforms it is only
available from the CodeReady Linux Builder repository (unsupported on
RHEL 10, disabled by default on CentOS Stream 10), so the platforms list it
under `unavailablePackages` and the requirement is **not met** by these
workstations. `PackageSet/example-virt-manager` records it; including it
makes composition fail with the platform's reason. `cockpit-machines` is
installed as an additional tool and is not an equivalent.

## Status

Composition and rendering are validated for both targets. Earlier
revisions of both workstations were built: the CentOS Stream 10 one with
Podman, the RHEL 10.2 one on an entitled host, where it passed `bootc
container lint` with no build-host subscription state and its QCOW2 was
booted once, manually. The current revision is not built, and neither
target has an automated boot or E2E test.

`assets/` holds placeholder artwork; see `assets/PROVENANCE.md`.
