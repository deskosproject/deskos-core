# A disposable Samba AD DC for the identity fixture. Not a support claim.
#
# Debian trixie: `latest` at the time of writing (13.7), pinned by digest so a
# moved tag cannot change the fixture. The Samba packages come from the
# distribution at build time, so their exact version moves with Debian's
# repositories; the job records `samba --version` in the evidence, and this
# image is built in the run, never published.
FROM docker.io/library/debian:latest@sha256:8c4733a92aacd0bcff3779eadc35a4a5147f36cb20a5fcb56504cbc740212259

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      samba samba-ad-dc samba-ad-provision python3-samba smbclient winbind \
      krb5-user ldap-utils dnsutils iproute2 \
 && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/entrypoint.sh

EXPOSE 53/tcp 53/udp 88/tcp 88/udp 389/tcp 445/tcp 464/tcp 464/udp 636/tcp

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
