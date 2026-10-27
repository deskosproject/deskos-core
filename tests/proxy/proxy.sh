#!/usr/bin/env bash
# A disposable corporate proxy and a private CA, for one test. Not production.
#
#   PROXY_PKI=/tmp/proxy-pki tests/proxy/proxy.sh ca      # write the CA and certs
#   PROXY_IMAGE=localhost/deskos-proxy:test PROXY_PKI=… sudo -E tests/proxy/proxy.sh start
#   sudo tests/proxy/proxy.sh status
#   sudo tests/proxy/proxy.sh purge
#
# The CA has to exist before the image is rendered: the same certificate becomes
# a TrustAnchor in the organization's resources. This script only generates it
# and runs the services; how the image is told to trust it is a Plan input.
set -euo pipefail

name=deskos-proxy
pki=${PROXY_PKI:?set PROXY_PKI to the directory holding ca.crt, server.crt and server.key}
hostname=${PROXY_HOSTNAME:-intranet.corp.test}
# QEMU user networking exposes the host's loopback as 10.0.2.2.
bind=${PROXY_BIND:-127.0.0.1}

usage() {
  sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
}

case "${1:-}" in
  ca)
    mkdir -p "$pki"
    if [ ! -s "$pki/ca.crt" ]; then
      openssl req -x509 -newkey rsa:4096 -nodes -days 2 \
        -keyout "$pki/ca.key" -out "$pki/ca.crt" \
        -subj "/CN=DeskOS test corporate CA" >/dev/null 2>&1
      openssl req -newkey rsa:2048 -nodes \
        -keyout "$pki/server.key" -out "$pki/server.csr" \
        -subj "/CN=$hostname" >/dev/null 2>&1
      printf 'subjectAltName=DNS:%s\nbasicConstraints=CA:FALSE\n' "$hostname" > "$pki/server.ext"
      openssl x509 -req -in "$pki/server.csr" -CA "$pki/ca.crt" -CAkey "$pki/ca.key" \
        -CAcreateserial -days 2 -extfile "$pki/server.ext" -out "$pki/server.crt" >/dev/null 2>&1
    fi
    echo "CA and server certificate in $pki"
    ;;
  start)
    : "${PROXY_IMAGE:?set PROXY_IMAGE to the built fixture image}"
    podman run -d --name "$name" --hostname "$hostname" \
      -p "$bind:3128:3128" -p "$bind:8080:8080" -p "$bind:8443:8443" \
      -v "$pki:/pki:ro,Z" \
      "$PROXY_IMAGE"
    echo "started; wait for: sudo $0 status"
    ;;
  status)
    # The intranet answers over TLS with the private CA, and the proxy is the
    # only path a corporate network would allow: check both.
    curl -fsS --cacert "$pki/ca.crt" "https://$hostname:8443/" >/dev/null
    curl -fsS -x "http://$bind:3128" --cacert "$pki/ca.crt" "https://$hostname:8443/" | grep -q DESKOS-PROXY-OK
    echo "intranet answers directly and through the proxy"
    ;;
  purge)
    podman rm -f "$name"
    ;;
  *)
    usage
    exit 2
    ;;
esac
