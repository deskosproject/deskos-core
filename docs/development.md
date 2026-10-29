# Development

Build `deskosctl` and the DeskOS image from a checkout, run the tests, cut
releases. Go 1.26 or later is required.

## Build and test

```bash
go build -o bin/deskosctl ./cmd/deskosctl
```

Run tests:

```bash
go test ./...
python3 -m unittest discover -s tests/vm   # VM harness; CI runs it, make check does not
python3 -m unittest discover -s tests/supply-chain # SBOM comparison harness, same
```

Validate the included resources, and inspect the public CentOS reference
workstation (`--format json` prints the canonical plan):

```bash
./bin/deskosctl validate ./examples/organization-and-role
./bin/deskosctl plan --workstation deskos-core-centos-stream-10
```

## Build the image yourself

1. Render the build context:

   ```bash
   ./bin/deskosctl render \
     --workstation deskos-core-centos-stream-10 \
     --backend containerfile \
     --output ./dist/deskos-core-centos-stream-10
   ```

2. Build it with **rootful Podman**, so bootc-image-builder can read it:

   ```bash
   sudo podman build -t localhost/deskos-core-centos-stream-10 ./dist/deskos-core-centos-stream-10
   ```

The QCOW2 and ISO commands in [Installing DeskOS](install.md) take
`localhost/deskos-core-centos-stream-10` as the image reference, and
[tests/vm/README.md](../tests/vm/README.md) boots the disk and checks the session.

### Example: baseline and role

An example that composes an organization baseline and a developer role on RHEL 10
reuses DeskOS resources without copying them
([README](../examples/organization-and-role/README.md)):

```bash
./bin/deskosctl plan ./examples/organization-and-role \
  --workstation example-devops-rhel-10

./bin/deskosctl render ./examples/organization-and-role \
  --workstation example-devops-rhel-10 \
  --output ./dist/example-devops-rhel-10
```

> [!IMPORTANT]
> RHEL builds require appropriate Red Hat access and **must not be publicly
> redistributed**.

## CLI

| Command | Purpose |
|---|---|
| `validate` | checks resources and composition |
| `plan` | shows the effective workstation *before anything is built* |
| `render` | produces a **deterministic** build context |
| `sbom` | writes the declared CycloneDX SBOM of one workstation |
| `version` | prints the release version and commit (`dev` and the Git revision for local builds) |

The declared SBOM and where a CVE scan fits are in [supply-chain.md](supply-chain.md).

The CLI is also published as a container, `quay.io/deskos/deskosctl:<tag>`, from a
release tag and signed like the other release artifacts. The embedded Core travels
inside the image, so only the output directory needs a mount:

```bash
mkdir -p out
podman run --rm --userns=keep-id -v "$PWD/out":/out:z \
    quay.io/deskos/deskosctl:latest render \
    --workstation deskos-core-centos-stream-10 --output /out
```

Your own resources are an **extra** root, one that adds resources rather than
repeating the Core, which fails the load with "defined again". `--userns=keep-id`
maps your uid so the tool can write the output, and `:z` relabels the mount: on an
SELinux host an unrelabeled root fails with `permission denied`, and relabeling a
whole checkout can fail on root-owned artifacts, so mount the resource tree:

```bash
podman run --rm --userns=keep-id \
    -v "$PWD/org-resources":/org:ro,z -v "$PWD/out":/out:z \
    quay.io/deskos/deskosctl:latest render \
    /org --workstation my-workstation --output /out
```

`:latest` is the newest **stable** release: a `vX.Y.Z-<prerelease>` tag publishes
and signs its own tag and leaves `:latest` alone.

## Releases

Pushing a tag `vX.Y.Z` (or `vX.Y.Z-<prerelease>`) runs the CI checks and creates a
GitHub Release (`.github/workflows/release.yml`) with:

| Asset | Contents |
|---|---|
| `deskosctl-vX.Y.Z-linux-amd64` | the CLI that builds an image; only **linux/amd64** is built |
| `deskos-vX.Y.Z-linux-amd64` | the runtime companion the image carries (ADR 0018), same commit and version checks |
| `deskos-core-resources-vX.Y.Z.tar.gz` | `resources/` of the same commit (DeskOS Core, platforms, assets), validated by that binary before release; a reproducible archive |
| `SHA256SUMS` | checksums of the three |

The version-less aliases `deskosctl-linux-amd64` and `deskos-linux-amd64` make
`releases/latest/download/<name>` work without knowing the tag; they track the
latest stable release, not a prerelease.

> [!NOTE]
> The binary is not signed.

```bash
git tag -a vX.Y.Z -m "deskosctl vX.Y.Z"
git push origin vX.Y.Z
```

To download and verify the current release (a `vX.Y.Z` name pins one), with `gh`
or without it:

```bash
gh release download --repo deskosproject/deskos-core \
  --pattern 'deskosctl-linux-amd64' --pattern 'SHA256SUMS'
sha256sum -c --ignore-missing SHA256SUMS
chmod +x deskosctl-linux-amd64
```

```bash
base=https://github.com/deskosproject/deskos-core/releases/latest/download
curl -fL -O "$base/deskosctl-linux-amd64" -O "$base/SHA256SUMS"
sha256sum -c --ignore-missing SHA256SUMS
```
