# Development

Building `deskosctl` and the DeskOS image from a checkout, running the
tests, and cutting releases.

**Contents:**
[Build and test](#build-and-test) ·
[Build the image yourself](#build-the-image-yourself) ·
[CLI](#cli) ·
[Releases](#releases)

## Build and test

Requires **Go 1.26 or later** (the oldest supported Go release, and the
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
python3 -m unittest discover -s tests/supply-chain # SBOM comparison harness, same
```

Validate the included resources:

```bash
./bin/deskosctl validate ./examples/baseline-and-role
```

Inspect the public CentOS reference workstation:

```bash
./bin/deskosctl plan \
  --workstation deskos-core-centos10
```

Add `--format json` to print the canonical plan.

## Build the image yourself

Contributors build the image from their checkout instead of pulling it.

1. Render the build context:

   ```bash
   ./bin/deskosctl render \
     --workstation deskos-core-centos10 \
     --backend containerfile \
     --output ./dist/deskos-core-centos10
   ```

2. Build it with **rootful Podman**, so bootc-image-builder can read it:

   ```bash
   sudo podman build -t localhost/deskos-core-centos10 ./dist/deskos-core-centos10
   ```

The QCOW2 and ISO commands in [Installing DeskOS](install.md) work
unchanged with `localhost/deskos-core-centos10` as the image reference.
[tests/vm/README.md](../tests/vm/README.md) boots the disk and checks the
GNOME session against the plan; [`tests/rhel/`](../tests/rhel/README.md)
does the same for RHEL on an entitled host.

### Example: baseline and role

An example that composes an organization baseline and a developer role on
RHEL 10 is included separately to prove organization- and role-specific
composition. It lives in its own resource root and reuses
DeskOS resources *without copying them*; see
[examples/baseline-and-role/README.md](../examples/baseline-and-role/README.md):

```bash
./bin/deskosctl plan ./examples/baseline-and-role \
  --workstation example-devops-rhel10

./bin/deskosctl render ./examples/baseline-and-role \
  --workstation example-devops-rhel10 \
  --output ./dist/example-devops-rhel10
```

> [!IMPORTANT]
> RHEL builds require appropriate Red Hat access and **must not be
> publicly redistributed**.

The `Makefile` wraps these commands for convenience (`make check`,
`make plan-example`, ...).

## CLI

The artifact-factory CLI is `deskosctl`. The initial workflow is:

```text
deskosctl validate ...
deskosctl plan ...
deskosctl render ...
deskosctl sbom ...
deskosctl version
```

| Command | Purpose |
|---|---|
| `validate` | checks resources and composition |
| `plan` | shows the effective workstation *before anything is built* |
| `render` | produces a **deterministic** build context |
| `sbom` | writes the declared CycloneDX SBOM of one workstation |
| `version` | prints the release version and commit (`dev` and the Git revision for local builds) |

The declared SBOM and where a CVE scan fits are described in
[supply-chain.md](supply-chain.md).

## Releases

Pushing a tag `vX.Y.Z` (or `vX.Y.Z-<prerelease>`) runs the CI checks and
creates a GitHub Release (`.github/workflows/release.yml`) with:

| Asset | Contents |
|---|---|
| `deskosctl-vX.Y.Z-linux-amd64` | the CLI; only **linux/amd64** is built |
| `deskos-resources-vX.Y.Z.tar.gz` | `resources/` of the same commit (DeskOS Core, platforms, assets), validated by that binary before release; a reproducible archive |
| `SHA256SUMS` | checksums of both |

> [!NOTE]
> The binary is not signed.

```bash
git tag -a vX.Y.Z -m "deskosctl vX.Y.Z"
git push origin vX.Y.Z
```

> [!IMPORTANT]
> The copy-paste version in [install.md](install.md) and in this file is
> concrete, not a placeholder. Bump it in the commit you tag: the release job
> refuses a tag the docs do not name, so it cannot go stale.

To download and verify `v0.9.2`, the current release:

```bash
gh release download v0.9.2 --repo deskosproject/deskos-core
sha256sum -c SHA256SUMS
chmod +x deskosctl-v0.9.2-linux-amd64
tar -xzf deskos-resources-v0.9.2.tar.gz
```

or without `gh`:

```bash
base=https://github.com/deskosproject/deskos-core/releases/download/v0.9.2
curl -fL -O "$base/deskosctl-v0.9.2-linux-amd64" \
     -O "$base/deskos-resources-v0.9.2.tar.gz" -O "$base/SHA256SUMS"
sha256sum -c SHA256SUMS
```
