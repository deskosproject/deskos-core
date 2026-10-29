# Where an upstream finding goes

DeskOS measures things on purpose: a boot check, a session assertion, a scan, a
re-pin. When the cause is upstream, the finding belongs to that project, with the
measurement attached.

## Rules

- A person files it. An agent may draft the report; a human sends it.
- Attach the measurement: run, digest, check and output. "It does not work" is not
  a report.
- One finding, one venue. If two projects are involved, say which part is whose.
- Our own bugs stay here. The GDM login logo was ours: the plan wrote the key to
  the `distro` database and GDM reads `system-db:gdm` (#247).

## The venues

| Finding | Venue | What we can attach | Filed |
|---|---|---|---|
| bootc install and first boot | [`bootc-dev/bootc`](https://github.com/bootc-dev/bootc) | the first-boot check and its summary; the composefs finding that `/run/ostree-booted` is absent on a bootc host (#177) | no |
| Fedora image mode | [Fedora bootc docs](https://docs.fedoraproject.org/en-US/bootc/) | the Fedora example build and its weekly check; a released-compose re-pin | no |
| AlmaLinux atomic desktop | [`AlmaLinux/atomic-desktop`](https://github.com/AlmaLinux/atomic-desktop) | the AlmaLinux example build and boot in `examples-weekly.yml` | no |
| CentOS Stream and alternative images | the Alternative Images SIG in the [CentOS GitLab group](https://gitlab.com/CentOS) | a base re-review and its scan; a package only CRB carries, so a composed image cannot install it (`virt-manager` on stream10) | no |
| GNOME and the login screen | [`gitlab.gnome.org/GNOME/gdm`](https://gitlab.gnome.org/GNOME/gdm) | a greeter behaviour reproduced in the VM, with the digest and the database it read | no |
| Flatpak preinstall | the Flatpak tracker | the preinstall unit the plan emits and what first boot observed | no |

Nothing in the table has been filed. That is the state, not a plan.

## What the measurements support

- **A host without `/run/ostree-booted`.** Detection for a bootc host that is not
  ostree, with the digest and the `bootc status --json` output that identifies it.
- **First-boot setup suppressed by an existing account.** The Ubuntu root cause, GDM
  counting users through AccountsService and a uid-1000 account hiding the wizard,
  is in [research-notes.md](research-notes.md) with the reproduction.
- **A base whose only copy of a tool is in CRB.** What an image-based workstation
  should do when a requirement lives in an unsupported repository; our answer is
  that the organization decides.

## Not a promise

This page is not a roadmap of upstream work and does not promise to file anything.
A finding nobody reports is still a finding; this page keeps it where a report
would start.
