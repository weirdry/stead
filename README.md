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

## Early CLI Shape

```bash
stead status
stead config path
stead config show
stead config init
stead config init --dry-run
stead doctor

stead host status
stead host authorize --public-key 'ssh-ed25519 ...' --alias devmac --dry-run
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
stead client uninstall

stead wake --alias devmac
stead connect --alias devmac
stead connect --alias devmac --wake
```

## Client Setup

`stead client init` prepares one client machine for one host. It can prompt for the host name, creates or updates `~/.config/stead/config.toml`, and generates a local Ed25519 SSH key if the configured key does not exist.

```bash
stead client init --alias devmac --discover tailscale --yes
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
stead client apply --dry-run --alias devmac
stead client apply --alias devmac
ssh devmac
```

`--discover tailscale` reads `tailscale status --json` peer metadata to find the host's MagicDNS name or Tailscale IP. This is still normal OpenSSH-over-Tailscale; Tailscale SSH is not used.

`stead host authorize` runs on the host Mac. It appends a client public key to `~/.ssh/authorized_keys` if it is not already present.

## Install Model

The intended distribution model is private/local:

```bash
git clone <private-repo> ~/src/stead
cd ~/src/stead
./install.sh
```

The preferred binary target is:

```text
~/.local/bin/stead
```

## Development

This repo uses `just`.

```bash
just
just check
```

See [docs/design.md](docs/design.md) for the full design.
