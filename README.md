<p align="center">
  <img src=".github/assets/logo.png" alt="talon" width="160">
</p>

# talon

The Heron Labs agent for Easypanel hosts. A static Go binary with three commands.

## Commands

| Command | What it does |
|---|---|
| `talon version` | Prints the version the binary was built with. |
| `talon setup` | Writes `/etc/talon/.env` with mode `600`, asking for every value that is not in the environment, and authorizes the deploy key of push upgrades. |
| `talon doctor` | Checks that `/etc/talon/.env` exists with mode `600` and that the Docker Engine answers `GET /_ping` on its socket within 5 seconds. |

Any other command, or none, is an error.

## Install

The host is an Ubuntu or Debian `amd64` machine. Add the Heron Labs apt repository and install the package:

```bash
sudo curl -fsSL https://apt.heronlabs.com.br/talon.gpg -o /etc/apt/keyrings/talon.gpg
echo "deb [signed-by=/etc/apt/keyrings/talon.gpg] https://apt.heronlabs.com.br stable main" | sudo tee /etc/apt/sources.list.d/talon.list
sudo apt update && sudo apt install talon
```

The binary lands in `/usr/bin/talon` and the package creates `/etc/talon`. For push upgrades it also creates the system user `talon-deploy`, with home `/var/lib/talon-deploy`, the script `/usr/lib/talon/upgrade` and `/etc/sudoers.d/talon-deploy`; it depends on `sudo`. To upgrade, run `sudo apt update && sudo apt install --only-upgrade talon`.

Without the repository, copy `talon_<version>_amd64.deb` from a release to the host and run `sudo dpkg -i` on it.

## Use

```bash
talon version
sudo talon setup
sudo talon doctor
```

The commands run from any directory. The configuration is `/etc/talon/.env`: `setup` writes it and `doctor` reads it; set `TALON_CONFIG_DIR` to use another directory. `doctor` talks to the Docker Engine at `/var/run/docker.sock`; set `DOCKER_SOCKET` to use another socket. `setup` writes the deploy key in `/var/lib/talon-deploy`, the home of `talon-deploy`, and reads the host key in `/etc/ssh`; set `TALON_DEPLOY_HOME` and `TALON_SSH_DIR` to use other directories.

### Setup

`setup` goes through the keys of `.env.example` in order. For each one it takes the environment variable of the same name; when that is empty it asks on the terminal, showing a fallback in brackets:

```text
CMS_URL: https://cms.example.com
AWS_REGION [us-east-1]:
```

The fallback is the value already in `.env` or, without one, the default of the key. The fallback of a token or secret key is shown as `[****]`; what you type is echoed. An empty answer keeps the fallback. A key with no default is required, except `DEPLOY_SSH_PUBLIC_KEY`, which is optional: when any of them ends up empty, `setup` lists them and writes nothing. Otherwise it replaces `.env` with the eleven keys, mode `600`. Lines of the old `.env` that are not one of the keys are dropped.

`DEPLOY_SSH_PUBLIC_KEY` is the public half of the deploy key (see [Push upgrades](#push-upgrades)). Before `.env`, `setup` mirrors it in `/var/lib/talon-deploy/.ssh/authorized_keys`: a key goes in as one line that can only run the upgrade, with its comment dropped; an empty key removes the file. A key that is not one `ssh-ed25519` public key is an error, and nothing is written. When the host has `/etc/ssh/ssh_host_ed25519_key.pub`, the result line carries it as `host_key`, without its comment.

Each command logs one line with its result and exits `0` on success or `1` on failure:

```text
2026/10/04 20:05:40 INFO version version=0.3.0
2026/10/04 20:05:40 INFO setup status=ok host_key="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."
2026/10/04 20:05:40 ERROR setup error="config: missing CMS_URL, CMS_AGENT_TOKEN"
2026/10/04 20:05:40 ERROR setup error="deploy: DEPLOY_SSH_PUBLIC_KEY is not an ssh-ed25519 public key"
2026/10/04 20:05:40 INFO doctor status=ok
2026/10/04 20:05:40 ERROR doctor error="config: .env has mode 644, want 600"
2026/10/04 20:05:40 ERROR command not found command=foo
```

## Releases

A release is a tag with a changelog and two assets for `linux/amd64`: `talon-linux-amd64`, the static binary, and `talon_<version>_amd64.deb`, the package that installs it. The release workflow builds and attaches them; earlier releases have no assets. For any other platform, build from this repository.

Each release is also published to the apt repository at `https://apt.heronlabs.com.br`: its index is rebuilt from every version published so far and signed with the key whose public half is `talon.gpg` at the root of the repository.

## Push upgrades

After each release reaches the apt repository, the release workflow upgrades talon on every host of `DEPLOY_HOSTS`. It connects over SSH as `talon-deploy`, whose key can only run `sudo -n /usr/lib/talon/upgrade`. That script runs `apt-get update` and `apt-get install --only-upgrade talon`, then `talon version`. It takes no arguments and always installs the latest version.

To turn it on:

1. Generate the key pair once, outside the hosts:

   ```bash
   ssh-keygen -t ed25519 -N '' -C talon-deploy -f talon-deploy
   ```

2. Store the private half, `talon-deploy`, as the secret `DEPLOY_SSH_PRIVATE_KEY` of the `main` environment. Then delete it locally.
3. On each host, with talon installed, give `setup` the public half:

   ```bash
   sudo DEPLOY_SSH_PUBLIC_KEY='ssh-ed25519 AAAA...' talon setup
   ```

4. Set the variable `DEPLOY_KNOWN_HOSTS` of the `main` environment to one line per host: its IP and the `host_key` of its `setup` result line.

   ```text
   203.0.113.10 ssh-ed25519 AAAA...
   ```

5. Last, set the variable `DEPLOY_HOSTS` of the `main` environment to the JSON array of the hosts, like `["203.0.113.10"]`. Without it, the job is skipped.

To roll back a host, run `sudo apt install talon=<version>` on it. To revoke the key on a host, remove `DEPLOY_SSH_PUBLIC_KEY` from `/etc/talon/.env` and run `sudo talon setup` again.

## Develop

```bash
make build              # static binary in bin/talon
make package            # Linux binary and its .deb in dist/
make test               # unit tests with coverage
make test:integration   # runs the real binary against a fake Docker Engine
make up                 # fake VPS: Ubuntu, systemd, Docker and Easypanel in a container
```

`CONTRIBUTING.md` has the setup and the workflow. `AGENTS.md` has the code map and the conventions.
