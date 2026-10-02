#!/usr/bin/env bash
# Runs one install.sh scenario inside the test container (see run.sh). The release is signed by a
# throwaway key, and the script under test is a copy of scripts/install.sh carrying that key.
# Usage: scenario.sh <fresh | migration | bad-signature | bad-checksum | deploy-user>
set -euo pipefail

readonly SRC=/src
readonly VERSION=0.2.0
readonly TOKEN=test-token-do-not-use
readonly DEPLOY_KEY_BLOB=AAAAC3NzaC1lZDI1NTE5AAAAIMJO6hd9vIfa3qb8Vilon7lkFnaxu6wTEdEZpWfxv+vn
readonly CONFIG=/etc/vps-agent/config.env
readonly BIN=/usr/local/bin/vps-agent

scenario="${1:?scenario name}"
work="$(mktemp -d)"
fixture="$work/release"
out="$work/out"
err="$work/err"

fail() {
  echo "FAIL $scenario: $*" >&2
  echo "--- stdout" >&2; cat "$out" >&2 2>/dev/null || true
  echo "--- stderr" >&2; cat "$err" >&2 2>/dev/null || true
  exit 1
}
expect_eq() { [ "$2" = "$3" ] || fail "$1: want [$2], got [$3]"; }
expect_contains() { grep -qF -- "$2" "$3" || fail "$1: [$2] not in $3"; }

case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported architecture $(uname -m)" ;;
esac
binary="vps-agent_${VERSION}_linux_${arch}"

# make_release <signing key>: the assets, their checksums signed by the given key, release.json.
make_release() {
  mkdir -p "$fixture/assets" "$work/bin"
  cp "$SRC/dist/installer/vps-agent_${VERSION}_linux_amd64" "$SRC/dist/installer/vps-agent_${VERSION}_linux_arm64" "$work/bin/"
  (cd "$work/bin" && sha256sum vps-agent_*) >"$fixture/assets/3"
  openssl pkeyutl -sign -inkey "$1" -rawin -in "$fixture/assets/3" -out "$fixture/assets/4"
  cp "$work/bin/vps-agent_${VERSION}_linux_amd64" "$fixture/assets/1"
  cp "$work/bin/vps-agent_${VERSION}_linux_arm64" "$fixture/assets/2"
  cat >"$fixture/release.json" <<JSON
{"tag_name": "v$VERSION", "assets": [
  {"id": 1, "name": "vps-agent_${VERSION}_linux_amd64"},
  {"id": 2, "name": "vps-agent_${VERSION}_linux_arm64"},
  {"id": 3, "name": "checksums.txt"},
  {"id": 4, "name": "checksums.txt.sig"}]}
JSON
}

# The script under test: scripts/install.sh with the test public key in place of the real one.
openssl genpkey -algorithm ed25519 -out "$work/test.key"
openssl pkey -in "$work/test.key" -pubout -out "$work/test.pub"
real_line="$(sed -n 2p "$SRC/internal/infrastructure/release/release-signing.pub")"
test_line="$(sed -n 2p "$work/test.pub")"
sed "s|^${real_line}\$|${test_line}|" "$SRC/scripts/install.sh" >"$work/install.sh"
grep -qxF "$test_line" "$work/install.sh" || fail "the test key was not swapped into install.sh"

# A Docker engine that answers, so the doctor run by `vps-agent install` is green.
python3 "$SRC/test/installer/fake-docker.py" /var/run/docker.sock &
for _ in 1 2 3 4 5 6 7 8 9 10; do [ -S /var/run/docker.sock ] && break; sleep 0.2; done

# run_install [VAR=value ...]: runs the script under test as root with the fake curl first on the PATH.
run_install() {
  set +e
  env PATH="$SRC/test/installer/fakebin:$PATH" FAKE_RELEASE_DIR="$fixture" FAKE_GITHUB_TOKEN="$TOKEN" \
    GITHUB_TOKEN="$TOKEN" "$@" bash "$work/install.sh" >"$out" 2>"$err"
  code=$?
  set -e
}

