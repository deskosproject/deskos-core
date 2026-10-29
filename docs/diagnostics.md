# Diagnostics

What the tools say when a resource, a build input or a machine is wrong, and what
to do next. Every message below is real. Four classes:

- **intent**: the resource root is inconsistent or ambiguous;
- **unsupported**: the platform cannot provide what a resource declares;
- **observed**: the machine differs from its plan;
- **unobserved**: the check could not measure at all.

## Resource and composition failures

`deskosctl` exits `1` for invalid input and `2` for a usage error. `validate` is
offline and stops at the first report.

| Situation | Message (excerpt) | Class | Next action |
|---|---|---|---|
| A workstation names a missing profile | `Workstation/diag references Profile/does-not-exist, which is not defined` | intent | create the profile or fix the name; the message names the referencing side |
| A profile names a missing resource | `Profile/diag-profile references PackageSet/does-not-exist` | intent | declare it in a root that composes with this one |
| Two files define one resource | `duplicate resource PackageSet/same` with both file and line | intent | keep one definition |
| Two values in one layer | `composition conflict for GNOME setting "session.idle.blankAfter"` with both resources and values | intent | choose one, or move one to another layer |
| A package the platform lacks | `package "virt-manager" is not available on Platform/centos-stream-10: only in …` | unsupported | use a supported package, or keep the gap visible |
| A setting needs an undeclared extension | `dock.showTrash needs the GNOME extension … and Platform/diag-platform does not declare it` | unsupported | drop the setting or add the platform support; there is no silent no-op |

## Download and build inputs

`tests/artifacts/resolve.py` proves that a declared artifact can be fetched and
that its bytes match the checksum. It separates verified, unavailable, too large
(`--max-bytes`, reported as not verified) and a repository whose body is not
`repomd.xml` (inconclusive).

```bash
python3 tests/artifacts/resolve.py plan.json --summary resolve.md
```

A version-only Renovate change fails this until the SHA-256 is filled: the
resolver proves bytes, and no tool can invent a checksum.

## The machine

`deskos` reads the plan the image carries. Exit `0` success, `1` attention or
refused, `2` usage.

| Situation | Message (excerpt) | Class | Next action |
|---|---|---|---|
| No plan on the machine | `this machine does not carry a plan (/usr/share/deskos/plan.json)` and a `--root` hint | unobserved | install a DeskOS image, or point `--root` at the tree to examine |
| A plan that does not parse | `is not a readable plan: unexpected end of JSON input` | unobserved | reinstall or re-render; do not guess the content |
| A declared file is absent | `CHANGED  file …` and `missing: the plan installs this file and it is not there` | observed | restore it, or accept that the machine differed; `check` reports and does not repair |
| A dconf default differs | `CHANGED  dconf default …` with both values | observed | decide whether the user's value should win; it is not a lock |
| `update` without a plan | `update runs on the machine whose image carries the plan; there is no --root` | unobserved | run it on the installed machine |
| `update` without root | `needs root to ask bootc to upgrade; run sudo deskos update` | unobserved | run with `sudo`; `report` and `check` do not need it |
| Not a bootc deployment | `this machine is a bootc deployment …` refused | unsupported | update it the way it was installed |

`deskos check` prints its scope and a `not measurable here` count. A count of zero
means the declared scope was measured, not that the machine is correct.

## Not a diagnostic

`deskos` does not restate what `bootc status`, `journalctl` or `systemctl --failed`
already says. When one of those is the answer, use it; [recovery.md](recovery.md)
links the commands.
