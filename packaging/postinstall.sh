#!/bin/sh
set -eu
if ! id talon-deploy >/dev/null 2>&1; then
  useradd --system --create-home --home-dir /var/lib/talon-deploy --shell /bin/sh talon-deploy
fi
usermod -p '*' talon-deploy
