# ADR 0029: a platform names the directories its base can leave unlabeled

**Status:** accepted (2026-11-05)

## Context

Every bootc base DeskOS pins symlinks `/home` to `var/home` and does not ship
`var/home` itself. Measured 2026-11-05 in the pinned images: CentOS Stream 10
`f43c24c5`, RHEL 10 `80485ddb`, AlmaLinux 10 `5ef1b06e` and Fedora 45 all show
`/home -> var/home` and `ls: cannot access '/var/home'`.

The base's `rpm-ostree-0-integration.conf` creates the directory at boot with
`d /var/home 0755 root root -`. But `d` does not relabel a directory that already
exists, and whichever component creates it first (the installer, an early boot
step) can leave it as `default_t`.

SELinux then blocks the login. In `identity-ad` run 41761052759 the guest denied
the home creation:

```
avc: denied { create } for pid=4721 comm="mkhomedir" name="qa"
  scontext=system_u:system_r:oddjob_mkhomedir_t:s0
  tcontext=system_u:object_r:default_t:s0 tclass=dir permissive=0
```

`oddjobd` was active and `authselect` had `with-mkhomedir`; the only problem was
the directory's label. The user got no home, and the session then failed every
check that depends on one. The same base on another disk created the home
correctly (`identity` run 42041183243), so this is a race in who creates the
directory first, not a constant of the base.

[Issue #409](https://github.com/deskosproject/deskos-core/issues/409) tracks the
bug. A test-only `restorecon` would hide a user-facing failure, and DeskOS has no
kind that carries an arbitrary file, so the fix needs a model decision.

## Decision

**A platform declares the directories its base can leave unlabeled as a typed
list, and the image ships the tmpfiles.d rule that creates and relabels them.**

- `Platform.spec.relabelDirs` is a list of absolute, clean paths other than `/`,
  unique. The platform that needs the rule declares it; the compiler does not
  guess from the base family.
- The Plan carries a generated file `/usr/lib/tmpfiles.d/zz-deskos-relabel.conf`
  with one pair per directory, sorted:

  ```
  d /var/home 0755 root root -
  z /var/home - - - -
  ```

- `d` creates the directory when the boot path did not; `z` restores the policy
  context of that directory at every boot. `z` is not recursive: children
  created later receive their own contexts from policy (`home_t` for a user's
  home, not `home_root_t`).
- The file sorts after the base's `rpm-ostree-0-integration.conf`, so the base's
  own `d` cannot run last, and the drop-in's `d` makes the result independent of
  whether the base ships that rule at all.
- Provenance is `Platform/<name>` at the foundation layer, like the platform's
  masked units. No kind is added, and the list lowers to a declarative tmpfiles
  entry, never to a shell command.

All four platforms declare `/var/home`. On a base that already ships the
directory labeled, both lines are no-ops.

## Consequences

- A directory user's login creates the home through `oddjob_mkhomedir_t`
  regardless of which component created `var/home` first.
- The identity suite records `home_root_label` from `/var/home` and checks it
  against `home_root_t`, so a regression fails as a labeling error instead of a
  missing home.
- The field is not a general file mechanism: it carries paths, not content or
  commands, and the tmpfiles entry only creates a directory and restores its
  context. A path outside the base's ostree layout would create an unused
  directory, which the platform resource review catches.
- When a base ships `var/home` with `home_root_t`, the declaration can be
  removed; leaving it is harmless.
