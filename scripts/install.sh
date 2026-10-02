#!/usr/bin/env bash
# Installs or updates vps-agent from the private GitHub release. Run as root.
# Keeps an existing /etc/vps-agent/config.env; writes it from the environment only on first install.
set -euo pipefail

readonly REPO="heronlabs/vps-agent"
readonly CONFIG_DIR="/etc/vps-agent"
readonly CONFIG_FILE="$CONFIG_DIR/config.env"
readonly BIN="/usr/local/bin/vps-agent"
readonly CONFIG_KEYS=(CMS_URL AGENT_TOKEN AGENT_HOST_NAME EASYPANEL_URL EASYPANEL_TOKEN
  AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION RECONCILE_INTERVAL REPORT_INTERVAL)

: "${GITHUB_TOKEN:?GITHUB_TOKEN is required (fine-grained, Contents: read on $REPO)}"
VERSION="${VERSION:-latest}"

fail() { echo "install: $*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "run as root"
command -v python3 >/dev/null || fail "python3 is required"

case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported architecture $(uname -m)" ;;
esac

api() {
  curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/$REPO/$1"
}

if [ "$VERSION" = "latest" ]; then
  release="$(api releases/latest)"
else
  release="$(api "releases/tags/v${VERSION#v}")"
fi

field() { python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1" <<<"$release"; }
asset_id() {
  python3 -c 'import json,sys; print(next((a["id"] for a in json.load(sys.stdin)["assets"] if a["name"] == sys.argv[1]), ""))' \
    "$1" <<<"$release"
}
download() {
  local id
  id="$(asset_id "$1")"
  [ -n "$id" ] || fail "asset $1 missing from $tag"
  curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/octet-stream" \
    "https://api.github.com/repos/$REPO/releases/assets/$id" -o "$2"
}

tag="$(field tag_name)"
version="${tag#v}"
binary="vps-agent_${version}_linux_${arch}"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
download "$binary" "$work/$binary"
download checksums.txt "$work/checksums.txt"
(cd "$work" && grep " $binary\$" checksums.txt | sha256sum -c --status -) ||
  fail "checksum mismatch for $binary; nothing installed"

install -m 0755 "$work/$binary" "$BIN"

if [ ! -f "$CONFIG_FILE" ]; then
  install -d -m 0700 "$CONFIG_DIR"
  (
    umask 077
    for key in "${CONFIG_KEYS[@]}"; do
      if [ -n "${!key:-}" ]; then
        printf '%s=%s\n' "$key" "${!key}"
      fi
    done >"$CONFIG_FILE"
  )
fi

echo "install: vps-agent $version at $BIN"
# exec replaces the shell, so the EXIT trap never runs on this path.
rm -rf "$work"
exec "$BIN" doctor
