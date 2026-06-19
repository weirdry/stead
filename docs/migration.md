# Existing Setup Migration

Use this guide when SSH already works and you want to bring it under `stead` gradually.

The safest path is to create a new managed alias first, verify it, and only then decide whether to remove or ignore older manual config.

## 1. Inspect Both Machines

On the host Mac:

```bash
stead host status
```

On the client Mac:

```bash
stead client status
```

These commands are read-only.

## 2. Keep The Existing Alias Working

If `ssh devmac` already works, do not overwrite it immediately.

Create a separate managed alias:

```text
stead-devmac
```

This lets you test `stead` without breaking the current workflow.

## 3. Reuse The Known Host Address

If an existing alias resolves correctly, you can inspect it from the client Mac:

```bash
ssh -G devmac | awk '/^hostname / {print $2; exit}'
```

Use the resolved hostname or IP for the new managed alias.

## 4. Initialize The Managed Alias

Run on the client Mac:

```bash
stead client init --alias stead-devmac --hostname <resolved-hostname-or-ip> --yes
```

This creates a separate key:

```text
~/.ssh/stead_stead-devmac_ed25519
```

It does not modify the old `devmac` alias.

## 5. Authorize The New Key On The Host

Copy the public key printed by `stead client init`.

Run on the host Mac:

```bash
stead host authorize --alias stead-devmac --public-key 'ssh-ed25519 ... stead stead-devmac' --dry-run
stead host authorize --alias stead-devmac --public-key 'ssh-ed25519 ... stead stead-devmac'
```

## 6. Apply The Managed Client Alias

Run on the client Mac:

```bash
stead client apply --dry-run --alias stead-devmac
stead client apply --alias stead-devmac
```

This adds only a `stead` managed block for `stead-devmac` in `~/.ssh/config`.

## 7. Verify Before Trusting It

Run on the client Mac:

```bash
stead verify --alias stead-devmac
stead setup --alias stead-devmac --dry-run --verify
ssh stead-devmac
```

When verification succeeds, the new managed alias is working independently from the old alias.

## 8. Decide What To Do With The Old Setup

Options:

- Keep the old alias as a fallback.
- Stop using it but leave it in place.
- Manually remove old non-stead SSH config after confirming it is not needed.

`stead client unapply` removes only `stead` managed blocks. It will not remove arbitrary pre-existing `Host devmac` entries.

## Rollback

On the client Mac:

```bash
stead client unapply --alias stead-devmac --dry-run
stead client unapply --alias stead-devmac
```

On the host Mac:

```bash
stead host unauthorize --alias stead-devmac --public-key 'ssh-ed25519 ... stead stead-devmac' --dry-run
stead host unauthorize --alias stead-devmac --public-key 'ssh-ed25519 ... stead stead-devmac'
```

The old SSH setup remains untouched unless you change it manually.
