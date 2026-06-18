# Stead

`stead` is a personal CLI for reproducible remote-development access to a trusted machine.

It does not replace SSH. It automates setup, status checks, connection helpers, wake flow, and uninstall around normal OpenSSH-based access.

## Architecture

```text
client machine
  -> optional wake flow
  -> normal OpenSSH client
  -> Tailscale/LAN address
  -> macOS sshd on host
  -> SSH key authentication
  -> macOS user policy
  -> zsh
  -> tmux session
```

## Principles

- Tailscale is private networking only: stable IP and MagicDNS.
- Tailscale SSH is intentionally out of scope.
- OpenSSH/macOS sshd remains the SSH transport and authentication layer.
- SSH keys and macOS user policy provide auth hardening.
- tmux/zsh provide session continuity after login.
- `stead` provides setup, status, connect, wake, and uninstall automation.
- CLI color is terminal-aware and disabled for non-interactive output or `NO_COLOR`.

## Early CLI Shape

```bash
stead status
stead setup --alias devmac --dry-run
stead setup --alias devmac --dry-run --verify
stead verify --alias devmac
stead config path
stead config show
stead config init
stead config init --dry-run
stead doctor

stead host status
stead host authorize --public-key 'ssh-ed25519 ...' --alias devmac --dry-run
stead host unauthorize --public-key 'ssh-ed25519 ...' --alias devmac --dry-run
stead host install
stead host harden
stead host uninstall

stead client status
stead client init
stead client init --alias devmac
stead client init --alias devmac --discover tailscale --yes
stead client init --alias devmac --hostname devmac.tailnet.ts.net --user ed --yes
stead client plan --alias devmac
stead client apply --dry-run --alias devmac
stead client apply --alias devmac
stead client unapply --alias devmac --dry-run
stead client uninstall

stead wake --alias devmac
stead connect --alias devmac
stead connect --alias devmac --wake
```

## Client Setup

Use the setup planner to see the remaining local steps without changing files:

```bash
stead setup --alias devmac --dry-run
```

`stead client init` prepares one client machine for one host. It can prompt for the host name, creates or updates `~/.config/stead/config.toml`, and generates a local Ed25519 SSH key if the configured key does not exist.

```bash
stead client init --alias devmac --discover tailscale --yes
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
stead client apply --dry-run --alias devmac
stead client apply --alias devmac
stead verify --alias devmac
ssh devmac
```

`--discover tailscale` reads `tailscale status --json` peer metadata to find the host's MagicDNS name or Tailscale IP. This is still normal OpenSSH-over-Tailscale; Tailscale SSH is not used.

`stead verify` runs a non-interactive `ssh` check using `BatchMode=yes`, so it verifies key-based login without prompting for a password.

`stead setup --verify` includes that same SSH check in the setup plan and can report host authorization as OK when login is already proven.

`stead host authorize` runs on the host Mac. It appends a client public key to `~/.ssh/authorized_keys` if it is not already present.

Undo commands are deliberately narrow:

```bash
stead client unapply --alias devmac --dry-run
stead host unauthorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
```

## Install Model

The intended distribution model is private/local:

```bash
git clone https://github.com/weirdry/stead.git ~/src/stead
cd ~/src/stead
./install.sh
```

The preferred binary target is:

```text
~/.local/bin/stead
```

`./install.sh` builds the local checkout and copies the binary to the target. It does not modify SSH configuration, SSH keys, `authorized_keys`, Tailscale, launchd, or macOS settings.

Preview the install:

```bash
./install.sh --dry-run
```

If `~/.local/bin` is not on `PATH`, add it to your shell profile so `stead` works outside the repo:

```zsh
export PATH="$HOME/.local/bin:$PATH"
```

Uninstall only the installed binary:

```bash
./uninstall.sh --dry-run
./uninstall.sh
```

## Development

This repo uses `just`.

```bash
just
just install-dry-run
just install
just check
```

See [docs/design.md](docs/design.md) for the full design.
