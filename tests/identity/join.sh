#!/usr/bin/env bash
# Join this guest to the disposable realm. Run as root inside the VM.
#
#   IPA_DOMAIN=lab.test IPA_REALM=LAB.TEST IPA_SERVER=ipa.lab.test \
#   IPA_PRINCIPAL=admin IPA_PASSWORD=… ./join.sh
set -euo pipefail

: "${IPA_DOMAIN:?}" "${IPA_REALM:?}" "${IPA_SERVER:?}" "${IPA_PRINCIPAL:?}" "${IPA_PASSWORD:?}"
# The realm serves its own DNS, reached through the guest's resolver set before
# this script. Check that the name resolves before the client tries SRV
# discovery; a host entry would hide a broken realm DNS.
getent hosts "$IPA_SERVER" >/dev/null 2>&1 || {
  echo "the realm DNS does not resolve $IPA_SERVER" >&2
  exit 1
}
ipa-client-install --unattended --force --hostname="${IPA_JOIN_HOSTNAME:-deskos-qa.$IPA_DOMAIN}" \
  --domain="$IPA_DOMAIN" --realm="$IPA_REALM" \
  --principal="$IPA_PRINCIPAL" --password="$IPA_PASSWORD" \
  --mkhomedir --no-ntp --force-join
