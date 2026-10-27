#!/bin/bash
# Provision a throwaway AD domain on first start, then run Samba in the
# foreground as PID 1.
#
# --use-rfc2307 is what makes this usable by Linux clients: it tells Samba to
# store uidNumber, gidNumber, loginShell and homeDirectory in the directory, and
# sssd maps the Unix identity from them. Without it the join succeeds and no
# user can log in.
set -euo pipefail

realm=${AD_REALM:-AD.LAB.TEST}
domain=${AD_DOMAIN:-AD}
: "${ADMIN_PASSWORD:?set ADMIN_PASSWORD for the throwaway Administrator}"

if [ ! -f /var/lib/samba/private/krb5.conf ]; then
  echo "samba: provisioning $realm"
  # Debian ships a standalone smb.conf; provision refuses to guess names while
  # a server role it did not choose is configured.
  rm -f /etc/samba/smb.conf
  samba-tool domain provision \
    --realm="$realm" \
    --domain="$domain" \
    --adminpass="$ADMIN_PASSWORD" \
    --server-role=dc \
    --dns-backend=SAMBA_INTERNAL \
    --use-rfc2307 \
    --option="dns forwarder = 1.1.1.1"
  cp /var/lib/samba/private/krb5.conf /etc/krb5.conf
fi

echo "samba: starting"
exec samba --foreground --no-process-group
