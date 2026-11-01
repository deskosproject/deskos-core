---
slug: /
---

# What DeskOS is

DeskOS builds a **Linux workstation** as an operating-system image. You describe
the machine in YAML, DeskOS compiles that description into an image, and the
machine runs that image.

It is **image mode**: the operating system is a bootc image, a standard OCI
container image that boots. The running system is not mutated package by
package. An update is a new image deployed beside the running one; a reboot
switches to it, and a rollback switches back.

The description is **declarative**: resources list packages, desktop settings,
branding and update policy. DeskOS validates them, composes the layers into a
typed Plan and renders one build context. The same inputs produce the same
context, with no model and no network during compilation.

It is built to repeat: one definition can produce the same workstation for a
team, an organization, a public-sector deployment or one person's machine. There
is no controller, inventory or agent. Each machine moves forward by updating to
another image, and the image carries the records of what was built, scanned and
booted.

DeskOS is **not a distribution**. It starts from a vendor's official base,
pinned by digest, and composes on top. The supported platforms are CentOS
Stream 10, RHEL 10, Fedora 45 and AlmaLinux 10. RHEL images are never published
by Core; an organization builds them with its own subscription.

## Start here

- [Quick start](quickstart.md): build an image and boot it.
- [Platforms](platforms.md): what each base supports today.
- [Architecture](architecture.md): the path from YAML to a bootable image.

## How it works

DeskOS compiles a workstation from declared intent into a bootable image.

You describe the machine (applications, desktop settings, branding, update
policy) in declarative resources. DeskOS validates them, composes the layers into
a typed Plan, and renders one build context. That context becomes a bootc image,
and the image a disk or an installer. The same inputs give the same Plan and the
same context; no model and no network take part.

## What that means

- **You deploy an image, not a controller.** The result is an image and a disk.
  There is no agent and no reconciliation loop. A machine updates by moving to
  another image.
- **The base belongs to the vendor.** Each platform starts from its vendor's
  official base image, pinned by digest. DeskOS adds its layers on top; it does
  not fork the distribution or replace its packages.
- **Profiles combine by rule.** Foundation, organization, role and workstation
  each contribute. Lists are combined, a key defined twice must agree, and a
  single value comes from the highest layer. The order of files does not change
  the result.
- **Every published image carries evidence.** The digest comes with what was
  built, scanned and booted, and with what was not measured. A claim without a
  digest behind it is not proof.

## Who it is for

- Someone who wants a workstation they can rebuild, not a machine whose history
  exists only on its disk.
- An organization that needs its own packages, settings and branding without
  copying or forking Core.
- A developer who wants to follow the whole path from YAML to a bootable image.

## What DeskOS does not do

These are deliberate exclusions. Changing one needs a decision, not a patch.

- **No fleet management.** No controller, inventory, policy server, agent or
  reconciliation loop, on the machine or off it.
- **No runtime enforcement.** The image sets defaults; it does not lock a running
  system. Locks are an organization's choice, never Core's.
- **No escape hatches.** No scripts, free-form commands, post-install hooks,
  templating, raw kernel arguments or Plymouth scripts. Extend the model instead.
- **No management of a user's home.** Personal tooling, dotfiles and application
  state belong to the user, not to the image.
- **No AI in compilation.** No kind, provider, Plan or backend depends on a
  model, and nothing in the compiler phones home.
- **It is not a general-purpose distribution.** DeskOS is not a distro, and it
  promises no hardware compatibility beyond the models a field validation
  covered.
- **No support contract.** Each platform states what was verified; a platform
  that was never booted says so.

## How this project works

- The digest and the checks are published, including what failed or was not
  measured.
- The model stays small; a new feature needs a reason.
- Automation can propose; a person approves.
