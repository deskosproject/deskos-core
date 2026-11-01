# Example: a developer workstation

A **role** over DeskOS Core: the compiler and container toolchain, a terminal and
a short list of favorites. It references Core profiles by name instead of
copying them.

```bash
deskosctl plan ./examples/developer \
  --workstation example-developer
```

| Resource | Contents |
|---|---|
| `example-developer` (role) | `git`, `make`, `gcc`, `golang`, `podman`, `buildah`, `skopeo`, `toolbox`, and a desktop profile |
| `example-developer-desktop` | the terminal binding and three favorites, as defaults |
| `example-developer` (workstation) | CentOS Stream 10 + the Core desktop + this role |

## What it shows

- **A role is a Profile**, not a whole root: `layer: role` composes over the
  foundation and only names what it adds.
- **The toolchain is RPMs the platform serves.** No vendor archive is added, so
  the build resolves from the same repositories as the rest of the image.
- **Defaults stay overridable.** The favorites and the terminal binding are
  defaults; a person can change them. A managed workstation locks them instead,
  which is what [kiosk](../kiosk/) shows.

## What it does not show

- **An editor from a vendor archive.** VS Code is an RPM from Microsoft's
  repository, and adding that repository is an organization decision with a
  signing key, shown in [layers](../layers/).
- **A container runtime other than Podman.** Docker is not in the EL10 archive.
- **A language toolchain the platform does not package.** Rust from rustup or a
  Node.js from nvm manages files under a user's home, which is outside the
  image (ADR 0005); an organization that needs one pins a `BinaryArtifact` in a
  system path instead.

Swap `platformRef` and the family profile for another platform; the role itself
does not change.
