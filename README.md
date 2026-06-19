# Stead

`stead` is a personal CLI for reproducible remote-development access to a trusted Mac.

It does not replace SSH. It helps set up and inspect normal OpenSSH access over a private network such as Tailscale.

## Model

```text
client Mac
  -> normal OpenSSH client
  -> Tailscale/LAN address
  -> macOS sshd on host Mac
  -> SSH key authentication
  -> macOS user policy
  -> shell/tmux session
```

Tailscale is used only for reachability and stable private addressing. Tailscale SSH is intentionally out of scope.

## Install

From a local clone:

```bash
git clone https://github.com/weirdry/stead.git ~/src/stead
cd ~/src/stead
./install.sh --dry-run
./install.sh
```

The default install target is:

```text
~/.local/bin/stead
```

If needed, add it to your shell profile:

```zsh
export PATH="$HOME/.local/bin:$PATH"
```

Update:

```bash
cd ~/src/stead
git pull origin main
./install.sh
```

Uninstall the installed binary only:

```bash
./uninstall.sh --dry-run
./uninstall.sh
```

## Common Commands

```bash
stead status
stead host status
stead host status --effective
stead client status

stead setup --alias devmac --dry-run
stead setup --alias devmac --dry-run --verify

stead client init --alias devmac --hostname <host-tailscale-name-or-ip> --yes
stead client apply --dry-run --alias devmac
stead client apply --alias devmac

stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
stead host harden --dry-run --user ed --disable-password

stead verify --alias devmac
ssh devmac
```

Use `--no-color` or `NO_COLOR=1` for plain output:

```bash
stead --no-color status
```

## Setup Guides

- [From scratch host + client setup](docs/from-scratch.md)
- [Existing setup migration](docs/migration.md)
- [Safety and ownership model](docs/safety.md)
- [Command reference](docs/commands.md)
- [Configuration reference](docs/config.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Design notes](docs/design.md)

## Development

This repo uses `just`.

```bash
just
just check
just build
just install-dry-run
just install
```
