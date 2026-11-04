---
slug: /

---

# What is DeskOS?

DeskOS builds a Linux workstation as an operating-system image. You describe the
machine in YAML, DeskOS compiles it, and the machine runs that image.

The image is bootc: an OCI image that boots. An update deploys a new image beside
the running one; a reboot switches to it, a rollback switches back. The
description is declarative: resources list packages, desktop settings, branding
and update policy. Compilation is deterministic, with no model and no network.

| DeskOS is | DeskOS is not |
|---|---|
| an image compiler: YAML to Plan to image | a distribution; it starts from the vendor's official base, pinned by digest |
| declarative, deterministic, offline | a controller, agent or reconciliation loop |
| one definition for a team, an organization or one person | a fleet manager, a policy server or a home-directory manager |
| evidence bound to a digest | a support contract or a hardware promise |
The supported platforms are CentOS Stream 10, RHEL 10, Fedora 45 and AlmaLinux

10 ([Platforms](platforms.md)). Core never publishes RHEL images: an organization
builds them with its own subscription.

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
