# ADR 0026: the families are the workstation's choice, not the platform's

**Status:** accepted (2026-10-30)

## Context

GNOME has shipped its own families since 47: **Adwaita Sans**, a variation of
Inter, and **Adwaita Mono**, Iosevka matched to it, both OFL. The
[HIG's typography page](https://developer.gnome.org/hig/guidelines/typography.html)
states it as the default an app should expect: "In GNOME, the default font is
Adwaita Sans, a custom built variant of Inter". GNOME OS builds them
(`elements/sdk/adwaita-fonts.bst`) and `gsettings-desktop-schemas` defaults to
`Adwaita Sans 11`, `Adwaita Sans 12` for documents and `Adwaita Mono 11`
(measured 2026-10-30 on the upstream tree). EL10 does not package them: its schema
defaults are still Cantarell 11 and Source Code Pro 10, and RHEL 10 overrides both
to **Red Hat Text Regular 11** and **Red Hat Mono Regular 10**
(`org.gnome.desktop.interface.rhel.gschema.override`, in
`gsettings-desktop-schemas-47.1-4.el10`).

So a DeskOS image inherits whichever family its platform happens to name, and on
EL10 that is a family a third party chose for its own brand. Two questions follow:
whose families does a DeskOS workstation draw, and where is that decided?

## Decision

**The workstation decides, and the platform keeps its own when nobody says
otherwise.**

- `Profile/deskos-adwaita` carries GNOME's families as pinned `BinaryArtifact`s
  (six files and the licence, from a commit of `GNOME/adwaita-fonts`) and the
  `GnomeProfile` that names them: interface `Adwaita Sans 11`, document
  `Adwaita Sans 12`, monospace `Adwaita Mono 11`.
- A workstation that draws GNOME's look adds that profile. The Core workstation on
  CentOS Stream 10 does, and so does the example organization's CentOS Stream 10
  workstation.
- A workstation on a vendor desktop does not, and inherits the vendor's families.
  The example organization names no font at all, so on RHEL 10 it keeps Red Hat
  Text and Red Hat Mono.
- The terminal is not named. Ptyxis uses the system monospace font by default, so
  it follows whichever family the workstation chose; a role that wants a different
  family sets `appearance.fonts.terminal` (ADR 0025).

`deskos-gnome` keeps the families the platform does not package and that every
workstation needs (Liberation, Carlito, Caladea, DejaVu, Noto, the URW base 35,
Gelasio and Overpass); it no longer carries GNOME's own, because those belong to a
look rather than to the desktop machinery.

## Consequences

- The default is a decision with an owner and a diff: a workstation lists
  `deskos-adwaita` or it does not, and the plan's provenance says which resource
  named each family.
- DeskOS no longer inherits RHEL's brand on the platform where it draws its own
  look, and it no longer overrides it where the platform's desktop is the
  platform's own (ADR 0016).
- The families travel as pinned artifacts because EL10 packages neither, and the
  licence travels with them (ADR 0018 as amended).
- Adwaita Sans has two static files (regular and italic); its weights are named
  instances of the variable font, and GNOME relies on that, so a bold title is
  synthesised the way GNOME synthesises it.
- A session check asserts that every family a profile names is installed, so a
  workstation that adds the profile without the artifacts fails on a boot instead
  of falling back (the `font family ... is installed` checks).
