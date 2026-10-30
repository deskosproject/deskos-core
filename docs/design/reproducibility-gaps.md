# What differs between two builds, and what matters

The compiler is deterministic: the same resources produce the same Plan, byte for
byte, and the tests pin it. The image is another question, and P3-05 asks which
differences matter for audit and recovery, with lockfiles proposed only for a
demonstrated gap. This is the measurement.

## Method

One rendered context (`example-devops-centos-stream-10`, 38 files), built twice with
`podman build --no-cache` on the same host, 45 minutes apart, from the same pinned
base digest and the same pinned artifacts. No repository snapshot was involved: both
builds resolved packages from the live CentOS Stream 10 repositories.

## What came out the same

| Thing | Result |
|---|---|
| Plan | identical; the compiler is deterministic and its goldens say so |
| Image config (env, labels, entrypoint) | identical |
| RPM set | **1473 packages, identical names and versions** |
| `/usr/bin/deskos` | `344a099b…`, identical |
| The generated keyfile | `5f308b61…`, identical |
| A pinned font (`AdwaitaSans-Regular.ttf`) | `8381c33b…`, identical |
| `/usr/share/deskos/plan.json` | `92b75f6b…`, identical |

So for the same inputs and the same repository state, the *content* is
reproducible: the files are the same bytes and the installed package set is the
same.

## What differed

| Thing | Result |
|---|---|
| Image ID and manifest digest | different (`f6df272d…` against `9c2b1dbf…`) |
| Layers | 25 of 93 differ, from the package transaction onward |
| The bytes inside those layers | file metadata: modification times from downloads and package installs, and the timestamps dnf and the RPM database write |

The difference is **metadata, not content**: an OCI layer digest covers the tar
entry, including its mtime, so a file downloaded at 11:34 and the same file
downloaded at 12:19 produce different layers and therefore a different image
digest. The config differs only in the timestamp it records.

## The gap that matters

Timestamps break bit-for-bit equality and nothing else: an auditor comparing the
plan, the package set or a file's hash sees the same answer.

The gap that matters is **the repository state**. Both builds were 45 minutes
apart, so they resolved the same packages. A build tomorrow, with the same inputs
and the same base digest, can resolve a newer package, and then the installed set
and the SBOM change while the resources did not. Nothing in the repository pins
that: the base is pinned by digest, the artifacts by checksum, and the RPMs by
nothing at all.

For audit that is survivable because the SBOM travels with the image and the
promotion record keeps the digest. For **recovery** it is the real question: "build
me this workstation as it was" is answered by the published image, not by the
resources, and only for as long as the registry keeps it.

## Satellite content views, and where the build has to meet them

A **published content view version is a frozen snapshot**: publishing copies the
selected repositories, and a later sync does not change that version. Satellite
serves it at
`https://satellite.example.com/pulp/content/<org>/<environment>/<view>/...`, and an
archived version can be distributed without a lifecycle environment. That is exactly
the pin the build needs: point dnf at a published version and the RPMs stop moving
until someone publishes a new one.

**Registration is the transport, not the freeze.** `subscription-manager` only
writes `redhat.repo` with whatever the server it registers to publishes. Registered
against the Red Hat CDN, those repositories move, and the packages are as unpinned
as CentOS Stream's. Registered against Satellite or Foreman with Katello, with an
activation key bound to a published content view, they are frozen. The mechanism is
the same; the server decides.

**Katello does this without a Red Hat subscription.** Satellite is the supported
product; Foreman with Katello is the same content engine and mirrors CentOS Stream
as a custom product, publishes a content view, and registers the host with
`subscription-manager --org ... --environment <environment>/<view>` or an
activation key. The registration needs the server's CA
(`katello-ca-consumer-latest.noarch.rpm`), and nothing else. That is how the first
DeskOS deployments consumed CentOS content, and it is why this page names Katello
and not only Satellite.

**On CentOS Stream the build does not register at all today**, so it resolves from
the base image's own repositories and a content view would be one more repository
next to them. Measured: a later `/etc/yum.repos.d/` file that redefines `[baseos]`
with `enabled=0` is ignored, and dnf keeps the base image's definition. Pointing at
a content view is not enough while the public repositories stay enabled.

Two ways to make the build meet the snapshot, and one of them is a decision:

1. **An organization-owned base** whose `/etc/yum.repos.d/` already points at
   Satellite, built from `centos-bootc` and pinned by digest. This works today, is
   the ADR 0014 path, and needs an owner and a tier.
2. **An isolated build**: a platform or package-set flag that runs the package
   transaction with `--disablerepo='*' --enablerepo=<the declared repositories>`, so
   only the content view answers. It changes what a `PackageSet` means, so it needs
   an ADR before it is a field.

Either way the **content view version is an input** and belongs in the record next
to the base digest: a republished view is a new input, exactly as a new base digest
is, and the image digest that comes out is a new record.

## What would close it, and what would not

- **A lockfile of resolved RPM versions**: worth it only if an organization needs to
  rebuild without a snapshot. It would record what the build resolved, which the
  SBOM already does for the image that exists; the lockfile would make it
  *replayable*. Not proposed until someone asks for that replay.
- **Reproducible timestamps** (`SOURCE_DATE_EPOCH` and stripping mtimes from the
  layer): it would make the digest stable for the same content. It buys a smaller
  diff and a nicer story, not an audit property the plan and SBOM do not already
  give.

## What the repository may claim

- "Compilation is deterministic; images are not yet bit-for-bit reproducible" is
  accurate, and now measured: the content is reproducible, the manifest is not, and
  the package set is only as reproducible as the repository it came from.
- The published image digest is the record of a build. A rebuild is a new record,
  even when the bytes match.
