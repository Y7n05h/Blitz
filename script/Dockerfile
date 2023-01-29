FROM alpine:latest
RUN apk add --no-cache iproute2 net-tools ca-certificates iptables && update-ca-certificates
COPY build/blitz build/blitzd /opt/bin/
COPY script/iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh
