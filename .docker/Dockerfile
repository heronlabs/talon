FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY Makefile nfpm.yaml version.txt ./
COPY src ./src
COPY packaging ./packaging
# The same package the release workflow builds, for this machine's architecture.
RUN make package VERSION="$(cat version.txt)" \
 && mkdir /out && cp dist/talon_*.deb /out/talon.deb

# Fake VPS: Ubuntu 22.04 with systemd as PID 1, Docker Engine and Easypanel.
# Easypanel is installed on first boot by easypanel-setup.service, because its
# setup needs a running dockerd. Run with .docker/compose.yaml (privileged).
FROM ubuntu:22.04
ENV container=docker DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends systemd systemd-sysv dbus ca-certificates curl iptables lsof \
 && install -m 0755 -d /etc/apt/keyrings \
 && curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc \
 && echo "deb [signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu jammy stable" > /etc/apt/sources.list.d/docker.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io \
 && rm -rf /var/lib/apt/lists/* \
 && systemctl mask getty.target systemd-logind.service systemd-resolved.service \
 && systemctl enable docker.service
ARG EASYPANEL_VERSION=latest
RUN echo "EASYPANEL_VERSION=${EASYPANEL_VERSION}" > /etc/default/easypanel-setup
COPY .docker/fake_vps/easypanel_setup.sh /usr/local/sbin/easypanel-setup
COPY .docker/fake_vps/easypanel_setup.service /etc/systemd/system/easypanel-setup.service
RUN systemctl enable easypanel-setup.service
# talon is installed from its .deb, as on a real host: /usr/bin/talon and /etc/talon.
# compose.yaml mounts bin/linux at /usr/local/bin, so the binary `make watch`
# builds shadows the packaged one, and .docker/config at /etc/talon.
COPY --from=build /out/talon.deb /root/talon.deb
RUN dpkg -i /root/talon.deb && rm /root/talon.deb
WORKDIR /root
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
