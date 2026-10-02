#!/usr/bin/env bash
# Generates the vps-agent keys, stores the private halves as GitHub Environment secrets and
# deletes them locally. Only the public halves are written, to OUT_DIR (default ~/vps-agent-keys).
#
#   scripts/generate-keys.sh all     release signing key + deploy SSH key (first setup)
#   scripts/generate-keys.sh deploy  deploy SSH key only (safe rotation at any time)
#
# Outputs:
#   release-signing.pub  goes to internal/infrastructure/release/release-signing.pub, the literal
#                        key in scripts/install.sh and the README fingerprint, in one PR
#   deploy.pub           is DEPLOY_SSH_PUBLIC_KEY for `install.sh` / `vps-agent install` on each VPS
#
# Rotating the release key on hosts that already run vps-agent: a release must be signed by the
# key the installed binary embeds. Do NOT run `all` then; first ship a release signed with the
# current key that embeds the new public key, update every host, and only then replace
# RELEASE_SIGNING_KEY.
set -euo pipefail

readonly REPO="heronlabs/vps-agent"
readonly OPENSSL="${OPENSSL:-/opt/homebrew/opt/openssl@3/bin/openssl}"
readonly OUT_DIR="${OUT_DIR:-$HOME/vps-agent-keys}"
readonly MODE="${1:-}"

fail() { echo "error: $*" >&2; exit 1; }

case "$MODE" in
  all | deploy) ;;
  *) fail "usage: $0 <all | deploy>" ;;
esac

command -v ssh-keygen >/dev/null || fail "ssh-keygen not found"
command -v gh >/dev/null || fail "gh not found"
gh auth status >/dev/null 2>&1 || fail "gh is not logged in (gh auth login)"
gh api "repos/$REPO/environments/production" >/dev/null 2>&1 || fail "environment 'production' not found in $REPO"
if [ "$MODE" = "all" ]; then
  [ -x "$OPENSSL" ] || fail "OpenSSL 3 not found at $OPENSSL (brew install openssl@3; macOS LibreSSL has no ed25519)"
  gh api "repos/$REPO/environments/release" >/dev/null 2>&1 || fail "environment 'release' not found in $REPO"
fi

mkdir -p "$OUT_DIR"
umask 077

outputs=(deploy.pub)
[ "$MODE" = "all" ] && outputs+=(release-signing.pub)
for f in "${outputs[@]}"; do
  if [ -e "$OUT_DIR/$f" ]; then
    read -r -p "$OUT_DIR/$f exists and will be replaced. Continue? [y/N] " answer
    [ "$answer" = "y" ] || fail "aborted"
  fi
done

work="$(mktemp -d)"
cleanup() { rm -P -f "$work"/* 2>/dev/null || rm -f "$work"/*; rm -rf "$work"; }
trap cleanup EXIT

if [ "$MODE" = "all" ]; then
  echo "release signing key (ed25519)"
  "$OPENSSL" genpkey -algorithm ed25519 -out "$work/release.key"
  "$OPENSSL" pkey -in "$work/release.key" -pubout -out "$work/release-signing.pub"
  printf 'roundtrip\n' >"$work/msg"
  "$OPENSSL" pkeyutl -sign -inkey "$work/release.key" -rawin -in "$work/msg" -out "$work/msg.sig"
  "$OPENSSL" pkeyutl -verify -pubin -inkey "$work/release-signing.pub" -rawin -in "$work/msg" \
    -sigfile "$work/msg.sig" >/dev/null || fail "sign/verify roundtrip failed; nothing was stored"
  gh secret set RELEASE_SIGNING_KEY --env release --repo "$REPO" <"$work/release.key"
  mv "$work/release-signing.pub" "$OUT_DIR/release-signing.pub"
fi

echo "deploy SSH key (ssh-ed25519)"
ssh-keygen -q -t ed25519 -N "" -C vps-agent-deploy -f "$work/deploy"
gh secret set DEPLOY_SSH_KEY --env production --repo "$REPO" <"$work/deploy"
mv "$work/deploy.pub" "$OUT_DIR/deploy.pub"

chmod 644 "$OUT_DIR"/*.pub

echo
echo "Done. Private keys exist only in the GitHub secrets; local copies were deleted."
if [ "$MODE" = "all" ]; then
  echo
  echo "== $OUT_DIR/release-signing.pub =="
  cat "$OUT_DIR/release-signing.pub"
  echo "sha256: $(shasum -a 256 "$OUT_DIR/release-signing.pub" | cut -d' ' -f1)"
fi
echo
echo "== $OUT_DIR/deploy.pub (DEPLOY_SSH_PUBLIC_KEY) =="
cat "$OUT_DIR/deploy.pub"
echo "fingerprint: $(ssh-keygen -lf "$OUT_DIR/deploy.pub")"
