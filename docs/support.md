# Support

## Where to ask

- **A bug or a feature**: an issue in this repository. Say what you ran, what
  you expected and what happened; a plan, a log or the digest helps more than a
  description.
- **A question about using or building a workstation**: GitHub Discussions.
- **A vulnerability**: not here. [SECURITY.md](../SECURITY.md) has the private
  channel and what is in scope.

## What support is

Community support, best effort, no SLA and no contract. There is no vendor
lifecycle promise and no support agreement to buy; [compatibility.md](compatibility.md)
states what the project does and does not promise.

## What is not supported here

- **RHEL-derived artifacts.** The RHEL workstation, its ISO and its signing
  key belong to the organization that builds them. Ask that organization.
- **Migrations from an older release.** Before 1.0 there is no supported older
  version to migrate from: install the current image and keep your resources.
- **Hardware beyond the tested model.** The session and boot checks run in a
  VM; a real machine is validated one model at a time, and the record says
  which. See [checks.md](checks.md).
- **An organization's resource root.** Its maintainer decides what it composes
  and what it supports.

## Before you ask

[troubleshooting.md](troubleshooting.md) is organised as symptom → the check
that distinguishes it → the remedy, and [diagnostics.md](diagnostics.md) says
what to collect. Most questions about a machine that will not update or will not
boot are answered there.
