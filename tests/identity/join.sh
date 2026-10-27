#!/usr/bin/env bash
# Join this guest to the disposable realm. Run as root inside the VM.
#
#   IPA_DOMAIN=test IPA_REALM=TEST IPA_SERVER=ipa.test \
#   IPA_PRINCIPAL=admin IPA_PASSWORD=… ./join.sh
set -euo pipefail

: "${IPA_DOMAIN:?}" "${IPA_REALM:?}" "${IPA_SERVER:?}" "${IPA_PRINCIPAL:?}" "${IPA_PASSWORD:?}"
# QEMU user networking exposes the host as 10.0.2.2. The server certificate names
# the host, so the guest resolves that name to the host and not to a public DNS.
if ! grep -qE "[[:space:]]${IPA_SERVER//./\\.}([[:space:]]|$)" /etc/hosts; then
  printf '10.0.2.2 %s\n' "$IPA_SERVER" >> /etc/hosts
fi
ipa-client-install --unattended --force --hostname="${IPA_JOIN_HOSTNAME:-deskos-qa.$IPA_DOMAIN}" \
  --domain="$IPA_DOMAIN" --realm="$IPA_REALM" --server="$IPA_SERVER" \
  --principal="$IPA_PRINCIPAL" --password="$IPA_PASSWORD" \
  --mkhomedir --no-ntp --force-join
