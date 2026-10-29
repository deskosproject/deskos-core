# Tracking the checksum-pinned resources

**Status:** implemented (2026-10-27). A pin can be found, checked and moved when
a curated rule says how to read its version from the release tag. The weekly
workflow opens the bundle; a person merges it.

A `BinaryArtifact` and a GNOME extension source name an upstream file, a version
and a sha256. Renovate keeps base-image digests and Actions current but resolves
neither, because the version lives in the URL and no tool knows how the asset is
named. `deskosctl track` answers two questions without deciding anything:

- **Is the pin honest?** The URL is downloaded and hashed; a different file is a
  mismatch, not a silent update.
- **Is there something newer?** For a GitHub release URL, `--fresh` asks for the
  latest release tag. Any other URL has no answer, which is not an error.

```
deskosctl track [--fresh] [--token TOKEN] ROOT...
```

It prints one line per pin and exits non-zero when a hash does not match or
`--fresh` finds a newer release. `--token` defaults to `$GITHUB_TOKEN`, for the
API rate limit.

## Why it lives beside the command

`internal/` may not import `net/http`, `net` or `os/exec`: the compiler must stay
offline and deterministic (`internal/compiler/architecture_test.go`). The package
is `cmd/deskosctl/track`, which the compiler never calls, and a run never changes
a Plan, a resource or a golden.

## What a pin looks like

Any mapping with `version`, `url` and `sha256`, with the version beside the source
(`BinaryArtifact`) or in the same mapping (an extension source):

```yaml
spec:
  version: v0.17.2
  source:
    url: https://github.com/…/deskos_0.17.2_linux_amd64.tar.gz
    sha256: 3e60…
```

## Moving a pin

The release tag does not always carry the version (`extensions.gnome.org-v110` for
version `110`), so `track.yaml` holds the curated rule per repository: the
repository to match and the prefix to strip from the tag. `--write` only touches a
pin named there, and it rewrites the three scalars on their own lines, keeping
indentation, quoting and any trailing comment. Everything else stays byte for
byte.

## The weekly workflow

`track.yml` runs on Monday, or on dispatch: it builds the tool, runs
`track --fresh --write --rules track.yaml resources`, regenerates the goldens, and
opens one pull request with **a commit per resource** and the goldens in a commit
of their own. Nothing is published and nothing is merged.

Two things a reviewer should know:

- a bundle opened with `GITHUB_TOKEN` does not trigger other workflows, so its
  checks run only when a person re-runs or pushes to it;
- if a candidate asset is missing or has no hash, the pin is reported and left
  alone rather than moved on an assumption.
