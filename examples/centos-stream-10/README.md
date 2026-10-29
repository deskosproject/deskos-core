# Example: DeskOS Core on CentOS Stream 10

The minimal Core composition on the public platform: a separate resource root
referencing `deskos-core`, `deskos-gnome` and `centos-stream-10` by name.

```bash
deskosctl plan ./examples/centos-stream-10 \
  --workstation example-centos-stream-10
```

| Resource | Contents |
|---|---|
| `example-centos-stream-10` | the target: `centos-stream-10` + the `deskos-core` and `deskos-gnome` profiles |

The composition example lives in `../organization-and-role/`.
