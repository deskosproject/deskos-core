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
    # domain questions, reads EOF from a detached container and fails. The
    # realm serves its own DNS, so the client discovers the server through SRV
    # records instead of a host entry. No forwarders: the guest only needs this
    # zone, and a forwarder would make the fixture depend on the runner's
    # resolver. The log driver keeps the whole install log, the evidence when
    # the install fails.
    opts="-U -r $realm -n $domain --setup-dns --no-forwarders --no-ntp --skip-mem-check"
    # The upstream image is documented for a plain (bridge) container, not for
    # host networking or privileged mode: privileged setup is explicitly not
    # supported. On the bridge the container resolves its own name to its own
    # address rather than to loopback, which is what the installer expects. The
    # guest reaches every published port through the host at 10.0.2.2, and the
    # server certificate still names the host. IPA_BIND lets a second directory
    # (the AD trust fixture) own the default loopback ports.
    bind=${IPA_BIND:-127.0.0.1}
    ports=(-p "$bind:53:53/udp" -p "$bind:53:53/tcp"
           -p "$bind:80:80" -p "$bind:443:443"
           -p "$bind:389:389" -p "$bind:636:636"
           -p "$bind:88:88" -p "$bind:88:88/udp"
           -p "$bind:464:464" -p "$bind:464:464/udp")
    network=${IPA_NETWORK:-podman}
    ip=${IPA_IP:-}
    ip_args=()
    [ -n "$ip" ] && ip_args=(--ip "$ip")
    podman run -d --name "$name" --hostname "$hostname" --systemd=always \
      --network "$network" "${ip_args[@]}" \
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
    # The realm's own DNS must answer, not the host resolver.
    podman exec "$name" host -t SRV "_ldap._tcp.$domain" "$hostname"
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
    # An administrator-set password is expired by policy: the greeter would
    # authenticate and then demand a change. This check types the password and
    # expects a session, so move the expiry past the run.
    podman exec "$name" ipa user-mod "$FREEIPA_USER" --password-expiration=20301231000000Z
    ;;
  dns)
    # The installer publishes the A record with the container's bridge address.
    # The guest reaches the published loopback through QEMU user networking at
    # 10.0.2.2, so clients need that address in the zone. The server keeps its
    # own name in /etc/hosts and is not affected.
    : "${IPA_PUBLIC_IP:?set IPA_PUBLIC_IP to the address clients reach}"
    printf '%s\n' "$FREEIPA_PASSWORD" | podman exec -i "$name" kinit admin
    if ! podman exec "$name" ipa dnsrecord-mod "$domain" "${hostname%%.*}" \
        --a-rec="$IPA_PUBLIC_IP"; then
      podman exec "$name" ipa dnsrecord-add "$domain" "${hostname%%.*}" \
        --a-rec="$IPA_PUBLIC_IP"
    fi
    podman exec "$name" host -t A "$hostname" "$hostname"
    ;;
  trust)
    # Establish a one-way trust to an AD domain, then prove that the IPA
    # server's SSSD resolves an AD user. AD_SERVER_IP is the DC on the podman
    # network, not the address the guest would use: only the IPA server talks
    # to the DC.
    : "${AD_DOMAIN:?set AD_DOMAIN}"
    : "${AD_SERVER_IP:?set AD_SERVER_IP to the DC on the podman network}"
    : "${AD_ADMIN_PASSWORD:?set AD_ADMIN_PASSWORD}"
    : "${AD_USER:?set AD_USER}"
    printf '%s\n' "$FREEIPA_PASSWORD" | podman exec -i "$name" kinit admin
    # The trust component is a separate package and setup step. A fresh IPA
    # server can serve its own clients without it, and trust-add fails without
    # it, so confirm the command exists before using it.
    podman exec "$name" bash -lc \
      'command -v ipa-adtrust-install >/dev/null || \
         dnf -y install ipa-server-trust-ad || \
         dnf -y install freeipa-server-trust-ad'
    podman exec "$name" ipa-adtrust-install --unattended --netbios-name=LAB \
      -a "$FREEIPA_PASSWORD"
    # The installer restarts services; a ticket obtained before it can be stale
    # when the next command reaches the API.
    printf '%s\n' "$FREEIPA_PASSWORD" | podman exec -i "$name" kinit admin
    # Only the AD zone goes to the DC. A global forwarder would send every
    # unknown name there; the conditional forward zone is the documented shape
    # for a trust when the two zones are not children of one another.
    if ! podman exec "$name" ipa dnsforwardzone-show "$AD_DOMAIN" >/dev/null 2>&1; then
      podman exec "$name" ipa dnsforwardzone-add "$AD_DOMAIN" \
        --forwarder="$AD_SERVER_IP" --forward-policy=only
    fi
    podman exec "$name" dig +short SRV "_ldap._tcp.$AD_DOMAIN" @127.0.0.1
    # `--password` is a prompt, not an option with a value: passing
    # --password=... makes ipa exit with "--password option does not take a
    # value" (run 41742726895). Feed the prompt on stdin instead.
    printf '%s\n' "$AD_ADMIN_PASSWORD" | podman exec -i "$name" ipa trust-add "$AD_DOMAIN" --type=ad \
      --admin Administrator --password \
      --range-type=ipa-ad-trust
    podman exec "$name" ipa trust-show "$AD_DOMAIN"
    podman exec "$name" ipa idrange-find
    # The IPA server's SSSD resolves the trusted user; wait for the domain to
    # come online before a client asks for the account.
    for _ in $(seq 1 60); do
      if podman exec "$name" getent passwd "$AD_USER@$AD_DOMAIN" >/dev/null 2>&1; then
        break
      fi
      sleep 5
    done
    podman exec "$name" getent passwd "$AD_USER@$AD_DOMAIN"
    # The user can resolve before the domain is online, and a client that asks
    # now can cache an offline answer. Wait for the domain, restarting SSSD when
    # it came up before the trust did.
    for _ in $(seq 1 30); do
      if podman exec "$name" sssctl domain-status "$AD_DOMAIN" 2>/dev/null |
          grep -q 'Online status: Online'; then
        break
      fi
      podman exec "$name" systemctl restart sssd || true
      sleep 5
    done
    podman exec "$name" sssctl domain-status "$AD_DOMAIN"
    ;;
  *)
    usage
    exit 2
    ;;
esac
