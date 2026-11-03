#!/usr/bin/env bash
# A disposable Samba AD DC for the identity fixture. Not production.
#
#   AD_IMAGE=localhost/deskos-ad:test ADMIN_PASSWORD=… sudo -E tests/identity/samba-ad.sh start
#   sudo tests/identity/samba-ad.sh status
#   AD_USER=qa AD_USER_PASSWORD=… sudo -E tests/identity/samba-ad.sh user
#   sudo tests/identity/samba-ad.sh purge
#
# The realm is throwaway and the container keeps no volume: every start
# provisions a fresh domain. Run it as root: provisioning Samba AD needs real
# ownership and `security.*` xattrs, which a rootless container does not have.
set -euo pipefail

name=deskos-ad
realm=${AD_REALM:-AD.LAB.TEST}
domain=${AD_DOMAIN:-AD}
hostname=${AD_SERVER_HOSTNAME:-dc.ad.lab.test}
dnsdomain=$(printf '%s' "$realm" | tr '[:upper:]' '[:lower:]')
# QEMU user networking exposes the host's loopback as 10.0.2.2, so the ports are
# published there. Nothing binds a public interface.
bind=${AD_BIND:-127.0.0.1}

usage() {
  sed -n '2,10p' "$0" | sed 's/^# \{0,1\}//'
}

case "${1:-}" in
  start)
    : "${AD_IMAGE:?set AD_IMAGE to the built DC image}"
    : "${ADMIN_PASSWORD:?set ADMIN_PASSWORD for the throwaway Administrator}"
    # SYS_ADMIN is what lets provisioning store the SYSVOL ACL: `security.NTACL`
    # is a security-namespace xattr, and the default rootful capability set does
    # not include SYS_ADMIN. Without it the provision dies with ACCESS_DENIED.
    podman run -d --name "$name" --hostname "$hostname" --cap-add=SYS_ADMIN \
      -p "$bind:53:53/udp" -p "$bind:53:53/tcp" \
      -p "$bind:88:88/tcp" -p "$bind:88:88/udp" \
      -p "$bind:389:389/tcp" -p "$bind:445:445/tcp" \
      -p "$bind:464:464/tcp" -p "$bind:464:464/udp" \
      -p "$bind:636:636/tcp" \
      -e ADMIN_PASSWORD="$ADMIN_PASSWORD" \
      -e AD_REALM="$realm" \
      -e AD_DOMAIN="$domain" \
      "$AD_IMAGE"
    echo "started; wait for: sudo $0 status"
    ;;
  status)
    podman exec "$name" samba-tool domain level show
    # Ask the DC's own DNS: the container's resolver is the host's, which knows
    # nothing about the throwaway zone.
    podman exec "$name" host -t SRV "_ldap._tcp.$dnsdomain" 127.0.0.1
    ;;
  forwarder)
    # A trust is validated from both sides. The IPA server reaches the DC
    # directly on the podman network, and the DC reaches the IPA server through
    # this forwarder. The two zones are siblings, so neither can rely on the
    # other being a parent.
    : "${AD_DNS_FORWARDER:?set AD_DNS_FORWARDER to the IPA server on the podman network}"
    podman exec -e AD_DNS_FORWARDER="$AD_DNS_FORWARDER" "$name" sh -c \
      'sed -i "s|^[[:space:]]*dns forwarder =.*|dns forwarder = $AD_DNS_FORWARDER 1.1.1.1|" /etc/samba/smb.conf
       grep -qE "^[[:space:]]*dns forwarder =" /etc/samba/smb.conf ||
         echo "dns forwarder = $AD_DNS_FORWARDER 1.1.1.1" >> /etc/samba/smb.conf'
    podman exec "$name" grep -n 'dns forwarder' /etc/samba/smb.conf
    podman restart "$name" >/dev/null
    echo "restarted with forwarder $AD_DNS_FORWARDER"
    ;;
  user)
    : "${AD_USER:?set AD_USER}"
    : "${AD_USER_PASSWORD:?set AD_USER_PASSWORD}"
    uid=${AD_USER_UID:-10000}
    gid=${AD_USER_GID:-10000}
    # The DC is a throwaway fixture with one generated password. Its default
    # complexity rule is for a real directory; turning it off lets the login
    # path use a lowercase password, which the VNC keyboard types without a
    # Caps Lock state.
    podman exec "$name" samba-tool domain passwordsettings set --complexity=off
    # The password goes in on stdin, never on the container's command line.
    if ! printf '%s\n%s\n' "$AD_USER_PASSWORD" "$AD_USER_PASSWORD" |
        podman exec -i "$name" samba-tool user add "$AD_USER" \
          --given-name="${AD_USER_FIRST:-DeskOS}" \
          --surname="${AD_USER_LAST:-QA}" \
          --uid-number="$uid" --gid-number="$gid" \
          --unix-home="/home/$AD_USER" \
          --login-shell=/bin/bash \
          --gecos="DeskOS QA" >/dev/null 2>&1; then
      printf '%s\n%s\n' "$AD_USER_PASSWORD" "$AD_USER_PASSWORD" |
        podman exec -i "$name" samba-tool user setpassword "$AD_USER" >/dev/null
    fi
    podman exec "$name" samba-tool user show "$AD_USER"
    ;;
  purge)
    podman rm -f "$name"
    ;;
  *)
    usage
    exit 2
    ;;
esac
