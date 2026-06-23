# Command Reference

This page documents the commands implemented by `stead`.

Global option:

```bash
stead --no-color <command>
```

`--no-color` disables terminal styling. Color is also disabled for redirected output, `TERM=dumb`, and `NO_COLOR`.

## Core

### `stead status`

Prints a read-only combined host/client status snapshot.

```bash
stead status
stead --no-color status
```

It inspects OpenSSH, user SSH files, Tailscale network metadata, tmux availability, client aliases, and local `stead` config.

### `stead setup`

Prints the remaining setup steps for an alias.

```bash
stead setup --alias devmac --dry-run
stead setup --alias devmac --dry-run --verify
```

`setup` currently requires `--dry-run`; it is a planner, not an apply command.

With `--verify`, it runs the same non-interactive SSH check as `stead verify`. If login succeeds, host authorization is considered proven.

When key login is verified and the configured host user is known, the setup plan also suggests:

```bash
stead host harden --dry-run --user <user> --disable-password
```

### `stead verify`

Checks whether OpenSSH key login works without prompting for a password.

```bash
stead verify --alias devmac
stead verify --alias devmac --timeout 15s
```

This runs the system `ssh` command with `BatchMode=yes`.

### `stead connect`

Execs the normal system SSH client for a configured alias.

```bash
stead connect --alias devmac
stead connect --alias devmac --wake
stead connect
```

If `--alias` is omitted, `stead` uses the default alias from `~/.config/stead/config.toml`.

This command does not replace SSH authentication, does not use Tailscale SSH, and does not store credentials. It checks that the alias exists in both `stead` config and `~/.ssh/config`, then execs:

```bash
ssh <alias>
```

With `--wake`, `stead` runs the wake flow first, then execs `ssh <alias>`. If the SSH port is already reachable, no Wake-on-LAN packet is sent.

### `stead wake`

Checks wake readiness or sends a Wake-on-LAN packet.

```bash
stead wake --alias devmac --dry-run
stead wake --alias devmac --dry-run --timeout 5s
stead wake --alias devmac
```

The dry run loads the configured hostname and SSH port, checks whether the TCP port is reachable, and reports whether `mac_address` and `broadcast` are configured. It does not send a Wake-on-LAN packet and does not run SSH authentication.

Without `--dry-run`, if the SSH port is already reachable, `stead` exits without sending a packet. If it is not reachable, `stead` requires `mac_address` and `broadcast`, sends one Wake-on-LAN magic packet, and waits for the SSH port until the configured timeout. It still does not perform SSH authentication.

### `stead client wake-config`

Updates wake metadata for an existing client host entry.

```bash
stead client wake-config --alias devmac --mac-address <host-lan-mac> --broadcast <lan-broadcast> --dry-run
stead client wake-config --alias devmac --mac-address <host-lan-mac> --broadcast <lan-broadcast>
stead client wake-config --alias devmac --mac-address <host-lan-mac> --broadcast <lan-broadcast> --timeout 90s --interval 2s
```

This command edits only `~/.config/stead/config.toml`. It does not touch `~/.ssh/config`, generate keys, run SSH, send Wake-on-LAN packets, or use Tailscale SSH.

## Host

### `stead host status`

Prints read-only host-side status.

```bash
stead host status
stead host status --effective
```

`--effective` runs `sshd -T` without sudo and summarizes selected effective server settings. On macOS this may report that root-readable host keys are required; `stead` does not escalate automatically.

### `stead host authorize`

Adds a client public key to the host user's `~/.ssh/authorized_keys` if it is not already present.

```bash
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
```

The command matches key material, so the same key is not duplicated with a different comment.

### `stead host unauthorize`

Removes a public key from the host user's `~/.ssh/authorized_keys`.

```bash
stead host unauthorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host unauthorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
```

It removes only the matching public key material.

### `stead host harden`

Prints the host sshd hardening drop-in that `stead` would install.

```bash
stead host harden --dry-run --user ed --disable-password
sudo stead host harden --apply --user ed --disable-password --confirm-key-login
stead host harden --dry-run
stead host harden --unapply --dry-run
sudo stead host harden --unapply --apply --confirm
```

