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
# A single-label domain such as test is rejected by ipa-server-install; the
# fixture uses a two-label domain that cannot collide with a real one.
hostname=${IPA_SERVER_HOSTNAME:-ipa.lab.test}
domain=${IPA_SERVER_DOMAIN:-lab.test}
realm=${IPA_SERVER_REALM:-LAB.TEST}

usage() {
  sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
}

case "${1:-}" in
  start)
    : "${FREEIPA_IMAGE:?set FREEIPA_IMAGE to a pinned server image}"
    : "${FREEIPA_PASSWORD:?set FREEIPA_PASSWORD for the throwaway Directory Manager}"
    # -U is required: without it ipa-server-install prompts for the DNS and
    # domain questions, reads EOF from a detached container and fails. No
    # integrated DNS: the fixture realm is reached through /etc/hosts, and port
    # 53 on the host belongs to the host resolver. The log driver keeps the
    # whole install log, which is the evidence when the install fails.
    opts="-U -r $realm -n $domain --no-ntp --skip-mem-check"
    # The upstream image is documented for a plain (bridge) container, not for
    # host networking or privileged mode: privileged setup is explicitly not
    # supported. On the bridge the container resolves its own name to its own
    # address rather than to loopback, which is what the installer expects. The
    # guest reaches every published port through the host at 10.0.2.2, and the
    # server certificate still names the host.
    ports=(-p 80:80 -p 443:443 -p 389:389 -p 636:636
           -p 88:88 -p 88:88/udp -p 464:464 -p 464:464/udp)
    podman run -d --name "$name" --hostname "$hostname" --systemd=always \
      --log-driver=k8s-file --log-opt=max-size=50mb \
      "${ports[@]}" \
      -e PASSWORD="$FREEIPA_PASSWORD" \
      -e IPA_SERVER_INSTALL_OPTS="$opts" \
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
    state=$(podman inspect --format '{{.State.Status}}' "$name" 2>/dev/null || echo missing)
    if [ "$state" != running ]; then
      echo "container $name is $state" >&2
      exit 1
    fi
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
