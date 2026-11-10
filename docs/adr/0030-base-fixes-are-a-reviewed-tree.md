# ADR 0030: base fixes ship as a reviewed `system_files` tree

**Status:** accepted (2026-11-06)

## Context

ADR 0029 fixed the unlabeled `/var/home` by adding a `Platform.spec.relabelDirs`
field, a schema, validation and a generated `tmpfiles.d` file. The content is a
static four-line rule that every bootc platform needs. The next fix, fstrim on
the bootc sysroot (#439), is another static drop-in.

A field per fix does not scale, and the content is not an input that varies: it
is a reviewed file. Universal Blue's Bluefin carries exactly this shape as
`system_files/shared/usr/lib/...`, copied into the image at build time.

## Decision

**Static base fixes live in `resources/system_files/`, a reviewed tree that
`deskosctl render` copies into the build context.**

- The tree accepts a closed set of declarative paths:
  - `usr/lib/systemd/system/<unit>.d/<name>.conf`: a drop-in for a unit the
    base already ships;
  - `usr/lib/tmpfiles.d/<name>.conf`: a directory or context rule.
- The renderer rejects every other path, symlinks, non-regular files and
  duplicate destinations. Files are installed with mode `0644`; the tree cannot
  carry a new unit, a script, a binary or an executable.
- The files are copied to `rootfs/<path>` in the context, which the generated
  Containerfile already copies to `/`.
- The release resources archive carries the tree, so a downstream render gets
  the same files from the released Core.
- This is not a Plan field: the files are static and reviewed as files. A fix
  that needs a package, a binary or a computed value still belongs to a typed
  Plan field or a signed package.

`Platform.spec.relabelDirs` is removed. `/var/home` moves to
`resources/system_files/usr/lib/tmpfiles.d/zz-deskos-relabel.conf`, and fstrim
to `resources/system_files/usr/lib/systemd/system/fstrim.service.d/10-sysroot.conf`.

## Consequences

- One mechanism covers both fixes, and the next static one, without new kinds or
  fields.
- The tree is visible in review and in the resources archive; the renderer's
  allowlist keeps it from becoming a general file or script facility.
- ADR 0029 is superseded: the decision to fix the label stands, the mechanism
  does not.
- The `home_root_label` assertion in the identity check stays; it verifies the
  effect, not how the file got there.

**Amendment 2026-11-10.** Exactly one resource root may carry the tree, and it
must be the Core root (the root that carries `profiles/deskos-core.yaml`).
With the embedded Core that is the materialized Core root; with `--no-core` the
caller passes the Core tree, so the tree may be in either argument. A second
root that also has one is rejected. The files are included in
`generated-manifest.json` and listed in the build context, but they remain
static build-context inputs, not Plan or SBOM components: they are reviewed as
files, not computed as intent. The validator allows only safe `tmpfiles.d` line
types and rejects `Exec*` keys other than the reviewed `ExecStart=` replacement
in the fstrim drop-in.
