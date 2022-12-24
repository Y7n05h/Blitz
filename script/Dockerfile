FROM archlinux
COPY build/blitz /opt/bin/blitz
COPY build/blitzd /opt/bin/blitzd
COPY script/iptables-wrapper-installer.sh /
RUN /iptables-wrapper-installer.sh
