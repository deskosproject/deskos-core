# ADR 0006: BootProfile as the tenth public kind

Status: accepted (approved by Ricardo, 2026-09-28)

## Context

Boot and shutdown appearance (Plymouth splash, quiet boot) is part of the
workstation artifact, but it is not GNOME configuration and not a
Platform fact, and Workstation has no layering. None of the nine v1alpha1
kinds could hold it without distorting its meaning.

## Decision

Add `system.deskos.org/v1alpha1 BootProfile` with layered scalars:
`splash` (`graphical` or `text`), `quiet` (boolean) and `watermark` (a
PNG asset inside the resource root). Platforms supply the kernel-argument
names, Plymouth packages, dracut module and the stock spinner frames. The
compiler lowers them to bootc `kargs.d`, an in-image initramfs rebuild
and, for `watermark`, an image-owned two-step theme `deskos` made the
default Plymouth theme.

BootProfile is artifact boot intent: it changes image content only and
runs nothing on deployed machines. It exposes no arbitrary kernel
arguments, scripts or theme files.

## Consequences

The public API has exactly ten kinds; a test enforces the count. Splash
artwork is a typed asset; there are no script themes and no package-owned
theme files are replaced.
