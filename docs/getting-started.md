# Getting started

A five-minute tour: what DeskOS is, the four words it uses, and how to see
it work. If you only read one page, read this one.

## The problem it solves

An organization with more than a handful of Linux desktops ends up with
**snowflakes**: each machine was installed once, by hand, and then drifted.
Nobody can say what is on them, reproduce one, or prove one is safe to
ship.

DeskOS takes the machine definition out of each installer's hands. You
**describe** the workstation in versioned text, and DeskOS **compiles** it
into one image — which it builds, tests, signs and publishes like any other
artifact in your pipeline.

> A workstation is compiled from organizational intent.

## The four words

- **Resource** — a YAML file describing one thing: a package set, a
  repository, a desktop setting, a theme, an update policy. There are
  **thirteen kinds**; each is small and composable. See
  [resources.md](resources.md).
- **Profile** — a named group of resources at a **layer**:
  `foundation` < `organization` < `role` < `workstation`. A higher layer
  overrides a lower one *on purpose*, never by file order.
- **Workstation** — the machine: a platform (CentOS Stream 10 or RHEL 10)
  plus the profiles it is made of. This is the file you author.
- **Plan, then render** — the compiler reads every resource, composes them
  into a typed **plan** (each value keeps the resource and layer it came
  from), and `render` writes a **deterministic** build context: the same
  inputs always give the same bytes.

## See it work

One file to author, and two commands to look before you build:

```yaml
# my-org/workstations/lab.yaml — the machine you want
apiVersion: core.deskos.org/v1alpha1
kind: Workstation
metadata: { name: lab }
spec:
  platformRef: centos-stream-10
  profiles: [deskos-core, example-baseline, example-devops]
```

```bash
deskosctl plan ./resources ./my-org --workstation lab        # read the composed machine
deskosctl render ./resources ./my-org --workstation lab --output ctx
sudo podman build -t localhost/lab ctx
```

The copy-paste version (with the release download and the QCOW2 or ISO
step) is the [**Quickstart**](../README.md#quickstart). The
[example organization](https://github.com/deskosproject/deskos-core/tree/main/examples/baseline-and-role)
is a complete, runnable set of resources to copy from.

## What DeskOS is not

- **Not a distribution.** Organizations compose their own workstations from
  DeskOS Core; there are no editions.
- **Not configuration management.** DeskOS owns the *image*, not the running
  machine. Ansible, Satellite, MDM and EDR keep managing endpoints.
- **Not a personal environment manager.** Dotfiles, Homebrew/mise and
  Toolbox contents stay outside the managed baseline.

## Where to go next

| | |
|---|---|
| [Install deskosctl and get a disk](install.md) | the release, a QCOW2, an ISO, private RHEL 10 builds |
| [Resources](resources.md) | every kind with an example |
| [Architecture](architecture.md) | the pipeline, the design principles, the validation status |
| [Development](development.md) | build `deskosctl`, run the tests, cut a release |
