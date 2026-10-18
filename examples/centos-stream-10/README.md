# Example: DeskOS Core on CentOS Stream 10

The minimal DeskOS Core composition on the public, redistributable
platform. It is a **separate resource root** that references the shipped
`deskos-core` and `deskos-gnome` profiles and the `centos-stream-10` Platform by
name:

```bash
deskosctl plan ./examples/centos-stream-10 \
  --workstation example-centos-stream-10
```

| Resource | Contents |
|---|---|
| `example-centos-stream-10` | the target: `centos-stream-10` + the `deskos-core` and `deskos-gnome` profiles |

This is the per-platform example. The composition example (an
organization baseline and a role) lives in `../organization-and-role/`.
