# Mission

DeskOS compiles a workstation from declared intent into a bootable image.

You describe the machine — its applications, desktop settings, branding and
update policy — in declarative resources. DeskOS validates them, composes the
layers into a typed Plan, and renders one build context. From that context comes
a bootc image, and from the image a disk or an installer. Identical inputs
produce the identical Plan and the identical context; no model and no network
take part in compilation.

## What follows from that

- **The artifact is the product.** An image and a disk, not a controller, an
  agent or a reconciliation loop. A machine runs the image it was built with and
  updates by moving to another one.
- **The vendor owns the base.** Each platform starts from its vendor's official
  base image, pinned by digest. DeskOS composes on top; it does not fork the
  distribution or replace its packages.
- **Layers, not last writer wins.** Foundation, organization, role and
  workstation each contribute; sets union, keyed definitions must agree, and a
  scalar takes the highest layer. File order never changes the output.
- **The record is checkable.** Every published digest carries evidence: what was
  built, what was scanned, what booted, what a session proved, and what was not
  measured. A claim without a digest is not evidence.

## Who it is for

- A person who wants a workstation they can rebuild, not a machine whose history
  only exists on its disk.
- An organization that needs its own packages, settings and branding without
  copying or forking Core.
- A developer who wants to read the whole path from YAML to a bootable image.

## Non-goals

These are deliberate, and a proposal that needs one of them needs a decision
first, not a patch:

- **Fleet management.** No controller, inventory, policy server, agent or
  reconciliation loop, on the machine or off it.
- **Runtime enforcement.** The image states defaults; it does not lock a running
  system. Locks are an organization's explicit choice, never Core's.
- **Escape hatches.** No scripts, free-form commands, post-install hooks,
  templating, raw kernel arguments or Plymouth scripts. If the model cannot say
  it, the model is extended deliberately.
- **Managing a user's home.** Personal tooling, dotfiles and application state
  belong to the user, not to the image.
- **AI in compilation.** No resource kind, provider, Plan or backend depends on
  a model, and nothing in the compiler phones home.
- **A general-purpose distribution.** DeskOS is not a distro, and it does not
  promise hardware compatibility beyond the models a field validation covered.
- **A support contract.** Each platform states what was verified; a platform that
  was not booted says so.

## Values

- **Determinism over convenience.** A build that cannot be repeated is a bug,
  even when it works once.
- **Vendor bases over forks.** Compose on what the vendor supports; do not
  become the maintainer of a distribution by accident.
- **Evidence over claims.** Publish the digest and the checks, including the
  gaps. "Not measured" is an honest result.
- **Narrow scope over feature count.** Saying no to a feature keeps the model
  small enough to reason about.
- **Human approval over automation.** Automation proposes, tests and reports; a
  person decides what is published.
