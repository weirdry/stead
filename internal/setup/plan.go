package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/sshconfig"
	"github.com/ed/stead/internal/ui"
	"github.com/ed/stead/internal/verify"
)

type Options struct {
	Alias         string
	ConfigPath    string
	SSHConfigPath string
	Verify        bool
	VerifyRunner  verify.Runner
	Out           io.Writer
}

func WritePlan(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	cfg, cfgPath, cfgErr := loadConfig(opts.ConfigPath)
	alias := opts.Alias
	if alias == "" && cfg != nil {
		alias = cfg.Defaults.Alias
	}
	if alias == "" {
		alias = "devmac"
	}

	sshConfigPath, err := defaultSSHConfigPath(opts.SSHConfigPath)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "Stead setup")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Alias: %s\n", alias)
	fmt.Fprintln(out, "Mode: dry-run (no files changed)")
	fmt.Fprintln(out, "SSH: normal OpenSSH; Tailscale SSH is not used")
	fmt.Fprintln(out)

	steps := make([]string, 0)

	fmt.Fprintln(out, "Client config")
	var host *config.Host
	if cfgErr != nil {
		if errors.Is(cfgErr, os.ErrNotExist) {
			fmt.Fprintf(out, "  Config: %s (%s)\n", ui.State(out, "missing"), cfgPath)
			steps = append(steps, "stead client init --alias "+alias+" --discover tailscale --yes")
		} else {
			return cfgErr
		}
	} else {
		fmt.Fprintf(out, "  Config: %s (%s)\n", ui.State(out, "ok"), cfgPath)
		host = cfg.Hosts[alias]
		if host == nil {
			fmt.Fprintf(out, "  Host: %s\n", ui.State(out, "missing"))
			steps = append(steps, "stead client init --alias "+alias+" --discover tailscale --yes")
		} else {
			fmt.Fprintf(out, "  Host: %s\n", ui.State(out, "ok"))
			fmt.Fprintf(out, "  Hostname: %s\n", valueOrUnset(host.Hostname))
			fmt.Fprintf(out, "  User: %s\n", valueOrUnset(host.User))
			fmt.Fprintf(out, "  IdentityFile: %s\n", valueOrUnset(host.IdentityFile))
		}
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Client key")
	publicKey := ""
	if host == nil || host.IdentityFile == "" {
		fmt.Fprintln(out, "  Key: unknown until client init")
	} else {
		keyPath, err := expandHome(host.IdentityFile)
		if err != nil {
			return err
		}
		pubPath := keyPath + ".pub"
		if fileExists(keyPath) {
			fmt.Fprintf(out, "  Private key: %s (%s)\n", ui.State(out, "ok"), host.IdentityFile)
		} else {
			fmt.Fprintf(out, "  Private key: %s (%s)\n", ui.State(out, "missing"), host.IdentityFile)
			steps = append(steps, clientInitStep(alias, host.Hostname))
		}
		if data, err := os.ReadFile(pubPath); err == nil {
			publicKey = strings.TrimSpace(string(data))
			fmt.Fprintf(out, "  Public key: %s (%s.pub)\n", ui.State(out, "ok"), host.IdentityFile)
		} else if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "  Public key: %s (%s.pub)\n", ui.State(out, "missing"), host.IdentityFile)
		} else {
			return err
		}
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "SSH alias")
	aliasState := sshAliasState(sshConfigPath, alias)
	fmt.Fprintf(out, "  ~/.ssh/config: %s\n", colorStatePrefix(out, aliasState.Config, statePrefix(aliasState.Config)))
	fmt.Fprintf(out, "  Host %s: %s\n", alias, colorStatePrefix(out, aliasState.Host, statePrefix(aliasState.Host)))
	if aliasState.Host != "ok" {
		steps = append(steps, "stead client apply --dry-run --alias "+alias)
		steps = append(steps, "stead client apply --alias "+alias)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Host authorization")
	verified := false
	verifyFailed := false
	if opts.Verify && aliasState.Host == "ok" {
		ok, err := verifyAlias(alias, opts.VerifyRunner)
		if err != nil {
			return err
		}
		verified = ok
		verifyFailed = !ok
	}
	if publicKey == "" {
		fmt.Fprintln(out, "  Public key handoff: pending until public key exists")
	} else if verified {
		fmt.Fprintf(out, "  SSH login: %s\n", ui.State(out, "ok"))
	} else {
		if opts.Verify && verifyFailed {
			fmt.Fprintf(out, "  SSH login: %s\n", ui.State(out, "failed"))
		}
		fmt.Fprintln(out, "  Public key handoff: run on the host if not already authorized")
		steps = append(steps, "stead host authorize --alias "+alias+" --public-key "+shellQuote(publicKey)+" --dry-run")
		steps = append(steps, "stead host authorize --alias "+alias+" --public-key "+shellQuote(publicKey))
		if aliasState.Host == "ok" {
			steps = append(steps, "stead verify --alias "+alias)
		}
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Next steps")
	if len(steps) == 0 {
		fmt.Fprintln(out, "  Ready")
		fmt.Fprintf(out, "  1. ssh %s\n", alias)
		return nil
	}
	fmt.Fprintln(out, "  Run in order")
	for i, step := range unique(steps) {
		fmt.Fprintf(out, "  %d. %s\n", i+1, step)
	}
	return nil
}

func verifyAlias(alias string, runner verify.Runner) (bool, error) {
	if runner == nil {
		runner = verify.SSHRunner
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := runner(ctx, alias)
	if err == nil {
		return true, nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return false, nil
	}
	return false, nil
}

func colorStatePrefix(out io.Writer, detail, state string) string {
	colored := ui.State(out, state)
	if colored == state {
		return detail
	}
	return colored + strings.TrimPrefix(detail, state)
}

func statePrefix(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return value
	}
	return fields[0]
}

type aliasState struct {
	Config string
	Host   string
}

func sshAliasState(path, alias string) aliasState {
	cfg, err := sshconfig.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return aliasState{Config: "missing", Host: "missing"}
		}
		return aliasState{Config: "error: " + err.Error(), Host: "unknown"}
	}
	status := sshconfig.CheckAlias(cfg, alias)
	if status.State == "ok" {
		return aliasState{Config: "ok", Host: "ok"}
	}
	if len(status.Findings) == 0 {
		return aliasState{Config: "ok", Host: status.State}
	}
	return aliasState{Config: "ok", Host: status.State + " (" + strings.Join(status.Findings, ", ") + ")"}
}

func loadConfig(path string) (*config.Config, string, error) {
	if path == "" {
		cfg, cfgPath, err := config.LoadDefault()
		return cfg, cfgPath, err
	}
	cfg, err := config.Load(path)
	return cfg, path, err
}

func defaultSSHConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

func expandHome(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func valueOrUnset(value string) string {
	if value == "" {
		return "(unset)"
	}
	return value
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func clientInitStep(alias, hostname string) string {
	if hostname == "" || isPlaceholder(hostname) {
		return "stead client init --alias " + alias + " --discover tailscale --yes"
	}
	return "stead client init --alias " + alias + " --hostname " + shellQuote(hostname) + " --yes"
}

func isPlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")
}

func unique(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
