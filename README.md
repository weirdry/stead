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
stead config init --dry-run
stead doctor

stead host status
stead host install
stead host harden
stead host uninstall

stead client status
stead client install
stead client uninstall

stead wake --alias devmac
stead connect --alias devmac
stead connect --alias devmac --wake
```

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
