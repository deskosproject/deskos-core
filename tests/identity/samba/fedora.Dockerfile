# A disposable Samba AD DC for the identity fixture. Not a support claim.
#
# Fedora 45 rather than Debian: the DC is the *partner* side of a FreeIPA trust,
# and the trust path that failed (a cross-realm TGS request rejected at the
# authorization-data layer, "HANDLE_AUTHDATA") is Samba's code. Testing it on the
# same distribution family the client and the IPA server run is one less
# difference to explain; it is still Samba, not the Windows Server Red Hat
# supports as a trust partner.
#
# The base is pinned by digest, and the Samba packages come from the
# distribution at build time, so their exact version moves with Fedora's
# repositories; the job records `samba --version` in the evidence, and this
# image is built in the run, never published.
FROM registry.fedoraproject.org/fedora:45@sha256:e736b5d06f96f7560b82d106444cb00eb5dd735f30b2c15e575213a218810e18

RUN dnf install -y --setopt=install_weak_deps=False \
      samba samba-dc samba-client samba-common-tools \
      python3-samba krb5-workstation bind-utils iproute \
 && dnf clean all

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/entrypoint.sh

EXPOSE 53/tcp 53/udp 88/tcp 88/udp 389/tcp 445/tcp 464/tcp 464/udp 636/tcp

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
