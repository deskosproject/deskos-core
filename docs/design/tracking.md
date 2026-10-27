# Tracking the checksum-pinned resources

**Status:** the read-only half is implemented (2026-10-27). A resource pin can be
found, downloaded and checked; moving it is still a person's edit.

A `BinaryArtifact` and a GNOME extension source name an upstream file, a version
and a sha256. Renovate keeps base-image digests and Actions current, but nothing
resolves these, because the version lives in the URL and no tool knows how the
asset is named. `deskosctl track` is the maintenance step that answers two
questions without deciding anything:

- **Is the pin honest?** The URL is downloaded and hashed; a different file is a
  mismatch, not a silent update.
- **Is there something newer?** For a GitHub release URL, `--fresh` asks for the
  latest release tag. A URL that is not a GitHub release has no answer, which is
  not an error.

```
deskosctl track [--fresh] [--token TOKEN] ROOT...
```

It prints one line per pin and exits non-zero when a hash does not match, or when
`--fresh` finds a newer release. `--token` defaults to `$GITHUB_TOKEN`, for the
API rate limit.

## Why it lives beside the command

`internal/` may not import `net/http`, `net` or `os/exec`: the compiler must stay
offline and deterministic (`internal/compiler/architecture_test.go`). The package
is `cmd/deskosctl/track`, and the compiler never calls it. A run of `track` never
changes a Plan, a resource or a golden.

## What a pin looks like

Any mapping with `version`, `url` and `sha256`, with the version beside the
source (`BinaryArtifact`) or in the same mapping (an extension source):

```yaml
spec:
  version: v0.17.2
  source:
    url: https://github.com/…/deskos_0.17.2_linux_amd64.tar.gz
    sha256: 3e60…
```

## What is not here yet

- **Moving the pin.** Rewriting needs the curated rule: the release tag does not
  always carry the version (`extensions.gnome.org-v110` for version `110`), so
  the tool cannot derive one from the other without being told. Until then it
  reports, and a person edits.
- **The scheduled workflow** with a curated list and one bundle pull request, one
  commit per resource. It runs `--fresh`, opens the bundle, and a person merges;
  nothing is published.
- **The goldens.** A moved pin changes the rendered context, so the same change
  carries `go test ./internal/compiler -update` in a second commit, reviewed like
  any other.
