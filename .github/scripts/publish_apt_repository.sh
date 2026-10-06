#!/bin/sh
# Builds and signs the apt repository in apt/ with the .deb of the release TAG,
# as the BUILD_COMMAND of heronlabs/action-web-server, which writes .env with
# APT_GPG_PRIVATE_KEY and publishes apt/ to BUCKET.
set -eu

suite=dists/stable
index=$suite/main/binary-amd64
pool=pool/main/t/talon

set -a
# shellcheck source=/dev/null
. ./.env
set +a

gh release download "$TAG" --pattern 'talon_*_amd64.deb' --dir dist

# The index lists the whole pool, and the action prunes what apt/ does not hold.
aws s3 sync "s3://$BUCKET/pool" apt/pool

if ! command -v apt-ftparchive > /dev/null; then
  sudo apt-get update
  sudo apt-get install -y --no-install-recommends apt-utils
fi

GNUPGHOME=$(mktemp -d)
export GNUPGHOME
printf '%s\n' "$APT_GPG_PRIVATE_KEY" | gpg --batch --import

mkdir -p "apt/$pool" "apt/$index"
cp dist/talon_*_amd64.deb "apt/$pool/"
cd apt

apt-ftparchive --arch amd64 packages pool > "$index/Packages"
gzip -9 -n -c "$index/Packages" > "$index/Packages.gz"

# Written outside the suite, so Release does not list itself.
apt-ftparchive \
  -o APT::FTPArchive::Release::Origin="Heron Labs" \
  -o APT::FTPArchive::Release::Label=talon \
  -o APT::FTPArchive::Release::Suite=stable \
  -o APT::FTPArchive::Release::Codename=stable \
  -o APT::FTPArchive::Release::Architectures=amd64 \
  -o APT::FTPArchive::Release::Components=main \
  release "$suite" > Release
mv Release "$suite/Release"

gpg --batch --yes --clearsign --output "$suite/InRelease" "$suite/Release"
gpg --batch --yes --armor --detach-sign --output "$suite/Release.gpg" "$suite/Release"
gpg --batch --yes --output talon.gpg --export
