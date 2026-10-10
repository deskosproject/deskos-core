# Example: DeskOS Core on RHEL 10

The minimal DeskOS Core composition on Red Hat Enterprise Linux 10. It is
a **separate resource root** that references the shipped `deskos-core`
profile and the `rhel-10` Platform by name:

```bash
deskosctl plan ./examples/core-rhel-10 \
  --workstation example-core-rhel-10
```

| Resource | Contents |
|---|---|
| `example-core-rhel-10` | the target: `rhel-10` + the `deskos-core` profile |

> [!IMPORTANT]
> RHEL-derived images are covered by the RHEL EULA and are **never
> published**. `rhel-10` marks them `redistributable: false`.
