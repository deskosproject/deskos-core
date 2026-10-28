# Where an upstream finding goes

DeskOS measures things on purpose: a boot check, a session assertion, a scan, a
re-pin. When the cause is upstream, the finding belongs to that project, with the
measurement attached. This page says where each kind goes and what we actually
have. It announces no integration and files nothing by itself.

## Rules

- **A person files it.** An agent may draft the report; a human sends it.
- **Attach the measurement**: the run, the digest, the check and the output that
  shows it. "It does not work" is not a report.
- **One finding, one venue.** If two projects are involved, say which part is
  whose.
- **Our own bugs stay here.** The GDM login logo was ours: the plan wrote the key
  to the `distro` database and GDM reads `system-db:gdm` (#247). That is a fix in
  this repository, not a GDM issue.

## The venues, checked

| Finding we have | Venue | Evidence we can attach | Filed |
|---|---|---|---|
| bootc install and first-boot behaviour | [`bootc-dev/bootc`](https://github.com/bootc-dev/bootc) issues | the first-boot check and its summary; the composefs finding that `/run/ostree-booted` is absent on a bootc host (#177) | no |
| Fedora image mode and its bootc images | [`gitlab.com/fedora/bootc`](https://gitlab.com/fedora/bootc), docs at [docs.fedoraproject.org/en-US/bootc](https://docs.fedoraproject.org/en-US/bootc/) | the Fedora 45 example build and its weekly check; the released-compose re-pin | no |
| AlmaLinux atomic desktop | [`AlmaLinux/atomic-desktop`](https://github.com/AlmaLinux/atomic-desktop) issues | the AlmaLinux example build and boot in `examples-weekly.yml` | no |
| CentOS Stream base and alternative images | the Alternative Images SIG in the [CentOS GitLab group](https://gitlab.com/CentOS) | the base pin re-review and its scan; a package that only CRB carries, so a composed image cannot install it (`virt-manager` on stream10) | no |
| GNOME and the login screen | [`gitlab.gnome.org/GNOME/gdm`](https://gitlab.gnome.org/GNOME/gdm) | a greeter behaviour reproduced in the VM with the image digest and the dconf database it read | no |
| Flatpak preinstall and first-boot apps | the Flatpak project's tracker | the preinstall unit the Plan emits and what the first-boot check observed (installed, or not) | no |

Nothing in the table has been filed. That is the state, not a plan: the column
exists so nobody assumes a report was sent because the evidence exists.

## Topics the measurements support

Each of these comes from a run in this repository, not from a hypothesis:

- **A host without `/run/ostree-booted`.** Documented detection for a bootc host
  that is not ostree, with the image digest and the `bootc status --json` output
  that identifies it.
- **First-boot setup suppressed by an existing account.** The Ubuntu root cause —
  GDM counting users through AccountsService, and a uid-1000 account suppressing
  the wizard — is written up in [research-notes.md](research-notes.md) with the
  reproduction.
- **A base whose only copy of a desktop tool is in CRB.** What an image-based
  workstation is supposed to do when a requirement lives in an unsupported
  repository; our answer is "the organization decides", and the report would ask
  the distribution rather than guess.

## What this page is not

Not a roadmap of upstream work, and not a promise to file anything. A finding
that nobody reports is still a finding; this page keeps the evidence where the
report would start.
