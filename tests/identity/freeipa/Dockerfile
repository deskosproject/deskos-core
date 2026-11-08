FROM quay.io/centos/centos:stream10@sha256:c4bda52c705aae497e690bc0190fb111f8ddd1ba939d57756ec33cf9bb11de6d

ENV container=podman LANG=C.UTF-8
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# CentOS Stream 10 BaseOS/AppStream: no EPEL and no RHEL 8 module streams.
# ipa-server-dns brings the DNS dependencies; the LDAP backend is explicit.
RUN dnf -y install \
        systemd dbus-broker \
        ipa-server ipa-server-dns ipa-server-trust-ad \
        bind-dyndb-ldap samba-client ipa-healthcheck \
        krb5-workstation bind-utils hostname iproute procps-ng \
    && dnf clean all \
    && rm -rf /var/cache/dnf \
    && rpm -qa | sort > /usr/share/ipa-ci-packages.txt

COPY entrypoint.sh /usr/local/sbin/entrypoint.sh
RUN chmod 0755 /usr/local/sbin/entrypoint.sh

# A target of its own keeps ipa.service from starting beside the bootstrap on
# a restart. It keeps sysinit/basic (tmpfiles, sockets, D-Bus). The units are
# files and not a heredoc: the runner's buildah does not expand a RUN heredoc
# (run 42415026037, "Unknown instruction: CAT").
COPY ipa-container.target ipa-container.service /etc/systemd/system/
RUN systemctl set-default ipa-container.target \
 && rm -f /etc/machine-id /var/lib/dbus/machine-id \
 && touch /etc/machine-id

# EXPOSE documents only; the fixture uses a private network and publishes
# the loopback ports it needs from the run script, not from the image.
EXPOSE 53/tcp 53/udp 80/tcp 443/tcp 389/tcp 636/tcp
EXPOSE 88/tcp 88/udp 464/tcp 464/udp
EXPOSE 135/tcp 445/tcp 49152-65535/tcp

STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/usr/local/sbin/entrypoint.sh"]
