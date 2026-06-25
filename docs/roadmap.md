# Roadmap

This project is functionally complete for its original purpose: reproducible personal SSH access to a trusted Mac using normal OpenSSH over Tailscale or LAN.

`stead` should stay small. Future work should improve reliability, clarity, and maintenance without changing the core architecture:

```text
Tailscale or LAN = reachability
OpenSSH = transport and authentication
macOS user policy = account authorization
tmux/zsh = session continuity
stead = setup, status, verify, connect, wake, and cleanup automation
```

Tailscale SSH remains out of scope.

## Current Done Boundary

The core workflow is implemented:

- host status and SSH hardening checks
- client config initialization and SSH alias management
- client key generation
- host public-key authorization and removal
- host hardening apply, unapply, validate, and reload helpers
- read-only setup planning and doctor diagnostics
- BatchMode SSH verification
- normal OpenSSH connect command
- optional Wake-on-LAN flow
- tmux auto-attach install and uninstall helpers
- conservative client uninstall flow
- local install and uninstall through `just`
- git-derived version metadata for local builds and installs
- user-facing docs for setup, commands, config, safety, troubleshooting, and migration

## Optional Future Work

### Integration Testing

Most behavior is unit-tested. Real SSH behavior is still best verified manually across two Macs.

Potential work:

- document a repeatable two-Mac smoke-test checklist
- add safer scripted checks for commands that do not mutate system state
- keep privileged host hardening tests manual unless a dedicated test environment exists

### UX Copy Review

The CLI output is stable and readable. Future usage may reveal wording that can be clearer.

Potential work:

- tighten command success/failure language after repeated real use
- keep status output concise
- avoid adding animation or non-copyable output to operational commands

### Cleanup Policy

Current cleanup behavior is intentionally conservative. `stead client uninstall` does not delete private keys or config files.

Potential work:

- consider explicit opt-in flags for removing generated client keys
- consider explicit opt-in flags for removing host config entries
- keep destructive cleanup behind dry-run, confirmation, and narrow targeting

### Packaging

Public package managers are intentionally not part of the design.

Potential work:

- keep local git clone plus `just install` as the primary install path
- consider private archive artifacts only if manual clone/install becomes inconvenient
- avoid adding Homebrew or public registry dependency unless the project scope changes

## Non-Goals

- enabling, configuring, or abstracting over Tailscale SSH
- replacing OpenSSH authentication
- managing Tailscale ACLs or auth keys
- deleting private keys by default
- broad system management beyond the SSH/tmux pieces described in the docs
