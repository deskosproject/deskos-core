FROM quay.io/centos/centos:stream10@sha256:c4bda52c705aae497e690bc0190fb111f8ddd1ba939d57756ec33cf9bb11de6d

ENV container=podman LANG=C.UTF-8
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# BaseOS/AppStream de CentOS Stream 10; sin EPEL ni módulos de RHEL 8.
# ipa-server-dns incorpora las dependencias DNS; explicitamos el backend LDAP.
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

# Un target propio evita que ipa.service arranque a la vez que el bootstrap
# en reinicios. Conserva sysinit/basic (tmpfiles, sockets, D-Bus, etc.). Las
# unidades son ficheros y no un heredoc: buildah en los runners no expande
# heredocs en RUN (run 42415026037, "Unknown instruction: CAT").
COPY ipa-container.target ipa-container.service /etc/systemd/system/
RUN systemctl set-default ipa-container.target \
 && rm -f /etc/machine-id /var/lib/dbus/machine-id \
 && touch /etc/machine-id

# EXPOSE sólo documenta; el README usa una red privada sin publicar puertos.
EXPOSE 53/tcp 53/udp 80/tcp 443/tcp 389/tcp 636/tcp
EXPOSE 88/tcp 88/udp 464/tcp 464/udp
EXPOSE 135/tcp 445/tcp 49152-65535/tcp

STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/usr/local/sbin/entrypoint.sh"]
