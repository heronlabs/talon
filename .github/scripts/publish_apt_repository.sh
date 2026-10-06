#!/bin/sh
# Builds the apt repository in apt/ with the .deb of a release, for
# heronlabs/action-web-server to publish.
#
#   publish_apt_repository.sh
#
# Runs from the root of the checkout as the BUILD_COMMAND of the action. TAG is
# the release that holds the .deb, BUCKET the bucket of the repository and
# GH_TOKEN a token that can read the release. The action provides the AWS
# credentials and writes .env with APT_GPG_PRIVATE_KEY, the armored signing key.
set -eu

set -a
# shellcheck source=/dev/null
. ./.env
set +a

gh release download "$TAG" --pattern 'talon_*_amd64.deb' --dir dist

# The action prunes from the bucket what is not in apt/, so the published pool
# comes down first: the index then lists every version and none is pruned.
aws s3 sync "s3://$BUCKET/pool" apt/pool

if ! command -v apt-ftparchive > /dev/null; then
  sudo apt-get update
  sudo apt-get install -y --no-install-recommends apt-utils
fi

# build_apt_repository.sh signs with the default key and exports every public
# key, so the keyring is a fresh one holding only the signing key.
GNUPGHOME=$(mktemp -d)
export GNUPGHOME
printf '%s\n' "$APT_GPG_PRIVATE_KEY" | gpg --batch --import

.github/scripts/build_apt_repository.sh apt dist/talon_*_amd64.deb
