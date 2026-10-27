#!/bin/bash
# Serve the intranet and the proxy. The certificates come from the run, mounted
# at /pki, because the same CA has to reach the image as a trust anchor.
set -euo pipefail

for f in /pki/ca.crt /pki/server.crt /pki/server.key; do
  [ -s "$f" ] || { echo "missing $f" >&2; exit 1; }
done

# The proxy resolves the intranet itself, so the name must point at this
# container. Podman also writes the container hostname into /etc/hosts, which
# here maps the same name to the host's address: replace that line, do not add
# beside it.
name=$(awk '/server_name/ {print $2}' /etc/nginx/sites-available/proxy-fixture | head -n1 | tr -d ';')
if [ -n "$name" ]; then
  awk -v n="$name" '$2 != n' /etc/hosts > /etc/hosts.new && cat /etc/hosts.new > /etc/hosts
  printf '127.0.0.1 %s\n' "$name" >> /etc/hosts
fi

nginx -g 'daemon off;' &
exec squid -N -d1
