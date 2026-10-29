# Example: DeskOS Core on AlmaLinux 10

The minimal Core composition on AlmaLinux 10 (EL10, RHEL-compatible): a
**self-contained resource root** with `almalinux-10` in [`platforms/`](platforms/).

```bash
deskosctl plan ./examples/almalinux-10 \
  --workstation example-almalinux-10
```

| Resource | Contents |
|---|---|
| `almalinux-10` (platform) | `platforms/almalinux-10.yaml`: EL10 facts modeled on `rhel-10`, on the vendor's bootc base `quay.io/almalinuxorg/almalinux-bootc:10` |
| `example-almalinux-10` | the target: `almalinux-10` + the `deskos-core` and `deskos-gnome` profiles |

AlmaLinux 10 is **not a shipped platform**: the vendor calls its bootc images
experimental, and this repository focuses on CentOS and RHEL. It composes and
renders in CI, and a weekly job builds and boots it without publishing. To build
locally, follow [docs/example-platforms.md](../../docs/example-platforms.md); do
not publish it.
