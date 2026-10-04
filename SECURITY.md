# Security Policy

## Reporting a Vulnerability

Report vulnerabilities to **security@heronlabs.com**. Do not open public issues.

Expect a response within 72 hours. After triage we will keep you updated on progress and coordinate disclosure timing.

## Scope

`talon` runs as root on Easypanel hosts. Today it handles:

- the Docker socket, which is equivalent to root on the host;
- the `.env` file in its working directory, which `talon doctor` refuses unless its mode is `600`.

As the planned commands arrive it will also handle:

- the CMS, Easypanel and cloud credentials of the host, kept in that `.env`;
- client secrets read from each client's own secret store, which pass through the agent on their way to Easypanel and are never sent to the CMS;
- release downloads, accepted only when their signature and checksum match.

Key concerns: a credential or an environment value showing up in logs or reports, the integrity of the binary a host installs, and anything that would let a CMS manifest make the agent read, write or delete beyond what it manages on its own host.

### Supported versions

| Version | Supported |
|---------|-----------|
| latest minor | ✅ |
| older | ❌ |

## Disclosure

We follow coordinated disclosure. After a fix is released we will publish a GitHub Security Advisory and credit reporters (unless you prefer anonymity).
