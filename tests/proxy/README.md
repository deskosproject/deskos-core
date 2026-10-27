# Corporate proxy and private CA fixture

A disposable forward proxy and an "intranet" site behind a private CA, for the
enterprise use-case checks. Not production, never published.

It exists to answer one question honestly: **can a DeskOS image be given a
corporate CA and reach internal sites through the proxy a company already has?**
The image is where the trust anchor lives; the proxy is the network's business.

## What runs

- **squid** on 3128: forward proxy, no cache, no authentication, no TLS
  interception. The client validates the origin certificate itself, which is what
  a proxy without TLS inspection does.
- **nginx** on 8080 (plain) and 8443 (TLS) serving `DESKOS-PROXY-OK`, with a
  certificate signed by the CA generated for the run.

`tests/proxy/proxy.sh ca` writes the CA and the server certificate into
`$PROXY_PKI`. The CA has to exist **before** the image is rendered, because the
same file becomes a `TrustAnchor` in the organization's resources; the script
only generates it and runs the services.

```bash
export PROXY_PKI=/tmp/proxy-pki
tests/proxy/proxy.sh ca
sudo podman build -t localhost/deskos-proxy:test tests/proxy
export PROXY_IMAGE=localhost/deskos-proxy:test
sudo -E tests/proxy/proxy.sh start
sudo tests/proxy/proxy.sh status
sudo tests/proxy/proxy.sh purge
```

## What it proves, and what it does not

- **Proves:** the intranet answers over TLS with the private CA, and the same
  request succeeds through the proxy.
- **Does not prove:** TLS inspection (`ssl_bump`), proxy authentication, PAC
  files, or that applications honour the desktop proxy settings — that is the
  session check's job, not this fixture's.
- **Does not model the proxy in the compiler.** `TrustAnchor` covers the CA and
  `GnomeProfile` covers the desktop settings; `dnf`, Flatpak and `deskos update`
  behind a proxy have no resource today. Whether the proxy is baseline (a new
  kind, with its own ADR) or configuration management's business is an open
  decision in the roadmap.

## Status

The fixture was validated on 2026-10-27 with podman on a workstation: plain and
TLS requests succeed through the proxy, and a request without the CA fails with
`unable to get local issuer certificate`. The workflow and the session
assertions are not written yet.
