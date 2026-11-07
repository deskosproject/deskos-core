#!/usr/bin/env bash
# A disposable FreeIPA server for the identity fixture. Not production.
#
#   FREEIPA_IMAGE=localhost/deskos-freeipa:ci \
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
    : "${FREEIPA_IMAGE:?set FREEIPA_IMAGE to the built server image}"
    : "${FREEIPA_PASSWORD:?set FREEIPA_PASSWORD for the throwaway Directory Manager}"
    # CentOS Stream 10 server image, built from tests/identity/freeipa. The
    # entrypoint installs the server on first boot inside systemd and writes
    # /run/ipa-ready; the wait below asks for that file, so a failed install is
    # reported rather than waited out.
    #
    # The realm serves its own DNS, so the client discovers the server through
    # SRV records instead of a host entry. No forwarders: the guest only needs
    # this zone, and a forwarder would make the fixture depend on the runner's
    # resolver. The trust adds a conditional forward zone for the AD domain.
    #
    # The disposable .test realms are not DNSSEC-signed; validation rejects the
    # forwarded SOA, which leaves trust-add without domain information (run
    # 42109140793). The image installs with --no-dnssec-validation.
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
    podman run -d --name "$name" --hostname "$hostname" \
      --privileged --systemd=always --cgroupns=private \
      --tmpfs /run:rw,nosuid,nodev,mode=0755 \
      --tmpfs /run/lock:rw,nosuid,nodev,mode=0755 \
      --tmpfs /tmp:rw,nosuid,nodev,mode=1777 \
      --shm-size=256m --pids-limit=4096 --ulimit nofile=65536:65536 \
      --stop-timeout=120 \
      --network "$network" "${ip_args[@]}" \
      --log-driver=k8s-file --log-opt=max-size=50mb \
      "${ports[@]}" \
      -e "IPA_SERVER_HOSTNAME=$hostname" \
      -e "IPA_SERVER_DOMAIN=$domain" \
      -e "IPA_SERVER_REALM=$realm" \
      -e "IPA_SERVER_IP=$ip" \
      -e "IPA_SERVER_PASSWORD=$FREEIPA_PASSWORD" \
      "$FREEIPA_IMAGE"
    echo "started; wait for: sudo $0 status"
    ;;
  stop)
    podman rm -f "$name"
    ;;
  purge)
    # The data lives in the container: the server is disposable, and an image
    # that installs on first boot has no volume to carry between runs.
    podman rm -f "$name"
    ;;
  status)
    state=$(podman inspect --format '{{.State.Status}}' "$name" 2>/dev/null || echo missing)
    if [ "$state" != running ]; then
      echo "container $name is $state" >&2
      exit 1
    fi
    # The image installs the server on first boot; until the entrypoint writes
    # /run/ipa-ready the services are being configured, and ipactl would fail
    # for the wrong reason.
    podman exec "$name" test -f /run/ipa-ready
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
    # The AD zone is not signed. Validation rejects the forwarded SOA and
    # trust-add cannot read the domain. Red Hat documents the switch for
    # RHEL 8.2+ in /etc/named/ipa-options-ext.conf (solution 1400113); this
    # image is RHEL 9 with BIND 9.16, and `ipa dnsconfig-mod` has no
    # --dnssec-validation (run 42116859715). Append or replace the directive
    # after ipa-adtrust-install, then reload named.
    podman exec "$name" bash -lc '
      f=/etc/named/ipa-options-ext.conf
      touch "$f"
      if grep -q "^[[:space:]]*dnssec-validation" "$f"; then
        sed -i "s/^[[:space:]]*dnssec-validation .*/dnssec-validation no;/" "$f"
      else
        echo "dnssec-validation no;" >> "$f"
      fi
      systemctl restart named
      grep -n "dnssec-validation" "$f"
    '
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
    # Keep what trust-add says: it reports the cross-realm principals it
    # created, and a trust that "succeeds" without them leaves clients with
    # nothing to ask their KDC for (run 42396135847).
    printf '%s\n' "$AD_ADMIN_PASSWORD" | podman exec -i "$name" ipa trust-add "$AD_DOMAIN" --type=ad \
      --admin Administrator --password \
      --range-type=ipa-ad-trust 2>&1 | tee /tmp/trust-add.txt
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
