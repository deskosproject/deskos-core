# ADR 0004: DeskOS Core is platform-independent intent

**Status:** accepted

## Context

A RHEL workstation built `FROM` a CentOS-derived DeskOS image would still
be CentOS, and could not be supported as RHEL.

## Decision

**DeskOS Core is a set of resources, not an image.** `Platform` resources
hold every platform fact (base image, group mapping, GNOME and Flatpak
integration, redistribution). Each workstation compiles Core directly onto
its platform's official bootc base. RHEL-derived images are **never**
publicly published or redistributed by the project.

## Consequences

Adding a platform means adding a `Platform` resource, not changing the
compiler. Platform differences stay in one reviewable place.
