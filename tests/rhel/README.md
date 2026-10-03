# RHEL factory validation

`factory.py` validates one deskos-core commit on an entitled RHEL 10 host:
render `example-devops-rhel10`, build it with Podman (`bootc container lint`
runs in the build), scan every layer for build-host subscription identity,
build a QCOW2 with the pinned `rhel10/bootc-image-builder` and run
`tests/vm/bootcheck.py` and `tests/vm/sessioncheck.py` from that commit.

RHEL images must not be publicly redistributed, so the image and the disk
stay on the host. The only thing published is a GitHub commit status,
`deskos/rhel10`, with a short description. GitHub never runs code on the
host: the host polls `main` and builds only commits already in the
canonical repository.

A commit whose rendered build context and harness are identical to an
already validated commit reuses that result instead of rebuilding.

## Host

A RHEL 10 VM or machine that is registered with subscription-manager, logged
in to `registry.redhat.io` as root, and has `/dev/kvm`. A run writes about
25 GB and deletes it; on a thin-provisioned VM disk use `discard='unmap'`
and mount the root with `discard`, so that space returns to the
hypervisor (the service also runs `fstrim` when it finishes).

    dnf install podman skopeo qemu-kvm qemu-img edk2-ovmf python3 git golang
    podman login --authfile /root/.config/containers/auth.json registry.redhat.io

The tool checkout is updated deliberately, not by the timer:

    git clone https://github.com/deskosproject/deskos-core.git /var/lib/deskos-factory/tools

Runs are kept under `/var/lib/deskos-factory/runs/<commit>/` (`summary.json`,
`factory.log`, boot and session evidence); the newest five are kept.

## Running

One commit, without publishing:

    python3 /var/lib/deskos-factory/tools/tests/rhel/factory.py check --sha <commit>

Exit status: 0 pass, 1 fail, 2 GitHub or credential error.

## Publishing the status

A fine-grained token for `deskosproject/deskos-core` with only
"Commit statuses: Read and write", stored encrypted and read from stdin:

    systemd-creds encrypt --name=github-token - /etc/credstore.encrypted/deskos-github-token
    cp systemd/deskos-rhel-factory.{service,timer} /etc/systemd/system/
    systemctl daemon-reload
    systemctl enable --now deskos-rhel-factory.timer

The timer checks the head of `main` every 15 minutes and skips commits that
already have a final `deskos/rhel10` status.
