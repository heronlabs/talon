#!/bin/sh
# Last steps of https://get.easypanel.io, which refuses to run inside a container.
set -eu

docker swarm leave --force >/dev/null 2>&1 || true
docker pull "easypanel/easypanel:${EASYPANEL_VERSION}"
docker run --rm -i \
  -v /etc/easypanel:/etc/easypanel \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  "easypanel/easypanel:${EASYPANEL_VERSION}" setup
touch /etc/easypanel/.fake-vps-setup-done
