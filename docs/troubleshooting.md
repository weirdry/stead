# Troubleshooting

## `stead: command not found`

The binary is probably not on `PATH`.

Check the default install target:

```bash
ls -l ~/.local/bin/stead
```

Add this to your shell profile if needed:

```zsh
export PATH="$HOME/.local/bin:$PATH"
```

Then open a new shell or run:

```bash
source ~/.zshrc
```

## `ssh: Could not resolve hostname <alias>`

The SSH alias is not installed or points to an unresolved hostname.

Check from the client Mac:

```bash
stead client status
stead client apply --dry-run --alias <alias>
```

If the alias is missing, apply it:

```bash
stead client apply --alias <alias>
```

If the alias exists but the hostname is wrong, update the host in:

```text
~/.config/stead/config.toml
```

Then re-apply:

```bash
stead client apply --dry-run --alias <alias>
stead client apply --alias <alias>
```

## `stead verify` Fails

Run:

```bash
stead setup --alias <alias> --dry-run --verify
```

Common causes:

- client SSH alias is missing
- host address is wrong
- host Remote Login / sshd is not enabled
- public key was not authorized on the host
- wrong `user` in config
- Tailscale or LAN path is down

Check host-side authorization:

```bash
stead host status
```

Authorize the public key if needed:

```bash
stead host authorize --alias <alias> --public-key 'ssh-ed25519 ...' --dry-run
stead host authorize --alias <alias> --public-key 'ssh-ed25519 ...'
```

## Password Prompt Appears

`stead verify` uses `BatchMode=yes`, so it should not prompt for a password.

If normal `ssh <alias>` prompts for a password, key auth probably did not succeed.

Check:

```bash
ssh -G <alias> | grep -E '^(hostname|user|port|identityfile) '
stead setup --alias <alias> --dry-run --verify
```

Then confirm the matching public key is present on the host:

```bash
stead host authorize --alias <alias> --public-key 'ssh-ed25519 ...' --dry-run
```

If it says no changes are needed, the key is already present.

## Tailscale CLI Missing

`stead status` may show:

```text
tailscale CLI: missing
Tailscale.app: ok
tailscale IP: ok
```

This means the macOS app exists, but the `tailscale` command-line tool is not available on `PATH`.

Normal SSH-over-Tailscale can still work if the network interface is up and you use a known Tailscale IP or MagicDNS name.

Discovery with:

```bash
stead client init --discover tailscale
```

requires a usable `tailscale` CLI.

## Existing SSH Alias Was Not Removed

`stead client unapply` removes only managed blocks:

```sshconfig
# BEGIN stead <alias>
...
# END stead <alias>
```

It will not remove arbitrary existing `Host <alias>` entries.

This is intentional. Remove old manual SSH config by editing `~/.ssh/config` yourself after confirming it is safe.

## Public Key Already Exists

`stead host authorize` matches public key material.

If the same key already exists with a different comment, the command reports no changes needed and does not add a duplicate.

## Colors Are Unwanted

Use:

```bash
stead --no-color status
```

or:

```bash
NO_COLOR=1 stead status
```

Color is also disabled automatically for redirected output and `TERM=dumb`.
