# ADR 0005: Managed baseline boundary

Status: accepted

## Context

A workstation can gain software through many channels. Modeling all of
them would turn DeskOS into a meta package manager.

## Decision

DeskOS manages machine-wide software an organization requires for any
authorized user, complete before any user's home directory exists:
distribution RPMs, vendor RPM repositories, checksum-pinned binaries and
system Flatpaks. Personal and project tooling (Homebrew, mise, SDKMAN,
language version managers, dotfiles, personal Toolboxes) is out of scope.

## Consequences

The resource model stays small. Users keep full freedom after login;
DeskOS neither provides nor prevents user-level tooling.
