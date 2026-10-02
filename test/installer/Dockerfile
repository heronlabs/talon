# Image for the install.sh tests: what a fresh Ubuntu VPS needs to run the bootstrap (python3,
# openssl) plus sudo/visudo for the deploy user. No curl: the tests put a fake one on the PATH.
FROM ubuntu:24.04

RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends python3 openssl sudo \
  && rm -rf /var/lib/apt/lists/*
