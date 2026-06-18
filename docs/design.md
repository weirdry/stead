# Stead Design

## Purpose

`stead` is a personal CLI for reproducible remote-development access to a trusted machine.

It does not replace SSH. It automates the setup, status checks, connection helpers, wake flow, and uninstall flow around normal OpenSSH-based access.

## Core Architecture

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

Layer responsibilities:

```text
Tailscale = private networking, stable IP, MagicDNS
OpenSSH/macOS sshd = SSH transport and authentication
SSH keys/macOS user policy = auth hardening
tmux/zsh = session continuity
stead = setup, status, connect, wake, uninstall automation
```

## Goals

- Configure a macOS host for secure OpenSSH access.
- Configure client machines to connect to that host.
- Support normal SSH-over-Tailscale without using Tailscale SSH.
- Prefer key-based SSH authentication.
- Disable password SSH on the host once key access is verified.
- Provide tmux-based session continuity.
- Provide optional Wake-on-LAN client flow.
- Be easy to install from a private/local git clone.
- Be easy to uninstall cleanly.

## Non-Goals

- No Tailscale SSH.
- No custom SSH daemon.
- No replacement for OpenSSH.
- No SSH authentication abstraction.
- No password storage.
- No cloud control plane.
- No public package-manager distribution requirement.
- No enterprise policy engine.

## Tailscale Policy

`stead` must never enable, configure, depend on, or abstract over Tailscale SSH.

Allowed Tailscale usage:

- Detect whether Tailscale is installed.
- Detect whether Tailscale appears running.
- Read Tailscale IP or MagicDNS metadata when safely available.
- Use that metadata to configure normal OpenSSH targets.

Disallowed Tailscale usage:

- Do not run `tailscale up --ssh`.
- Do not configure Tailscale SSH.
- Do not depend on Tailscale SSH ACLs for SSH authorization.
- Do not expose a `--tailscale-ssh` connect mode.
- Do not treat Tailscale identity as SSH authentication.

If Tailscale SSH is detected, `stead` should report it as external and unmanaged.

## CLI Shape

```bash
stead status
stead setup --alias devmac --dry-run
stead setup --alias devmac --dry-run --verify
stead verify --alias devmac
stead config path
stead config show
stead config init
stead config init --dry-run
stead doctor

stead host status
stead host authorize --public-key 'ssh-ed25519 ...' --alias devmac --dry-run
stead host unauthorize --public-key 'ssh-ed25519 ...' --alias devmac --dry-run
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
stead client unapply --alias devmac --dry-run
stead client uninstall

stead wake --alias devmac
stead connect --alias devmac
stead connect --alias devmac --wake
```

Example flags:

```bash
stead host install --user ed --tmux-session main
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
stead host harden --user ed --disable-password
stead client init --alias devmac --hostname <tailscale-ip-or-magicdns> --user ed --yes
stead client apply --dry-run --alias devmac
stead client apply --alias devmac
stead wake --alias devmac
stead connect --alias devmac --wake
```

## Host Mode

Host mode manages the trusted macOS machine as the SSH server.

It may:

- Check Remote Login / `sshd` state.
- Check launchd state for `com.openssh.sshd`.
- Check effective sshd configuration when permissions allow.
- Install or verify a public key in `~/.ssh/authorized_keys`.
- Create a managed sshd config drop-in.
- Install a managed tmux auto-attach block in `~/.zshrc`.
- Validate sshd configuration before applying hardening.
- Refuse to disable password auth until key-based access is verified or explicitly forced.

Target hardening:

```sshconfig
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
AllowUsers ed
```

Preferred host drop-in:

```text
/etc/ssh/sshd_config.d/stead.conf
```

The CLI should prefer owned drop-in files over editing Apple default files directly.

## Client Mode

Client mode manages connection convenience on client machines.

It may:

- Create or update a managed `~/.ssh/config` host entry.
- Generate or register a local SSH identity key.
- Detect Tailscale IP/MagicDNS metadata.
- Install optional wake/connect convenience behavior.
- Run the normal system `ssh` command.

Recommended setup flow:

```bash
stead setup --alias devmac --dry-run
stead client init --alias devmac --discover tailscale --yes
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
stead client apply --dry-run --alias devmac
stead client apply --alias devmac
stead verify --alias devmac
```

`stead client init` may prompt for the hostname when it is not supplied. For scripts, callers can provide all required fields explicitly:

```bash
stead client init --alias devmac --hostname devmac.tailnet.ts.net --user ed --yes
```

When Tailscale is installed on the client, `stead client init --discover tailscale` may call `tailscale status --json` to find a peer matching the alias. It should prefer the peer's MagicDNS name and fall back to its Tailscale IP. This requires a usable local Tailscale CLI. This is discovery only; it must not enable or use Tailscale SSH.

The hostname can be a Tailscale IP or MagicDNS name. This is not Tailscale SSH; it is only the network address used by normal OpenSSH.

Default client identity path:

```text
~/.ssh/stead_<alias>_ed25519
```

Generated private keys stay on the client machine. `stead` may print the generated public key so the user can install it on the host's `~/.ssh/authorized_keys`.

`stead host authorize` runs on the host Mac. It creates `~/.ssh` if needed, creates or updates `~/.ssh/authorized_keys`, and appends a client public key only when it is not already present.

Reversal commands should be precise:

```bash
stead client unapply --alias devmac --dry-run
stead host unauthorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
```

`client unapply` removes only the matching managed SSH config marker block. `host unauthorize` removes only matching public key material, regardless of comment.

