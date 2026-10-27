#!/usr/bin/env bash
# A disposable FreeIPA server for the identity fixture. Not production.
#
#   FREEIPA_IMAGE=freeipa/freeipa-server@sha256:… \
#   FREEIPA_PASSWORD=… sudo -E tests/identity/freeipa.sh start
#   sudo tests/identity/freeipa.sh status
#   FREEIPA_USER=qa FREEIPA_USER_PASSWORD=… sudo -E tests/identity/freeipa.sh user
#   sudo tests/identity/freeipa.sh stop     # keep the data volume
#   sudo tests/identity/freeipa.sh purge    # remove container and volume
set -euo pipefail

name=deskos-freeipa
volume=deskos-freeipa-data
hostname=${IPA_SERVER_HOSTNAME:-ipa.test}
domain=${IPA_SERVER_DOMAIN:-test}
realm=${IPA_SERVER_REALM:-TEST}

usage() {
  sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
}

case "${1:-}" in
  start)
    : "${FREEIPA_IMAGE:?set FREEIPA_IMAGE to a pinned server image}"
    : "${FREEIPA_PASSWORD:?set FREEIPA_PASSWORD for the throwaway Directory Manager}"
    podman run -d --name "$name" --hostname "$hostname" \
      --network host --privileged --systemd=always \
      -e PASSWORD="$FREEIPA_PASSWORD" \
      -e IPA_SERVER_HOSTNAME="$hostname" \
      -e IPA_SERVER_DOMAIN="$domain" \
      -e IPA_SERVER_REALM="$realm" \
      -v "$volume:/data:Z" "$FREEIPA_IMAGE"
    echo "started; wait for: sudo $0 status"
    ;;
  stop)
    podman rm -f "$name"
    ;;
  purge)
    podman rm -f "$name"
    podman volume rm "$volume"
    ;;
  status)
    podman exec "$name" ipactl status
    ;;
  user)
    : "${FREEIPA_USER:?set FREEIPA_USER}"
    : "${FREEIPA_USER_PASSWORD:?set FREEIPA_USER_PASSWORD}"
    printf '%s\n' "$FREEIPA_PASSWORD" | podman exec -i "$name" kinit admin
    if ! podman exec "$name" ipa user-add "$FREEIPA_USER" \
        --first="${FREEIPA_FIRST:-DeskOS}" --last="${FREEIPA_LAST:-QA}"; then
      podman exec "$name" ipa user-show "$FREEIPA_USER" >/dev/null
    fi
    printf '%s\n%s\n' "$FREEIPA_USER_PASSWORD" "$FREEIPA_USER_PASSWORD" \
      | podman exec -i "$name" ipa passwd "$FREEIPA_USER"
    ;;
  *)
    usage
    exit 2
    ;;
esac
