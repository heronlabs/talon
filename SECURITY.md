# Security Policy

## Reporting a Vulnerability

Report vulnerabilities to **security@heronlabs.com**. Do not open public issues.

Expect a response within 72 hours. After triage we will keep you updated on progress and coordinate disclosure timing.

## Scope

`talon` runs on Easypanel hosts. It handles:

- the Docker socket, which is equivalent to root on the host;
- the `.env` file in its working directory, which `talon doctor` rejects unless its mode is `600`.

### Supported versions

| Version | Supported |
|---------|-----------|
| latest minor | ✅ |
| older | ❌ |

## Disclosure

We follow coordinated disclosure. After a fix is released we will publish a GitHub Security Advisory and credit reporters (unless you prefer anonymity).
