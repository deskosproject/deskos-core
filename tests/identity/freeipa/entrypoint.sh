#!/bin/bash
set -Eeuo pipefail
umask 0022
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

die() { printf 'FreeIPA: %s\n' "$*" >&2; exit 1; }

failed() {
    local rc=$?
    rm -f /run/ipa-ready
    printf 'FreeIPA: fallo (%s); revisar journalctl -u ipa-container y /var/log/ipaserver-install.log\n' "$rc" >&2
    exit "$rc"
}

state=/var/lib/ipa-container
config=/run/ipa-container.env

if [[ ${1:-} == --configure && $# == 1 ]]; then
    # Esta rama la ejecuta systemd, con D-Bus y los servicios ya disponibles.
    [[ $(cat /proc/1/comm) == systemd ]] || die 'El bootstrap necesita systemd como PID 1.'
    # El fichero contiene asignaciones Bash escapadas por printf %q, no eval.
    # shellcheck source=/dev/null
    source "$config"
    rm -f "$config"
    trap failed ERR

    [[ ! -e $state/installing ]] || die 'Instalación anterior incompleta; recrear el fixture y conservar los logs.'
    if [[ ! -e /etc/ipa/default.conf ]]; then
        mkdir -p "$state"
        # default.conf puede existir ANTES de que termine una instalación fallida.
        # Conservamos esta marca también ante SIGKILL/corte del contenedor.
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
            # Omite la comprobación DNS previa, no la necesidad de una IP local.
            args+=(--no-host-dns)
        fi
        if [[ -n $IPA_DNS_FORWARDER ]]; then
            args+=("--forwarder=$IPA_DNS_FORWARDER")
        else
            args+=(--no-forwarders)
        fi
        # stdin cerrado y -U: un prompt inesperado debe fallar, nunca bloquear CI.
        ipa-server-install "${args[@]}" </dev/null
        unset args IPA_SERVER_PASSWORD
        rm -f "$state/installing"
    fi

    # ipa.service usa ipactl y coordina LDAP, KDC, HTTP/CA, DNS y demás servicios.
    # También se ejecuta al reiniciar el mismo contenedor sin reinstalarlo.
    systemctl start ipa.service
    ipactl status
    dig +time=5 +tries=1 @127.0.0.1 "$IPA_SERVER_HOSTNAME" A +short | grep -q .
    touch /run/ipa-ready
    printf 'FreeIPA: fixture disponible.\n'
    exit 0
fi

[[ $# == 0 ]] || die 'No admite argumentos; --configure está reservado para systemd.'
[[ $EUID == 0 ]] || die 'Se necesita root dentro del contenedor.'
[[ $$ == 1 ]] || die 'Ejecutar como entrypoint/PID 1, sin --init de Podman.'
[[ -n ${IPA_SERVER_HOSTNAME:-} ]] || die 'Falta IPA_SERVER_HOSTNAME (FQDN).'
[[ $IPA_SERVER_HOSTNAME == *.* && $IPA_SERVER_HOSTNAME != *[[:space:]]* ]] || die 'IPA_SERVER_HOSTNAME debe ser un FQDN.'
[[ $(hostname) == "$IPA_SERVER_HOSTNAME" ]] || die 'Usar --hostname con el mismo valor que IPA_SERVER_HOSTNAME.'

IPA_SERVER_DOMAIN=${IPA_SERVER_DOMAIN:-${IPA_SERVER_HOSTNAME#*.}}
IPA_SERVER_REALM=${IPA_SERVER_REALM:-${IPA_SERVER_DOMAIN^^}}
IPA_SERVER_IP=${IPA_SERVER_IP:-}
IPA_DNS_FORWARDER=${IPA_DNS_FORWARDER:-}
IPA_SERVER_PASSWORD=${IPA_SERVER_PASSWORD:-}
if [[ ! -e /etc/ipa/default.conf ]]; then
    [[ -n $IPA_SERVER_PASSWORD ]] || die 'Falta IPA_SERVER_PASSWORD para admin y Directory Manager.'
fi
[[ -d /sys/fs/cgroup && -w /sys/fs/cgroup ]] || die 'Se necesita cgroupfs montado y escribible; ver README.'

# Los servicios del system manager no heredan automáticamente el entorno de
# PID 1. Pasamos sólo estas variables en un fichero root:root 0600 sobre /run.
rm -f /run/ipa-ready
install -m 0600 /dev/null "$config"
for name in IPA_SERVER_HOSTNAME IPA_SERVER_DOMAIN IPA_SERVER_REALM IPA_SERVER_IP IPA_DNS_FORWARDER IPA_SERVER_PASSWORD; do
    printf '%s=%q\n' "$name" "${!name}" >> "$config"
done
unset IPA_SERVER_PASSWORD
exec /sbin/init
