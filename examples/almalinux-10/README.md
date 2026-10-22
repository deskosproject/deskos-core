# Example: DeskOS Core on AlmaLinux 10

The minimal DeskOS Core composition on AlmaLinux 10 (EL10, RHEL-compatible).
It is a **self-contained resource root**: the `almalinux-10` Platform lives in
[`platforms/`](platforms/), next to the example workstation.

```bash
deskosctl plan ./examples/almalinux-10 \
  --workstation example-almalinux-10
```

| Resource | Contents |
|---|---|
| `almalinux-10` (platform) | `platforms/almalinux-10.yaml`: EL10 facts modeled on `rhel-10`, on the vendor's bootc base `quay.io/almalinuxorg/almalinux-bootc:10` |
| `example-almalinux-10` | the target: `almalinux-10` + the `deskos-core` and `deskos-gnome` profiles |

AlmaLinux 10 is **not a shipped platform**. The vendor labels its bootc images
experimental, and this repository focuses its publish and release work on
CentOS Stream 10 and RHEL 10. The example still composes and renders in CI; a
weekly job builds and boots it without publishing.

To build it locally, including refreshing the base digest and writing a QCOW2,
follow [docs/parked-platforms.md](../../docs/parked-platforms.md). Do not
publish the resulting image.
