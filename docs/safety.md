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

## Host Hardening

Host hardening is intentionally not automatic yet.

`stead host harden` is currently dry-run only. It prints the proposed `/etc/ssh/sshd_config.d/stead.conf` content and refuses to run without `--dry-run`.

Current status output may warn about:

- password authentication
- keyboard-interactive authentication
- missing `AllowUsers` or `AllowGroups`
- missing managed host config

Those warnings are informational until the privileged apply path is implemented and reviewed.
