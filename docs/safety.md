# Safety And Ownership

`stead` is intentionally narrow. It automates personal OpenSSH setup without hiding or replacing SSH.

## What Stead Does Not Own

`stead` does not own:

- Tailscale authentication
- Tailscale SSH
- macOS user passwords
- private SSH keys outside configured `stead` identity paths
- arbitrary existing `~/.ssh/config` entries
- arbitrary existing `authorized_keys` lines
- macOS Remote Login state
- launchd service state
- firewall settings
- system sshd hardening not explicitly installed by a future `stead` host command

## What Stead May Create Or Modify

Current commands may create or modify:

```text
~/.config/stead/config.toml
~/.ssh/stead_<alias>_ed25519
~/.ssh/stead_<alias>_ed25519.pub
~/.ssh/config
~/.ssh/authorized_keys
~/.local/bin/stead
```

The client SSH config changes are marker based:

```sshconfig
# BEGIN stead <alias>
Host <alias>
    ...
# END stead <alias>
```

`stead client unapply` removes only the matching managed block.

`stead host authorize` and `stead host unauthorize` match public key material, not only comments. This avoids duplicate keys with different comments and removes only the intended key.

## Dry Run First

Most write-capable commands support dry-run:

```bash
stead client init --alias devmac --hostname <host> --dry-run --yes
stead client apply --dry-run --alias devmac
stead client unapply --dry-run --alias devmac
stead host authorize --alias devmac --public-key 'ssh-ed25519 ...' --dry-run
stead host unauthorize --alias devmac --public-key 'ssh-ed25519 ...' --dry-run
./install.sh --dry-run
./uninstall.sh --dry-run
```

Use dry-run before applying changes.

## Tailscale Boundary

`stead` may read Tailscale network metadata when available, such as an IP or MagicDNS name.

`stead` must not:

- enable Tailscale SSH
- depend on Tailscale SSH ACLs
- configure Tailscale auth
- treat Tailscale identity as SSH authentication

The intended model is:

```text
Tailscale = private network path
OpenSSH = SSH transport and authentication
macOS user policy = account authorization
stead = setup/status/undo automation
```

## Install And Uninstall

`./install.sh` builds the local checkout and copies the binary to:

```text
~/.local/bin/stead
```

It does not modify SSH configuration, SSH keys, `authorized_keys`, Tailscale, launchd, firewall, or macOS settings.

`./uninstall.sh` removes only the installed binary. It does not remove config, keys, managed SSH blocks, or authorized keys.

## Host Session Install

`stead host install --dry-run` previews the managed tmux auto-attach block for the current user's shell config.

`stead host install --apply` edits only the target shell config, defaulting to `~/.zshrc`. It creates a timestamped backup when modifying an existing file. It does not change SSH authentication, sshd config, Remote Login, Tailscale, launchd, firewall, keys, or authorized keys.

`stead host uninstall --apply --confirm` removes only the managed tmux auto-attach block. It does not remove custom shell snippets.

## Host Hardening

Host hardening is intentionally not automatic yet.

`stead host harden --dry-run` prints the proposed `/etc/ssh/sshd_config.d/stead.conf` content without changing files.

`stead host harden --apply` writes only the managed drop-in target. It validates a temporary candidate first, creates a timestamped backup when replacing an existing file, and does not reload sshd automatically.

`stead host harden --unapply` removes only `/etc/ssh/sshd_config.d/stead.conf`. It leaves backups, SSH keys, authorized keys, Apple config files, Remote Login, and Tailscale untouched.

When password auth is disabled during apply, `stead` requires `--confirm-key-login` or `--force`.

`stead host validate` is read-only. `stead host reload --dry-run` prints manual commands only and does not call `launchctl`. `stead host reload --apply --confirm` validates sshd before calling `launchctl kickstart`.

`stead connect` execs the system `ssh` command for a configured alias. It does not replace SSH authentication, store credentials, or use Tailscale SSH.

`stead connect --wake` runs the same wake flow first, then execs the system `ssh` command. It still does not replace SSH authentication, store credentials, or use Tailscale SSH.

`stead wake --dry-run` checks TCP reachability and wake configuration only. It does not send Wake-on-LAN packets or run SSH authentication.

`stead wake` sends a Wake-on-LAN packet only when SSH is not already reachable and wake MAC/broadcast config is complete. It does not run SSH authentication.

`stead client wake-config` updates only the local Stead config wake fields for an existing alias. It does not send network packets, edit SSH config, generate keys, or use Tailscale SSH.

Current status output may warn about:

- password authentication
- keyboard-interactive authentication
- missing `AllowUsers` or `AllowGroups`
- missing managed host config

Those warnings are informational until you intentionally run the privileged apply path.
