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

## Mission

DeskOS compiles a workstation from declared intent into a bootable image.

You describe the machine (applications, desktop settings, branding, update
policy) in declarative resources. DeskOS validates them, composes the layers into
a typed Plan, and renders one build context. That context becomes a bootc image,
and the image a disk or an installer. The same inputs give the same Plan and the
same context; no model and no network take part.

## What follows from that

- **The artifact is the product.** An image and a disk, not a controller, an
  agent or a reconciliation loop; a machine updates by moving to another image.
- **The vendor owns the base.** Each platform starts from its vendor's official
  base image, pinned by digest. DeskOS composes on top; it does not fork the
  distribution or replace its packages.
- **Layers, not last writer wins.** Foundation, organization, role and
  workstation each contribute. Sets union, keyed definitions must agree, and a
  scalar takes the highest layer. File order never changes the output.
- **The record is checkable.** Every published digest carries records: what was
  built, what was scanned, what booted, what a session proved, and what was not
  measured. A claim with no digest behind it is not proof.

## Who it is for

- A person who wants a workstation they can rebuild, not a machine whose history
  exists only on its disk.
- An organization that needs its own packages, settings and branding without
  copying or forking Core.
- A developer who wants to read the whole path from YAML to a bootable image.

## Non-goals

A proposal that needs one of these needs a decision first, not a patch:

- **Fleet management.** No controller, inventory, policy server, agent or
  reconciliation loop, on the machine or off it.
- **Runtime enforcement.** The image states defaults; it does not lock a running
  system. Locks are an organization's choice, never Core's.
- **Escape hatches.** No scripts, free-form commands, post-install hooks,
  templating, raw kernel arguments or Plymouth scripts; extend the model instead.
- **Managing a user's home.** Personal tooling, dotfiles and application state
  belong to the user, not to the image.
- **AI in compilation.** No kind, provider, Plan or backend depends on a model,
  and nothing in the compiler phones home.
- **A general-purpose distribution.** DeskOS is not a distro, and it promises no
  hardware compatibility beyond the models a field validation covered.
- **A support contract.** Each platform states what was verified; a platform
  that was never booted says so.

## Values

- **Results over claims.** Publish the digest and the checks, gaps included.
- **Narrow scope over feature count.** Saying no keeps the model small.
- **Human approval over automation.** Automation proposes; a person publishes.
