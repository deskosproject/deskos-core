# A corporate proxy and a private CA, for one test. Not production.
#
# Squid is the forward proxy a corporate network puts in front of everything;
# nginx answers over TLS with a certificate this run's private CA signed. The
# point is not the software: it is that a DeskOS image can be given that CA as a
# trust anchor and can reach an internal site only through the proxy.
FROM docker.io/library/debian:latest@sha256:4c25275446ca371766b8b093c696782eabce06263fccacb8d5d813b3dc44b308

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
 && apt-get install -y --no-install-recommends squid nginx-light openssl curl \
 && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY squid.conf /etc/squid/squid.conf
COPY nginx.conf /etc/nginx/sites-available/proxy-fixture
RUN chmod 0755 /usr/local/bin/entrypoint.sh \
 && rm -f /etc/nginx/sites-enabled/default \
 && ln -s /etc/nginx/sites-available/proxy-fixture /etc/nginx/sites-enabled/proxy-fixture

EXPOSE 3128/tcp 8080/tcp 8443/tcp

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
