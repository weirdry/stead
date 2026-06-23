# From Scratch Setup

This guide sets up one client Mac to SSH into one host Mac using normal OpenSSH over Tailscale or LAN.

Tailscale SSH is not used.

## Assumptions

- Tailscale is already installed and connected on both Macs, or another private network path exists.
- macOS Remote Login / `sshd` is enabled on the host Mac.
- You can run commands on both the client Mac and the host Mac.
- `stead` is installed from a local clone on both Macs.

Install or update `stead` first:

```bash
cd ~/src/stead
git pull origin main
./install.sh
```

## 1. Inspect The Host Mac

Run on the host Mac:

```bash
stead host status
stead host status --effective
```

Confirm:

- Remote Login is enabled.
- `~/.ssh/authorized_keys` exists or can be created later.
- Tailscale network metadata shows an IP or a usable private hostname.

These commands are read-only. `--effective` asks `sshd` to evaluate its effective config without sudo; on macOS it may be unable to read host keys as a normal user.

## 2. Choose A Client Alias

Pick the name you want to type from the client Mac:

```text
devmac
```

The alias is local to the client SSH config. It does not have to match the host computer name.

## 3. Identify The Host Address

Use one of:

- Tailscale MagicDNS name
- Tailscale IP
- LAN hostname
- LAN IP

Example placeholder:

```text
devmac.tailnet.example.ts.net
```

This address is only the network target for normal OpenSSH. It is not Tailscale SSH.

## 4. Initialize Client Config And Key

Run on the client Mac:

```bash
stead client init --alias devmac --hostname <host-tailscale-name-or-ip> --yes
```

This creates or updates:

```text
~/.config/stead/config.toml
~/.ssh/stead_devmac_ed25519
~/.ssh/stead_devmac_ed25519.pub
```

The private key stays on the client Mac. The command prints the public key so it can be authorized on the host.

Preview first if desired:

```bash
stead client init --alias devmac --hostname <host-tailscale-name-or-ip> --dry-run --yes
```

## 5. Authorize The Client Key On The Host

Copy the public key printed by `stead client init`.

Run on the host Mac:

```bash
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host authorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
```

This appends the public key to:

```text
~/.ssh/authorized_keys
```

It is idempotent. If the key already exists, no duplicate is added.

## 6. Install The Client SSH Alias

Run on the client Mac:

```bash
stead client apply --dry-run --alias devmac
stead client apply --alias devmac
```

This adds a managed block to:

```text
~/.ssh/config
```

Only the matching managed block is owned by `stead`.

## 7. Verify Key Login

Run on the client Mac:

```bash
stead verify --alias devmac
```

This runs a non-interactive OpenSSH check using `BatchMode=yes`. It verifies key login without prompting for a password.

You can also run:

```bash
stead setup --alias devmac --dry-run --verify
```

When everything is complete, it should report SSH login as OK and suggest:

```bash
ssh devmac
```

## 8. Connect

Run on the client Mac:

```bash
ssh devmac
stead connect --alias devmac
```

The actual SSH transport and authentication are still handled by the system `ssh` command and macOS `sshd`.

## 9. Preview Host Hardening

After key login works, preview the host sshd hardening drop-in:

```bash
stead host harden --dry-run --user ed --disable-password
```

The dry run shows the proposed `/etc/ssh/sshd_config.d/stead.conf` content but does not install it or reload sshd.

To apply host hardening later, keep a local host session open and run:

```bash
sudo stead host harden --apply --user ed --disable-password --confirm-key-login
```

This writes the managed drop-in after validation, but still does not reload sshd automatically.

Plan the reload path:

```bash
stead host validate
stead host reload --dry-run
```

Apply the reload only after keeping a local host session open and confirming client key login still works:

```bash
sudo stead host reload --apply --confirm
```

## Undo

Remove the managed client SSH config block:

```bash
stead client unapply --alias devmac --dry-run
stead client unapply --alias devmac
```

Remove the authorized public key from the host:

```bash
stead host unauthorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac' --dry-run
stead host unauthorize --alias devmac --public-key 'ssh-ed25519 ... stead devmac'
```

Remove host hardening:

```bash
stead host harden --unapply --dry-run
sudo stead host harden --unapply --apply --confirm
sudo stead host reload --apply --confirm
```

Uninstall only the `stead` binary:

```bash
./uninstall.sh --dry-run
./uninstall.sh
```
