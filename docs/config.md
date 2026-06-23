# Configuration Reference

`stead` stores user config at:

```text
~/.config/stead/config.toml
```

Check the path:

```bash
stead config path
```

Show the current summary:

```bash
stead config show
```

## Starter Config

Preview:

```bash
stead config init --dry-run
```

Example structure:

```toml
[defaults]
alias = "devmac"

[hosts.devmac]
hostname = "<tailscale-ip-or-magicdns>"
user = "ed"
port = 22
identity_file = "~/.ssh/stead_ed25519"
preferred_network = "tailscale"

[hosts.devmac.wake]
mac_address = "<host-mac-address>"
broadcast = "<lan-broadcast-address>"
timeout = "90s"
interval = "2s"

[hosts.devmac.session]
tmux = true
tmux_session = "main"
project_dir = "~/src"
```

## `[defaults]`

### `alias`

Default host alias used when a command does not receive `--alias`.

```toml
[defaults]
alias = "devmac"
```

## `[hosts.<alias>]`

### `hostname`

Network address used by normal OpenSSH.

This can be:

- Tailscale MagicDNS name
- Tailscale IP
- LAN hostname
- LAN IP

This is not Tailscale SSH.

### `user`

macOS user to log in as on the host.

### `port`

SSH TCP port. Defaults to `22` when omitted or zero in generated config.

### `identity_file`

Client private key path used in the generated SSH config block.

Generated default:

```text
~/.ssh/stead_<alias>_ed25519
```

The private key stays on the client machine.

### `preferred_network`

Informational network preference. Current common value:

```toml
preferred_network = "tailscale"
```

This does not enable or configure Tailscale SSH.

## `[hosts.<alias>.wake]`

Wake settings control the optional Wake-on-LAN flow used by `stead wake` and `stead connect --wake`.

If the target SSH port is already reachable, no Wake-on-LAN packet is sent. If the target is not reachable, apply mode requires a real `mac_address` and `broadcast` before sending a packet.

Use `stead client wake-config` to update these fields without hand-editing TOML:

```bash
stead client wake-config --alias devmac --mac-address <host-lan-mac> --broadcast <lan-broadcast>
```

### `mac_address`

Host MAC address for Wake-on-LAN.

### `broadcast`

LAN broadcast address for Wake-on-LAN.

### `timeout`

How long the wake flow should wait for SSH reachability.

### `interval`

How often the wake flow should poll SSH reachability.

## `[hosts.<alias>.session]`

Session settings describe desired login ergonomics.

### `tmux`

Whether tmux session continuity is desired.

`stead host install` can install a managed tmux auto-attach block for SSH sessions.

### `tmux_session`

Preferred tmux session name.

### `project_dir`

Preferred project directory after login.

Current commands store and display it. They do not automatically `cd` into it yet.

## Placeholders

Values wrapped in angle brackets are placeholders:

```text
<tailscale-ip-or-magicdns>
<host-mac-address>
<lan-broadcast-address>
```

`stead status`, `stead client status`, and `stead setup` report placeholder values as incomplete or not ready where relevant.
