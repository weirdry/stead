# Stead

```text
███████╗████████╗███████╗ █████╗ ██████╗
██╔════╝╚══██╔══╝██╔════╝██╔══██╗██╔══██╗
███████╗   ██║   █████╗  ███████║██║  ██║
╚════██║   ██║   ██╔══╝  ██╔══██║██║  ██║
███████║   ██║   ███████╗██║  ██║██████╔╝
╚══════╝   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═════╝
```

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

## Happy Path

Use the same local checkout on both Macs:

```bash
git pull origin main
./install.sh
```

On the host Mac, inspect current SSH server state:

```bash
stead host status
```

On the client Mac, create a local alias and key:

```bash
stead client init --alias devmac --hostname <host-tailscale-name-or-ip> --yes
stead client apply --dry-run --alias devmac
stead client apply --alias devmac
```

On the host Mac, authorize the printed client public key:

```bash
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
```

On the client Mac, verify and connect:

```bash
stead verify --alias devmac
stead doctor --alias devmac --verify
stead connect --alias devmac
```

After key login works, optionally harden the host:

```bash
stead host harden --dry-run --user ed --disable-password
sudo stead host harden --apply --user ed --disable-password --confirm-key-login
sudo stead host reload --apply --confirm
```

Optional wake flow, configured on the client:

```bash
stead client wake-config --alias devmac --mac-address <host-lan-mac> --broadcast <lan-broadcast> --dry-run
stead client wake-config --alias devmac --mac-address <host-lan-mac> --broadcast <lan-broadcast>
stead wake --alias devmac --dry-run
stead connect --alias devmac --wake
```

## Useful Commands

```bash
stead status
stead host status
stead host status --effective
stead client status
stead doctor --alias devmac
stead setup --alias devmac --dry-run --verify
stead client uninstall --alias devmac --dry-run
stead version
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
