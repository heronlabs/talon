#!/usr/bin/env bash
# Bootstrap for the first install of vps-agent and for the migration from 0.1.0. Run as root.
# Downloads a release, checks that checksums.txt is signed by the heronlabs release key and that
# the binary's sha256 matches, then hands over to `vps-agent install`, which inherits this
# environment (GITHUB_TOKEN, AGENT_HOST_NAME, DEPLOY_SSH_PUBLIC_KEY, ...). After this,
# `sudo vps-agent update` installs new releases.
set -euo pipefail

readonly REPO="heronlabs/vps-agent"
readonly API="https://api.github.com/repos/$REPO"
readonly MIN_VERSION="0.2.0"
# The release signing key (ed25519, PEM). A Go test keeps it equal to
# internal/infrastructure/release/release-signing.pub.
readonly RELEASE_PUBLIC_KEY='-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAOl0PNj3bVBcVqEsJwekZYdadvp79VsLZzgoDbeY2n7k=
-----END PUBLIC KEY-----
'

fail() { echo "install: $*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "run as root"
command -v python3 >/dev/null || fail "python3 is required"
command -v openssl >/dev/null || fail "openssl is required"
[ -n "${GITHUB_TOKEN:-}" ] || fail "GITHUB_TOKEN is required (fine-grained, Contents: read on $REPO)"

case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported architecture $(uname -m)" ;;
esac

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
chmod 0700 "$work"

# The token reaches curl through a 0600 header file, never through argv.
(umask 077 && printf 'Authorization: Bearer %s\n' "$GITHUB_TOKEN" >"$work/auth")
printf '%s' "$RELEASE_PUBLIC_KEY" >"$work/release-signing.pub"

api() {
  curl -fsSL -H @"$work/auth" -H "Accept: application/vnd.github+json" "$API/$1"
}

VERSION="${VERSION:-latest}"
if [ "$VERSION" = "latest" ]; then
  release="$(api releases/latest)" || fail "cannot read the latest release"
else
  release="$(api "releases/tags/v${VERSION#v}")" || fail "cannot read release v${VERSION#v}"
fi

tag="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["tag_name"])' <<<"$release")"
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "unexpected release tag"
version="${tag#v}"
python3 - "$version" "$MIN_VERSION" <<'PY' || fail "version $version is older than $MIN_VERSION, the first one this script installs"
import sys
version, minimum = (tuple(int(p) for p in v.split(".")) for v in sys.argv[1:3])
sys.exit(0 if version >= minimum else 1)
PY

binary="vps-agent_${version}_linux_${arch}"

download() {
  local id
  id="$(python3 -c 'import json,sys; print(next((a["id"] for a in json.load(sys.stdin)["assets"] if a["name"] == sys.argv[1]), ""))' \
    "$1" <<<"$release")"
  [ -n "$id" ] || fail "release $tag is incomplete: $1 is missing"
  curl -fsSL -H @"$work/auth" -H "Accept: application/octet-stream" "$API/releases/assets/$id" -o "$work/$1" ||
    fail "cannot download $1"
}

download "$binary"
download checksums.txt
download checksums.txt.sig

openssl pkeyutl -verify -pubin -inkey "$work/release-signing.pub" -rawin \
  -in "$work/checksums.txt" -sigfile "$work/checksums.txt.sig" >/dev/null 2>&1 ||
  fail "bad signature on checksums.txt of $tag; nothing installed"
(cd "$work" && awk -v name="$binary" 'NF == 2 && $2 == name' checksums.txt | sha256sum -c --status -) ||
  fail "checksum mismatch for $binary; nothing installed"

chmod 0755 "$work/$binary"
code=0
"$work/$binary" install || code=$?
exit "$code"