`--dry-run` prints the proposed `/etc/ssh/sshd_config.d/stead.conf` without changing files.

`--apply` writes the managed drop-in. It usually needs `sudo` for the default `/etc/ssh/sshd_config.d/stead.conf` target. Before writing the target, `stead` validates a temporary candidate with `sshd -t`. If an existing target is replaced, `stead` writes a timestamped backup next to it.

`stead` does not reload sshd, restart services, or change Remote Login.

When `--disable-password` is used with `--apply`, the command requires `--confirm-key-login` or `--force`.

With `--disable-password`, the proposed drop-in includes:

```sshconfig
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
AllowUsers ed
```

Without `--disable-password`, password-style SSH authentication is left unchanged in the preview.

`--unapply` removes only `/etc/ssh/sshd_config.d/stead.conf`. It does not remove backups, authorized keys, SSH keys, Apple sshd config files, Remote Login settings, or Tailscale settings. After applying unhardening, run:

```bash
sudo stead host reload --apply --confirm
```

### `stead host install`

Installs the managed tmux auto-attach block in the host user's shell config.

```bash
stead host install --dry-run
stead host install --dry-run --tmux-session main
stead host install --apply --tmux-session main
stead host install --dry-run --force
```

The default target is `~/.zshrc`. Use `--shell-config path` for tests or unusual shell layouts.

The managed block runs only for interactive SSH sessions, only when `tmux` exists, and only when not already inside tmux. It does not change SSH authentication, sshd config, Tailscale, launchd, or firewall settings.

If an unmanaged custom tmux auto-attach snippet is already present, `stead` leaves it alone unless `--force` is passed.

### `stead host uninstall`

Removes only the managed tmux auto-attach block from the host user's shell config.

```bash
stead host uninstall --dry-run
stead host uninstall --apply --confirm
```

It preserves unrelated shell config content and creates a timestamped backup when it changes the file.

### `stead host validate`

Runs read-only host validation.

```bash
stead host validate
```

This checks the expected sshd files and runs `sshd -t` without sudo. On macOS it may report that root-readable host keys are required; in that case, run the shown sudo validation manually if you want a privileged validation result.

### `stead host reload`

Prints the manual reload plan.

```bash
stead host reload --dry-run
sudo stead host reload --apply --confirm
```

`--dry-run` prints the manual reload plan and does not call `launchctl`.

`--apply --confirm` validates sshd with `/usr/sbin/sshd -t`, then runs `launchctl kickstart -k system/com.openssh.sshd`. It does not modify Remote Login settings.

## Client

### `stead client status`

Prints read-only client-side status.

```bash
stead client status
```

### `stead client init`

Creates or updates `~/.config/stead/config.toml` and creates a client Ed25519 key if needed.

```bash
stead client init --alias devmac --hostname <host> --yes
stead client init --alias devmac --hostname <host> --dry-run --yes
stead client init --alias devmac --discover tailscale --yes
```

Useful flags:

- `--alias name`
- `--hostname host`
- `--discover tailscale`
- `--user user`
- `--identity-file path`
- `--dry-run`
- `--yes`

When `--yes` is used, either `--hostname` or `--discover tailscale` is required.

### `stead client plan`

Prints a read-only plan for a configured host alias.

```bash
stead client plan --alias devmac
```

### `stead client apply`

Adds or replaces the managed SSH config block for an alias.

```bash
stead client apply --dry-run --alias devmac
stead client apply --alias devmac
```

The managed block is written to `~/.ssh/config`.

### `stead client unapply`

Removes the managed SSH config block for an alias.

```bash
stead client unapply --alias devmac --dry-run
stead client unapply --alias devmac
```

It does not remove arbitrary non-stead SSH config entries.

## Config

### `stead config path`

Prints the default config path.

```bash
stead config path
```

### `stead config show`

Prints a redacted/structured summary of local `stead` config.

```bash
stead config show
```

### `stead config init`

Creates a starter config if one does not already exist.

```bash
stead config init --dry-run
stead config init
```

Prefer `stead client init` for real client setup, because it fills in one host and can generate the key.
