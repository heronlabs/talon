#!/bin/sh
# Adds .deb packages to a local copy of the apt repository and signs its index.
#
#   build_apt_repository.sh <repository> <package.deb>...
#
# The index is rebuilt from <repository>/pool, so the packages already
# published must be there. The signing key is the default secret key of the gpg
# keyring (GNUPGHOME). ARCH is the Debian architecture, amd64 by default.
set -eu

repository=$1
shift
arch=${ARCH:-amd64}
suite=dists/stable
index=$suite/main/binary-$arch

mkdir -p "$repository/pool/main/t/talon" "$repository/$index"
cp "$@" "$repository/pool/main/t/talon/"
cd "$repository"

apt-ftparchive --arch "$arch" packages pool > "$index/Packages"
gzip -9 -n -c "$index/Packages" > "$index/Packages.gz"

# apt-ftparchive lists every file under the suite, so the release files of the
# previous run go first and the new one is written outside it.
rm -f "$suite/Release" "$suite/Release.gpg" "$suite/InRelease"
apt-ftparchive \
  -o APT::FTPArchive::Release::Origin="Heron Labs" \
  -o APT::FTPArchive::Release::Label=talon \
  -o APT::FTPArchive::Release::Suite=stable \
  -o APT::FTPArchive::Release::Codename=stable \
  -o APT::FTPArchive::Release::Architectures="$arch" \
  -o APT::FTPArchive::Release::Components=main \
  release "$suite" > Release
mv Release "$suite/Release"

gpg --batch --yes --clearsign --output "$suite/InRelease" "$suite/Release"
gpg --batch --yes --armor --detach-sign --output "$suite/Release.gpg" "$suite/Release"
gpg --batch --yes --output talon.gpg --export
