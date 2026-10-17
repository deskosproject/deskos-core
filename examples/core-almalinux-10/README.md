# Example: DeskOS Core on AlmaLinux 10

The minimal DeskOS Core composition on AlmaLinux 10 (EL10, RHEL-compatible).
It is a **separate resource root** that references the shipped `deskos-core`
profile and the shipped `almalinux-10` Platform by name:

```bash
deskosctl plan ./examples/core-almalinux-10 \
  --workstation example-core-almalinux-10
```

| Resource | Contents |
|---|---|
| `almalinux-10` (platform) | shipped in `resources/platforms/`: EL10 facts modeled on `rhel-10`, on the vendor's bootc base `quay.io/almalinuxorg/almalinux-bootc:10` |
| `example-core-almalinux-10` | the target: `almalinux-10` + the `deskos-core` profile |

The base is published by the AlmaLinux project itself, which labels its bootc
images **experimental**. The platform follows ADR 0004's official-base rule and
declares no organization owner or tier (see the platform resource). Its facts
are EL10 conventions modeled on `rhel-10`: CI builds and boots this platform's
image and runs the boot, session and first-boot checks on it. Rocky Linux
publishes no bootc image, so AlmaLinux 10 (the closest EL10 bootc) stands in for
it.
