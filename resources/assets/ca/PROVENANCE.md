# Distrusted CA provenance

Two public root certificates DeskOS does not trust, referenced only by
`resources/system/trust-distrusted-roots.yaml`. Both are vendor-shipped CA
certificates (public data, not secrets); the image puts them in the platform
trust store's **blocklist**, never in its anchors.

| File | Subject | Source | sha256 |
|---|---|---|---|
| `emsign-ecc-root-ca-g3.crt` | `CN=emSign ECC Root CA - G3` | NSS `certdata.txt` as built into `ca-certificates-2026.2.90_v9.0.316-100.0.el10`, extracted from `/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.crt` of the pinned CentOS Stream 10 base | `36e68e205b53c67c7a013894e0d5c8583063468118d1ce78ecbc2200d1dd185c` |
| `aaa-certificate-services.crt` | `CN=AAA Certificate Services` | NSS `certdata.txt` as built into `ca-certificates-2025.2.80_v9.0.305-102.el10_1` (the RHEL 10 / AlmaLinux 10 build of the base version), extracted from `ca-bundle.trust.p11-kit` of the `noarch` package | `a5ddabd1602ae1c66ce11ad078e734cc473dcb8e9f573037832d8536ae3de90b` |

Fingerprints (SHA-256, DER):

- `emSign ECC Root CA - G3`:
  `86:A1:EC:BA:08:9C:4A:8D:3B:BE:27:34:C6:12:BA:34:1D:81:3E:04:3C:F9:E8:A8:62:CD:5C:57:A3:6B:BE:6B`
- `AAA Certificate Services`:
  `D7:A7:A0:FB:5D:7E:27:31:D7:71:E9:48:4E:BC:DE:F7:1D:5F:0C:3E:0A:29:48:78:2B:C8:3E:E0:EA:69:9E:F4`

`Comodo AAA Services root` is the same certificate under the label Mozilla
used; one entry covers both.
