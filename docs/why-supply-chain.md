# Why the supply-chain records exist

A machine image is a black box unless someone writes down what is inside it and
who built it. The records around a DeskOS image are there to answer five plain
questions:

1. What did we ask the build to install?
2. What did the build actually install?
3. Does that content carry known vulnerabilities?
4. Is the thing we tested the same bytes we publish?
5. Who built it, from which commit and base, and can someone check that later?

No single file answers all five. That is why there are several, and why the
pipeline looks heavier than `podman build && podman push`.

| Question | Record | In plain words |
|---|---|---|
| What did we ask for? | Declared SBOM (`deskosctl sbom`) | The ingredient list we wrote, before the build: packages, files, binaries, Flatpaks, trust anchors |
| What is installed? | Installed SBOM (Syft) | The ingredient list read from the built image. On Fedora it is the RPM inventory the comparison needs; the scan covers the binaries it does not list |
| Does it carry known problems? | Grype report and exception record | A scanner compares the installed content with known vulnerabilities; every accepted risk is exact, owned and dated |
| Did it work? | Boot and session evidence | The image was turned into a disk and actually started; the desktop checks ran |
| Who built it, from what? | SLSA provenance | A standard note that names the source commit, the base digest and the workflow that produced the image |
| Is this statement about these bytes? | Signature over the digest, plus attestations | The signature is on the image digest; the attestations attach the lists and the scan to that same digest |
| Can I check later? | Durable evidence index | The records are stored under the digest, so the 14-day workflow artifact is not the only copy |

## The chain

The order matters:

```text
build -> declared SBOM -> installed SBOM -> scan -> boot/session
      -> push the digest -> sign and attest -> store evidence -> promote
```

Each link answers a question the previous one cannot:

- **Two SBOMs, not one.** The declared list can be right while the build
  installs something else. Comparing them catches a package that resolved to a
  different name or version.
- **A scan needs installed content.** Scanning the declaration would test what
  we meant to install, not what the image carries.
- **A scan needs a digest.** A report without the digest it belongs to cannot
  be tied to the bytes a machine runs.
- **A signature needs a digest.** A signature over a tag proves little, because
  the tag can move. The signature is over the digest; the tag is only a way to
  find it.
- **An attestation is a signed statement about the digest.** It carries the
  SBOM, the exceptions and the provenance, so a consumer does not have to trust
  a build log that may have expired.
- **Evidence is the archive.** A signature can be checked years later; the
  build log usually cannot. The durable index keeps the proof with the digest.
- **Promotion is a separate move.** `:candidate` and `:latest` are mutable
  references; `:stable` moves only after the records and the signature are
  checked.

## What this does not prove

The records make a claim checkable. They do not make the claim larger than it
is:

- A signed image is not a secure image. It means the bytes are the ones that
  were built and tested, not that they contain no bugs.
- A scan covers the scanner's database at that moment. A new advisory can
  arrive after the build; the weekly rebuild and the rescan procedure are how
  that is handled.
- An exception is a recorded decision, not a fix. The evidence still reports
  the finding; the gate is told to accept it until the review date.
- Provenance names the builder and the inputs. It is not a support contract and
  not a statement that the vendor supports the result.
- Nothing here enforces policy on a running machine. The image carries
  defaults; a machine changes only when its owner moves it to another image.

## Why it is worth the work

The alternative is a build log and a person saying "trust me". That is enough
until the machine is in a government office, a hospital or a fleet someone else
operates. Then the question is not "did it build?" but "can you show what is
inside, who built it, what was checked, and that the answer belongs to this
digest?" The records are the answer, and they cost a few minutes per build.

If you only keep one sentence: the digest is the identity, the signature says
who stands behind it, the attestations carry the lists and the scan, and the
evidence lets a stranger check the chain without trusting this repository.