`stead verify --alias devmac` should run a non-interactive SSH check such as `ssh -o BatchMode=yes devmac true`. It verifies that key-based login works without opening an interactive session or falling back to password prompts.

`stead setup --verify` should include that non-interactive SSH check in the setup plan. When verification succeeds, the plan can treat host authorization as proven and suggest `ssh <alias>` instead of repeating the host authorization handoff.

Example generated SSH config:

```sshconfig
Host devmac
    HostName <tailscale-ip-or-magicdns>
    User ed
    Port 22
    IdentityFile ~/.ssh/stead_ed25519
    IdentitiesOnly yes
    ServerAliveInterval 60
    ServerAliveCountMax 3
```

`stead connect --alias devmac` should normally exec:

```bash
ssh devmac
```

This keeps SSH transparent and leaves authentication to OpenSSH.

## Wake Flow

Client mode may support an optional wake flow:

```bash
stead wake --alias devmac
stead connect --alias devmac --wake
```

The wake flow should:

- Send a Wake-on-LAN magic packet if a MAC address is configured.
- Wait for the target SSH TCP port to become reachable.
- Then exec the normal system `ssh` command for connect flows.
- Never replace or participate in SSH authentication.

Example config:

```toml
[hosts.devmac]
hostname = "<tailscale-ip-or-magicdns>"
user = "ed"
port = 22
identity_file = "~/.ssh/stead_ed25519"

[hosts.devmac.wake]
mac_address = "<host-mac-address>"
broadcast = "<lan-broadcast-address>"
timeout = "90s"
interval = "2s"
```

Behavior:

```text
stead wake --alias devmac
  -> if host:port is already reachable, exit 0
  -> send Wake-on-LAN packet if mac_address is configured
  -> poll TCP host:port until reachable
  -> exit 0 when reachable
```

```text
stead connect --alias devmac --wake
  -> run wake flow
  -> exec system ssh using the configured alias
```

If no MAC address is configured, `stead wake` may skip packet sending and only wait for reachability.

## Session Continuity

Host mode may install a managed shell block that attaches SSH sessions to tmux:

```zsh
# >>> stead managed block: tmux auto-attach
if command -v tmux >/dev/null 2>&1 && [ -n "$PS1" ] && [ -n "$SSH_CONNECTION" ] && [ -z "$TMUX" ]; then
    exec tmux new-session -A -s main
fi
# <<< stead managed block: tmux auto-attach
```

This runs after SSH login. It is not part of authentication.

## Safety Model

All edits must be idempotent and reversible.

Use managed marker blocks for user files:

```text
# >>> stead managed block
...
# <<< stead managed block
```

For system files, prefer owned files that can be removed cleanly:

```text
/etc/ssh/sshd_config.d/stead.conf
```

Before disabling password authentication:

1. Install or verify the intended public key.
2. Validate file permissions.
3. Validate sshd configuration.
4. Require a successful key-auth test or explicit `--force`.
5. Apply hardening.
6. Print rollback instructions.

`stead uninstall` must remove only what `stead` created.

It should not delete user private keys by default. It may remove managed references and ask before deleting generated keys.

## CLI UX

CLI output should stay quiet, readable, and script-friendly.

- Use color only for status meaning: green `ok`, yellow warning or incomplete states, red failed or risky states.
- Enable color only for interactive terminal output.
- Disable color when output is redirected or `NO_COLOR` is set.
- Keep normal command output stable and copyable.
- Reserve animation for future long-running interactive waits such as wake/connect.

## Install Model

Private/local install is preferred:

```bash
git clone https://github.com/weirdry/stead.git ~/src/stead
cd ~/src/stead
./install.sh
```

Install targets:

```text
~/.local/bin/stead
~/.config/stead/config.toml
```

`./install.sh` builds the local checkout and copies the binary to `~/.local/bin/stead` by default. It must not modify SSH configuration, SSH keys, `authorized_keys`, Tailscale, launchd, or macOS settings.

No Homebrew formula or public package registry is required.

Preview:

```bash
./install.sh --dry-run
```

If needed, users can choose a different binary target:

```bash
STEAD_INSTALL_DIR="$HOME/bin" ./install.sh
```

Updates:

```bash
cd ~/src/stead
git pull
./install.sh
```

Uninstall:

```bash
./uninstall.sh --dry-run
./uninstall.sh
```

`./uninstall.sh` removes only the installed binary. It must leave `~/.config/stead/config.toml`, generated SSH keys, managed SSH config blocks, and `authorized_keys` entries alone. Those are handled by explicit stead commands such as `stead client unapply` and `stead host unauthorize`.

## Recommended Implementation

Use Go.

Reasons:

- Single binary.
- Good macOS support.
- Easy private distribution.
- Good standard library support for file parsing, TCP checks, and process execution.
- No runtime dependency.

Potential repo layout:

```text
stead/
  install.sh
  uninstall.sh
  justfile
  just/
    build.just
    check.just
    install.just
  docs/
    design.md
  cmd/stead/main.go
  internal/host/
  internal/client/
  internal/sshconfig/
  internal/managedfile/
  internal/tailscale/
  internal/wake/
  internal/tmux/
  internal/checks/
```

## Suggested Build Order

1. `stead status`
2. `stead host status`
3. `stead client status`
4. Managed file/block helpers
5. `stead client install`
6. `stead connect`
7. `stead wake`
8. `stead connect --wake`
9. `stead host install`
10. `stead host harden`
11. `stead uninstall`
