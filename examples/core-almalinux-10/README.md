# Example: DeskOS Core on AlmaLinux 10

The minimal DeskOS Core composition on AlmaLinux 10 (EL10,
RHEL-compatible). It is a **separate resource root** that carries its own
`almalinux-10` Platform (not shipped in `resources/`) and references the
`deskos-core` profile by name:

```bash
deskosctl plan ./examples/core-almalinux-10 \
  --workstation example-core-almalinux-10
```

| Resource | Contents |
|---|---|
| `almalinux-10` (platform) | EL10 facts modeled on `rhel-10`; bootc base `quay.io/almalinuxorg/almalinux-bootc:10` |
| `example-core-almalinux-10` | the target: `almalinux-10` + the `deskos-core` profile |

Rocky Linux publishes no bootc image, so AlmaLinux 10 (the closest EL10
bootc) stands in for it. The platform facts are EL10 conventions and are
**not verified** against AlmaLinux. This is example content: promote it to
`resources/platforms/` only after it is verified.