case "$scenario" in
  fresh)
    make_release "$work/test.key"
    run_install AGENT_HOST_NAME=srv-test
    expect_eq "exit code" 0 "$code"
    expect_eq "installed version" "$VERSION" "$("$BIN" version)"
    expect_eq "config.env" "$(printf 'AGENT_HOST_NAME=srv-test\nGITHUB_TOKEN=%s' "$TOKEN")" "$(cat "$CONFIG")"
    expect_eq "config.env mode" "600 root:root" "$(stat -c '%a %U:%G' "$CONFIG")"
    expect_eq "config dir mode" "700" "$(stat -c '%a' /etc/vps-agent)"
    expect_contains "install line" "install: vps-agent $VERSION at $BIN" "$out"
    expect_contains "doctor" "ok   docker" "$out"
    if grep -qF "$TOKEN" "$out" "$err"; then fail "the token was printed"; fi
    if id vps-agent-deploy >/dev/null 2>&1; then fail "deploy user created without DEPLOY_SSH_PUBLIC_KEY"; fi
    ;;
  migration)
    make_release "$work/test.key"
    install -d -m 0700 /etc/vps-agent
    old="$(printf '# written by install.sh 0.1.0\nCMS_URL=https://cms.example\nAGENT_HOST_NAME=srv-old\nREPORT_INTERVAL=10m')"
    (umask 077 && printf '%s\n' "$old" >"$CONFIG")
    printf '#!/bin/sh\necho 0.1.0\n' >"$BIN" && chmod 0755 "$BIN"
    run_install AGENT_HOST_NAME=srv-new
    expect_eq "exit code" 0 "$code"
    expect_eq "installed version" "$VERSION" "$("$BIN" version)"
    expect_eq "config.env" "$(printf '%s\nGITHUB_TOKEN=%s' "$old" "$TOKEN")" "$(cat "$CONFIG")"
    expect_eq "config.env mode" "600" "$(stat -c '%a' "$CONFIG")"
    ;;
  bad-signature)
    openssl genpkey -algorithm ed25519 -out "$work/other.key"
    make_release "$work/other.key"
    run_install AGENT_HOST_NAME=srv-test
    expect_eq "exit code" 1 "$code"
    expect_contains "message" "bad signature on checksums.txt of v$VERSION; nothing installed" "$err"
    [ ! -e "$BIN" ] || fail "a binary was installed"
    [ ! -e /etc/vps-agent ] || fail "config was written"
    ;;
  bad-checksum)
    make_release "$work/test.key"
    asset=1
    [ "$arch" = amd64 ] || asset=2
    printf 'tampered' >>"$fixture/assets/$asset"
    run_install AGENT_HOST_NAME=srv-test
    expect_eq "exit code" 1 "$code"
    expect_contains "message" "checksum mismatch for $binary; nothing installed" "$err"
    [ ! -e "$BIN" ] || fail "a binary was installed"
    [ ! -e /etc/vps-agent ] || fail "config was written"
    ;;
  deploy-user)
    make_release "$work/test.key"
    run_install AGENT_HOST_NAME=srv-test DEPLOY_SSH_PUBLIC_KEY="ssh-ed25519 $DEPLOY_KEY_BLOB operator@laptop"
    expect_eq "exit code" 0 "$code"
    home=/var/lib/vps-agent-deploy
    expect_eq "home" "$home" "$(getent passwd vps-agent-deploy | cut -d: -f6)"
    expect_eq "password locked" "L" "$(passwd -S vps-agent-deploy | cut -d' ' -f2)"
    expect_eq "authorized_keys" \
      "restrict,command=\"sudo /usr/local/bin/vps-agent deploy\" ssh-ed25519 $DEPLOY_KEY_BLOB vps-agent-deploy" \
      "$(cat "$home/.ssh/authorized_keys")"
    expect_eq "authorized_keys mode" "600 vps-agent-deploy" "$(stat -c '%a %U' "$home/.ssh/authorized_keys")"
    expect_eq ".ssh mode" "700 vps-agent-deploy" "$(stat -c '%a %U' "$home/.ssh")"
    expect_eq "sudoers mode" "440 root" "$(stat -c '%a %U' /etc/sudoers.d/vps-agent-deploy)"
    visudo -cf /etc/sudoers.d/vps-agent-deploy >/dev/null || fail "visudo rejects the sudoers file"
    if grep -q DEPLOY_SSH_PUBLIC_KEY "$CONFIG"; then fail "DEPLOY_SSH_PUBLIC_KEY was persisted"; fi

    # What sshd does with the forced command: the request arrives in SSH_ORIGINAL_COMMAND.
    # There is no network here, so `update` fails talking to GitHub, after logging update.started.
    set +e
    SSH_ORIGINAL_COMMAND="$VERSION" su vps-agent-deploy -c "sudo /usr/local/bin/vps-agent deploy" >"$out" 2>"$err"
    code=$?
    set -e
    expect_eq "deploy exit code" 1 "$code"
    expect_contains "deploy reached update" '"logId":"update.started"' "$err"
    if grep -qF "$TOKEN" "$out" "$err"; then fail "the token was printed"; fi

    set +e
    SSH_ORIGINAL_COMMAND="$VERSION; id" su vps-agent-deploy -c "sudo /usr/local/bin/vps-agent deploy" >"$out" 2>"$err"
    code=$?
    set -e
    expect_eq "bad request exit code" 2 "$code"
    expect_contains "bad request message" "deploy: expected a version like 1.2.3" "$err"

    set +e
    su vps-agent-deploy -c "sudo -n /usr/local/bin/vps-agent update" >"$out" 2>"$err"
    code=$?
    set -e
    [ "$code" -ne 0 ] || fail "sudo allowed vps-agent update to the deploy user"
    ;;
  *) fail "unknown scenario" ;;
esac

if compgen -G "/tmp/tmp.*/vps-agent_*" >/dev/null; then fail "install.sh left its temporary directory"; fi
echo "ok   $scenario"
