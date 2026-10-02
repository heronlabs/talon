# Security Policy

## Reporting a Vulnerability

Report vulnerabilities to **security@heronlabs.com**. Do not open public issues.

Expect a response within 72 hours. After triage we will keep you updated on progress and coordinate disclosure timing.

## Scope

`vps-agent` runs as root on Easypanel hosts. It handles:

- the Docker socket, which is equivalent to root on the host;
- `/etc/vps-agent/config.env` (mode 0600) with the CMS, Easypanel and AWS credentials of the host;
- the release download token used by `scripts/install.sh`.

Key concerns: credential exposure in logs or reports, integrity of the released binary (`checksums.txt`), and anything that would let a CMS manifest make the agent read or write beyond its host.

### Supported versions

| Version | Supported |
|---------|-----------|
| latest minor | ✅ |
| older | ❌ |

## Disclosure

We follow coordinated disclosure. After a fix is released we will publish a GitHub Security Advisory and credit reporters (unless you prefer anonymity).
