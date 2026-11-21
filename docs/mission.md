---
slug: /

---

# What is DeskOS?

DeskOS builds a Linux workstation as an operating-system image. You describe the
machine in YAML, DeskOS compiles it, and the machine runs that image.

The image is bootc: an OCI image that boots. An update deploys a new image beside
the running one; a reboot switches to it, a rollback switches back. The machine
is described in YAML and compiled, not scripted in place.

DeskOS is an image compiler, not a distribution. Each platform keeps its
vendor's bootc base and DeskOS adds layers without forking it. One definition
can serve one person, a team or an organization, and the result is evidence
bound to a digest.

No part of DeskOS runs as a service on the machine: no agent, no controller, no
reconciliation loop. It is not a fleet manager, a policy server or a support
contract.

The supported platforms are CentOS Stream 10, RHEL 10, AlmaLinux 10 and Fedora
45 ([Platforms](platforms.md)). Core never publishes RHEL images: an organization
builds them with its own subscription.

## How it works

1. **Declare.** Write YAML in four layers: `foundation`, `organization`, `role`
   and `workstation`. A layer names resources: packages, desktop defaults,
   repositories, trust anchors, update policy. There is no shell and no hook.
2. **Compile.** Run `deskosctl render` offline. It validates every resource,
   merges the layers into a typed Plan and emits a deterministic
   `Containerfile`.
3. **Build.** Build the bootable OCI image with `podman`, on the vendor's bootc
   base pinned by digest.
4. **Boot.** Write the image to a disk with `bootc-image-builder`, or boot a
   published installer, and start it.

```bash
deskosctl render --workstation deskos-core-centos-stream-10 --output ./dist
sudo podman build -t registry.example.com/deskos/core:stable ./dist
```

## Start here

| I want to | Read |
|---|---|
| build an image and boot it | [Quick start](quickstart.md) |
| know what each base supports | [Platforms](platforms.md) |
| see the path from YAML to a bootable image | [Architecture](architecture.md) |

## Rules that do not move

- **The base belongs to the vendor.** Each platform starts from its official
  bootc base. DeskOS adds layers; it does not fork the distribution.
- **Layers combine by rule.** Foundation, organization, role and workstation each
  contribute. Lists union, a key defined twice must agree, and a scalar comes from
  the highest layer. File order never changes the result.
- **No escape hatches.** No scripts, free-form commands, post-install hooks,
  templating, raw kernel arguments or Plymouth scripts. Extend the model instead.
- **No fleet or runtime enforcement.** The image sets defaults; it does not lock a
  running system. Locks are an organization's choice, never Core's.
- **No AI in compiler semantics.** No kind, provider, Plan or backend depends on a
  model, and nothing phones home.
- **Automation proposes; a person approves.** Generated changes are reviewed like
  any other patch.

## Who it is for

Someone who wants a workstation they can rebuild, an organization that needs its
own packages and branding without forking Core, and a developer who wants to
follow the whole path from YAML to a bootable image.
