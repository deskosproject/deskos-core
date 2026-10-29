# ADR 0005: Managed baseline boundary

**Status:** accepted

## Context

Modeling every channel by which a workstation gains software would turn DeskOS into a meta package manager.

## Decision

DeskOS manages **machine-wide software an organization requires for any authorized user**, complete before any
user's home directory exists:

- distribution RPMs;
- vendor RPM repositories;
- signed vendor RPM files, pinned by checksum;
- checksum-pinned binaries;
- system Flatpaks.

Personal and project tooling (Homebrew, mise, SDKMAN, language version managers, dotfiles, personal Toolboxes) is
**out of scope**. The exception is `deskos-theme apply --user` (ADR 0013): a person runs it, and the resource model
still does not manage `$HOME`.

## Consequences

The resource model stays small. Users keep full freedom after login; DeskOS neither provides nor prevents
user-level tooling.
