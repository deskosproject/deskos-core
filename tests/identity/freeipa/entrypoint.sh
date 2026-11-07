#!/bin/bash
# Boot a disposable FreeIPA server on CentOS Stream 10 for the identity fixture.
#
# The entrypoint hands PID 1 to systemd. A oneshot unit runs ipa-server-install
# on the first boot (only when /etc/ipa/default.conf is absent), then starts
# ipa.service and writes /run/ipa-ready. The harness waits for that file, so a
# failed install is reported instead of waited out. The installer needs
# systemctl to start LDAP, the KDC, the CA and DNS while it runs; a sleep loop
# cannot do that, which is why systemd is inside the container.
set -Eeuo pipefail
umask 0022
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

die() { printf 'FreeIPA: %s\n' "$*" >&2; exit 1; }

failed() {
    local rc=$?
    rm -f /run/ipa-ready
    printf 'FreeIPA: failed (%s); see journalctl -u ipa-container and /var/log/ipaserver-install.log\n' "$rc" >&2
    exit "$rc"
}

state=/var/lib/ipa-container
config=/run/ipa-container.env

if [[ ${1:-} == --configure && $# == 1 ]]; then
    # This branch runs under systemd, with D-Bus and the base services up.
    [[ $(cat /proc/1/comm) == systemd ]] || die 'the bootstrap needs systemd as PID 1'
    # The file holds printf %q escaped assignments, not something to eval blindly.
    # shellcheck source=/dev/null
    source "$config"
    rm -f "$config"
    trap failed ERR

    [[ ! -e $state/installing ]] || die 'a previous install did not finish; recreate the fixture and keep the logs'
    if [[ ! -e /etc/ipa/default.conf ]]; then
        mkdir -p "$state"
        # default.conf can exist before an install has finished; this mark also
        # survives SIGKILL or a container stop.
        touch "$state/installing"
        args=(
            --unattended
            "--hostname=$IPA_SERVER_HOSTNAME"
            "--domain=$IPA_SERVER_DOMAIN"
            "--realm=$IPA_SERVER_REALM"
            "--ds-password=$IPA_SERVER_PASSWORD"
            "--admin-password=$IPA_SERVER_PASSWORD"
            --setup-dns --no-reverse --no-ntp
            --allow-zone-overlap
        )
        if [[ -n $IPA_SERVER_IP ]]; then
            args+=("--ip-address=$IPA_SERVER_IP")
        else
            # Skips the DNS check before install, not the need for a local IP.
            args+=(--no-host-dns)
        fi
        if [[ -n $IPA_DNS_FORWARDER ]]; then
            args+=("--forwarder=$IPA_DNS_FORWARDER")
        else
            args+=(--no-forwarders)
        fi
        # Closed stdin and -U: an unexpected prompt must fail, never block CI.
        ipa-server-install "${args[@]}" </dev/null
        unset args IPA_SERVER_PASSWORD
        rm -f "$state/installing"
    fi

    # ipa.service runs ipactl and coordinates LDAP, the KDC, HTTP/CA and DNS. It
    # also runs when the same container is restarted without reinstalling.
    systemctl start ipa.service
    ipactl status
    dig +time=5 +tries=1 @127.0.0.1 "$IPA_SERVER_HOSTNAME" A +short | grep -q .
    touch /run/ipa-ready
    printf 'FreeIPA: fixture ready.\n'
    exit 0
fi

[[ $# == 0 ]] || die 'no arguments; --configure is reserved for systemd'
[[ $EUID == 0 ]] || die 'root is needed inside the container'
[[ $$ == 1 ]] || die 'run as the entrypoint/PID 1, without podman --init'
[[ -n ${IPA_SERVER_HOSTNAME:-} ]] || die 'IPA_SERVER_HOSTNAME (FQDN) is required'
[[ $IPA_SERVER_HOSTNAME == *.* && $IPA_SERVER_HOSTNAME != *[[:space:]]* ]] || die 'IPA_SERVER_HOSTNAME must be an FQDN'
[[ $(hostname) == "$IPA_SERVER_HOSTNAME" ]] || die 'pass --hostname with the same value as IPA_SERVER_HOSTNAME'

IPA_SERVER_DOMAIN=${IPA_SERVER_DOMAIN:-${IPA_SERVER_HOSTNAME#*.}}
IPA_SERVER_REALM=${IPA_SERVER_REALM:-${IPA_SERVER_DOMAIN^^}}
IPA_SERVER_IP=${IPA_SERVER_IP:-}
IPA_DNS_FORWARDER=${IPA_DNS_FORWARDER:-}
IPA_SERVER_PASSWORD=${IPA_SERVER_PASSWORD:-}
if [[ ! -e /etc/ipa/default.conf ]]; then
    [[ -n $IPA_SERVER_PASSWORD ]] || die 'IPA_SERVER_PASSWORD is required for admin and Directory Manager'
fi
[[ -d /sys/fs/cgroup && -w /sys/fs/cgroup ]] || die 'cgroupfs must be mounted and writable; see the run script'

# Services started by the system manager do not inherit PID 1's environment.
# Pass only these variables, in a root:root 0600 file on /run.
rm -f /run/ipa-ready
install -m 0600 /dev/null "$config"
for name in IPA_SERVER_HOSTNAME IPA_SERVER_DOMAIN IPA_SERVER_REALM IPA_SERVER_IP IPA_DNS_FORWARDER IPA_SERVER_PASSWORD; do
    printf '%s=%q\n' "$name" "${!name}" >> "$config"
done
unset IPA_SERVER_PASSWORD
exec /sbin/init
