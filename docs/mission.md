---
slug: /
---

# What is DeskOS?

DeskOS compiles a Linux workstation. You describe the machine in YAML, the
compiler writes a Containerfile, and the machine runs the image built from it.

The image is bootc: an update replaces the running image, and the previous one
stays one reboot away.

Nothing runs as a service on the machine: no agent, no controller, no
reconciliation loop.

## How it works

1. **Declare.** Write YAML in four layers: `foundation`, `organization`, `role`
   and `workstation`. A layer names resources, never commands.
2. **Compile.** `deskosctl render` validates every resource, merges the layers
   into a typed Plan and emits a deterministic `Containerfile`. It runs offline.
3. **Build.** `podman` builds the bootable OCI image on the vendor's bootc base,
   pinned by digest.
4. **Boot.** Write the image to a disk with `bootc-image-builder`, or boot a
   published installer.

```bash
deskosctl render --workstation deskos-core-centos-stream-10 --output ./dist
sudo podman build -t registry.example.com/deskos/core:stable ./dist
```

## Start here

- **Build and boot a workstation.** [Quick start](quickstart.md)
- **Check what each base supports.** [Platforms](platforms.md): CentOS Stream
  10, RHEL 10, AlmaLinux 10 and Fedora 45.
- **Follow YAML to a bootable image.** [Architecture](architecture.md)
- **Build RHEL 10 privately.** Core never publishes it; an organization uses its
  own subscription.

## Rules that do not move

- **The base belongs to the vendor.** Each platform starts from its official
  bootc base. DeskOS adds layers; it does not fork the distribution.
- **Layers combine by rule.** Foundation, organization, role and workstation each
  contribute. Lists union, a key defined twice must agree, and a scalar comes
  from the highest layer.
- **No escape hatches.** No scripts, free-form commands, post-install hooks,
  templating, raw kernel arguments or Plymouth scripts.
- **No fleet or runtime enforcement.** The image sets defaults; it does not lock
  a running system. Locks are an organization's choice, never Core's.
- **No AI in compiler semantics.** No kind, provider, Plan or backend depends on
  a model, and nothing phones home.
- **Automation proposes; a person approves.** Generated changes are reviewed like
  any other patch.

## Who it is for

Someone who wants a workstation they can rebuild, and an organization that needs
its own packages and branding without forking Core.
