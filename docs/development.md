# Development

Requires Go 1.26 or later (the oldest supported Go release, and the
minimum of the `golang.org/x/text` dependency).

Build the CLI:

```bash
go build -o bin/deskosctl ./cmd/deskosctl
```

Run tests:

```bash
go test ./...
python3 -m unittest discover -s tests/vm   # VM harness; CI runs it, make check does not
python3 -m unittest discover -s tests/rhel # RHEL factory harness, same
```

Validate the included resources:

```bash
./bin/deskosctl validate ./resources ./examples/example-org
```

Inspect the public CentOS reference workstation:

```bash
./bin/deskosctl plan ./resources \
  --workstation deskos-core-centos10
```

Add `--format json` to print the canonical plan.

## Build the image yourself

Contributors build the image from their checkout instead of pulling it.
Render the build context:

```bash
./bin/deskosctl render ./resources \
  --workstation deskos-core-centos10 \
  --backend containerfile \
  --output ./dist/deskos-core-centos10
```

and build it with rootful Podman, so bootc-image-builder can read it:

```bash
sudo podman build -t localhost/deskos-core-centos10 ./dist/deskos-core-centos10
```

The QCOW2 and ISO commands in [Installing DeskOS](install.md) work
unchanged with `localhost/deskos-core-centos10` as the image reference.
[tests/vm/README.md](../tests/vm/README.md) boots the disk and checks the
GNOME session against the plan; `tests/rhel/` does the same for RHEL on
an entitled host.

An example organization (`example-org`) with a RHEL 10 developer
workstation is included separately to prove organization- and
role-specific composition. It lives in its own
resource root and reuses DeskOS resources without copying them:

```bash
./bin/deskosctl plan ./resources ./examples/example-org \
  --workstation example-devops-rhel10

./bin/deskosctl render ./resources ./examples/example-org \
  --workstation example-devops-rhel10 \
  --output ./dist/example-devops-rhel10
```

RHEL builds require appropriate Red Hat access and must not be publicly
redistributed.

The `Makefile` wraps these commands for convenience (`make check`,
`make plan-example`, ...).

## CLI

The artifact-factory CLI is:

```bash
deskosctl
```

The initial workflow is:

```text
deskosctl validate ...
deskosctl plan ...
deskosctl render ...
deskosctl version
```

`validate` checks resources and composition.

`plan` shows the effective workstation before anything is built.

`render` produces a deterministic build context.

`version` prints the release version and commit (`dev` and the Git
revision for local builds).

## Releases

Pushing a tag `vX.Y.Z` (or `vX.Y.Z-<prerelease>`) runs the CI checks and
creates a GitHub Release with `deskosctl-vX.Y.Z-linux-amd64` and
`SHA256SUMS` (`.github/workflows/release.yml`). Only linux/amd64 is built.
The binary is not signed.

```bash
git tag -a vX.Y.Z -m "deskosctl vX.Y.Z"
git push origin vX.Y.Z
```

To download and verify `v0.1.0`, the current release:

```bash
gh release download v0.1.0 --repo deskosproject/deskos-core
sha256sum -c SHA256SUMS
chmod +x deskosctl-v0.1.0-linux-amd64
```

or without `gh`:

```bash
base=https://github.com/deskosproject/deskos-core/releases/download/v0.1.0
curl -fL -O "$base/deskosctl-v0.1.0-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c SHA256SUMS
```
