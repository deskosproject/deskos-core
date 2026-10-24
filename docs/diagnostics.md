# Diagnostics

What the tools say when a resource, a build input or a machine is wrong, and what
to do next. Each row is a real message from a small fixture; the class is the
distinction the tools are expected to keep:

- **intent** — the resource root is inconsistent or ambiguous;
- **unsupported capability** — the platform cannot provide what a resource
  declares;
- **observed** — the machine differs from the plan, or the probe saw the state;
- **missing evidence** — the observation could not be made at all.

## Resource and composition failures

All of these are `deskosctl` failures: exit `1` for invalid input, `2` for a
usage error. `validate` is offline and stops at the first report.

| Situation | Message (excerpt) | Class | Next action |
|---|---|---|---|
| Workstation references a missing profile | `ws.yaml:1: Workstation/diag references Profile/does-not-exist, which is not defined` | intent | Create the profile or fix the name; the file and line are the referencing side |
| Profile references a missing resource | `profile.yaml:1: Profile/diag-profile references PackageSet/does-not-exist, which is not defined` | intent | Declare the resource in a root that composes with this one |
| Two files define the same resource | `duplicate resource PackageSet/same` + `first defined at a.yaml:1` + `defined again at b.yaml:1` | intent | Keep one definition; a keyed definition must be identical to deduplicate |
| Two different values in one layer | `composition conflict for GNOME setting "session.idle.blankAfter"` with both resources, values and sources | intent | Choose one value, or move one definition to another layer if the higher layer should win |
| A package the platform cannot supply | `package "virt-manager" is not available on Platform/centos-stream-10: only in …; requested by: role PackageSet/example-virt-manager in Profile/diag-profile (virt.yaml:1)` | unsupported capability | Use a supported package, or decide the capability is a real requirement and do not hide the gap |
| A setting needs an extension the platform does not declare | `dock.showTrash needs the GNOME extension … and Platform/diag-platform does not declare it`, with the setting and resource | unsupported capability | Drop the setting or add the extension/platform support; a silent no-op is not offered |

## Download and build inputs

`tests/artifacts/resolve.py` is the check that a declared artifact can be
fetched and that its bytes match its checksum. It distinguishes a verified
artifact, an unavailable URL, a size above `--max-bytes` (reported as **not
verified**, never a pass), and a repository whose body is not `repomd.xml`
(inconclusive). Run it against the rendered plan:

```bash
python3 tests/artifacts/resolve.py plan.json --summary resolve.md
```

A version-only Renovate change fails this check until the SHA-256 is filled:
the resolver can prove the bytes, but no tool can invent the checksum.

## Runtime evidence

`deskos` reads the plan the image carries. Exit codes: `0` success, `1` the
machine needs attention or the action was refused, `2` usage.

| Situation | Message (excerpt) | Class | Next action |
|---|---|---|---|
| No plan on the machine | `deskos report: this machine does not carry a plan (/usr/share/deskos/plan.json): …` + `hint: install a DeskOS image, or pass --root DIR for another tree` | missing evidence | Install a DeskOS image, or point `--root` at the tree being examined |
| Plan exists but is not readable | `/usr/share/deskos/plan.json is not a readable plan: unexpected end of JSON input` | missing evidence (corrupt) | Reinstall or re-render; do not guess at the missing content |
| A declared file is absent | `CHANGED  file …` + `missing: the plan installs this file and it is not there` | observed | Restore the file or accept that the machine differed; `check` reports, it does not repair |
| A dconf default differs | `CHANGED  dconf default …` with the observed value and the declared one | observed | Decide whether the user's value should win; it is not a lock |
| Update without the plan | `deskos update: this machine does not carry a plan …` + `hint: update runs on the machine whose image carries the plan; there is no --root` | missing evidence | Run it on the installed machine, not on another tree |
| Update without root | `deskos update: needs root to ask bootc to upgrade; run \`sudo deskos update\`` | missing evidence (privilege) | Run with `sudo`; read-only `report`/`check` do not need it |
| Not a bootc deployment | `[REFUSED] this machine is a bootc deployment: deskos updates a bootc image; this machine was not installed that way …` | unsupported capability | Update it the way it was installed; do not point `deskos` at a different machine |

`deskos check` also prints its scope and a `not measurable here` count. A check
that reports **0 not measurable** is not a claim that the machine is correct;
it means the declared scope was measured. Read the scope lines with the counts.

## What is deliberately not a diagnostic

`deskos` does not invent a command to restate what `bootc status`, systemd or
the kernel already says. When the right answer is `bootc status`, `journalctl`
or `systemctl --failed`, use that tool; the recovery page links the commands.
