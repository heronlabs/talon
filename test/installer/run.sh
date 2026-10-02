#!/usr/bin/env bash
# Tests scripts/install.sh in ubuntu:24.04 containers against a fake GitHub API (a fake curl) that
# serves a release of this checkout signed by a throwaway key. Needs Go and Docker.
# Usage: test/installer/run.sh [scenario ...]   (default: every scenario)
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly repo
readonly image=vps-agent-installer-test
readonly version=0.2.0

scenarios=("$@")
[ "${#scenarios[@]}" -gt 0 ] || scenarios=(fresh migration bad-signature bad-checksum deploy-user)

for arch in amd64 arm64; do
  make -C "$repo" --no-print-directory build GOOS=linux GOARCH="$arch" VERSION="$version" \
    BIN="dist/installer/vps-agent_${version}_linux_${arch}"
done
docker build --quiet --tag "$image" "$repo/test/installer" >/dev/null

failed=0
for scenario in "${scenarios[@]}"; do
  # No network: the only GitHub is the fake curl, and `update` cannot reach the real one.
  docker run --rm --network none --volume "$repo:/src:ro" "$image" \
    bash /src/test/installer/scenario.sh "$scenario" || failed=$((failed + 1))
done

echo "installer: $((${#scenarios[@]} - failed))/${#scenarios[@]} scenarios passed"
[ "$failed" -eq 0 ]
