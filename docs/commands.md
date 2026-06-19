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

### `stead verify`

Checks whether OpenSSH key login works without prompting for a password.

```bash
stead verify --alias devmac
stead verify --alias devmac --timeout 15s
```

This runs the system `ssh` command with `BatchMode=yes`.

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
stead host harden --dry-run
```

This command is preview-only for now and requires `--dry-run`. It does not write `/etc/ssh/sshd_config.d/stead.conf`, validate sshd, reload services, or change Remote Login.

With `--disable-password`, the proposed drop-in includes:

```sshconfig
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
AllowUsers ed
```

Without `--disable-password`, password-style SSH authentication is left unchanged in the preview.

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
