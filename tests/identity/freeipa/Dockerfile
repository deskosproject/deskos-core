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
# en reinicios. Conserva sysinit/basic (tmpfiles, sockets, D-Bus, etc.).
RUN <<'BUILD'
cat > /etc/systemd/system/ipa-container.target <<'UNIT'
[Unit]
Description=FreeIPA CI container
Requires=basic.target
Wants=network.target ipa-container.service
After=basic.target
AllowIsolate=yes
UNIT
cat > /etc/systemd/system/ipa-container.service <<'UNIT'
[Unit]
Description=Install or start the FreeIPA CI fixture
Requires=dbus.socket
After=network.target dbus.socket systemd-tmpfiles-setup.service
FailureAction=exit-force

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/entrypoint.sh --configure
RemainAfterExit=yes
TimeoutStartSec=1800
UMask=0022
StandardOutput=journal+console
StandardError=journal+console
UNIT
systemctl set-default ipa-container.target
rm -f /etc/machine-id /var/lib/dbus/machine-id
touch /etc/machine-id
BUILD

# EXPOSE sólo documenta; el README usa una red privada sin publicar puertos.
EXPOSE 53/tcp 53/udp 80/tcp 443/tcp 389/tcp 636/tcp
EXPOSE 88/tcp 88/udp 464/tcp 464/udp
EXPOSE 135/tcp 445/tcp 49152-65535/tcp

STOPSIGNAL SIGRTMIN+3
ENTRYPOINT ["/usr/local/sbin/entrypoint.sh"]
