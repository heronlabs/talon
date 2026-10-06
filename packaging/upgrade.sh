#!/bin/sh
# Upgrades talon from its apt repository. The talon-deploy SSH key runs it through sudo.
set -eu
export DEBIAN_FRONTEND=noninteractive
apt-get -qq -o DPkg::Lock::Timeout=300 update
apt-get -qq -y -o DPkg::Lock::Timeout=300 -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold install --only-upgrade talon
talon version
