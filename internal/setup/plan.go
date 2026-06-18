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

	ui.PrintTitle(out, "Stead setup")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Alias", alias)
	ui.PrintKV(out, "Mode", "dry-run (no files changed)")
	ui.PrintKV(out, "SSH", "normal OpenSSH; Tailscale SSH is not used")
	fmt.Fprintln(out)

	steps := make([]string, 0)

	ui.PrintSection(out, "Client config")
	var host *config.Host
	if cfgErr != nil {
		if errors.Is(cfgErr, os.ErrNotExist) {
			ui.PrintKV(out, "Config", ui.StateDetail(out, "missing", cfgPath))
			steps = append(steps, "stead client init --alias "+alias+" --discover tailscale --yes")
		} else {
			return cfgErr
		}
	} else {
		ui.PrintKV(out, "Config", ui.StateDetail(out, "ok", cfgPath))
		host = cfg.Hosts[alias]
		if host == nil {
			ui.PrintKV(out, "Host", ui.State(out, "missing"))
			steps = append(steps, "stead client init --alias "+alias+" --discover tailscale --yes")
		} else {
			ui.PrintKV(out, "Host", ui.State(out, "ok"))
			ui.PrintKV(out, "Hostname", valueOrUnset(host.Hostname))
			ui.PrintKV(out, "User", valueOrUnset(host.User))
			ui.PrintKV(out, "IdentityFile", valueOrUnset(host.IdentityFile))
		}
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Client key")
	publicKey := ""
	if host == nil || host.IdentityFile == "" {
		ui.PrintKV(out, "Key", "unknown until client init")
	} else {
		keyPath, err := expandHome(host.IdentityFile)
		if err != nil {
			return err
		}
		pubPath := keyPath + ".pub"
		if fileExists(keyPath) {
			ui.PrintKV(out, "Private key", ui.StateDetail(out, "ok", host.IdentityFile))
		} else {
			ui.PrintKV(out, "Private key", ui.StateDetail(out, "missing", host.IdentityFile))
			steps = append(steps, clientInitStep(alias, host.Hostname))
		}
		if data, err := os.ReadFile(pubPath); err == nil {
			publicKey = strings.TrimSpace(string(data))
			ui.PrintKV(out, "Public key", ui.StateDetail(out, "ok", host.IdentityFile+".pub"))
		} else if errors.Is(err, os.ErrNotExist) {
			ui.PrintKV(out, "Public key", ui.StateDetail(out, "missing", host.IdentityFile+".pub"))
		} else {
			return err
		}
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "SSH alias")
	aliasState := sshAliasState(sshConfigPath, alias)
	ui.PrintKV(out, "~/.ssh/config", colorStatePrefix(out, aliasState.Config, statePrefix(aliasState.Config)))
	ui.PrintKV(out, "Host "+alias, colorStatePrefix(out, aliasState.Host, statePrefix(aliasState.Host)))
	if aliasState.Host != "ok" {
		steps = append(steps, "stead client apply --dry-run --alias "+alias)
		steps = append(steps, "stead client apply --alias "+alias)
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Host authorization")
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
		ui.PrintKV(out, "Public key handoff", "pending until public key exists")
	} else if verified {
		ui.PrintKV(out, "SSH login", ui.State(out, "ok"))
	} else {
		if opts.Verify && verifyFailed {
			ui.PrintKV(out, "SSH login", ui.State(out, "failed"))
		}
		ui.PrintKV(out, "Public key handoff", "run on the host if not already authorized")
		steps = append(steps, "stead host authorize --alias "+alias+" --public-key "+shellQuote(publicKey)+" --dry-run")
		steps = append(steps, "stead host authorize --alias "+alias+" --public-key "+shellQuote(publicKey))
		if aliasState.Host == "ok" {
			steps = append(steps, "stead verify --alias "+alias)
		}
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Next steps")
	if len(steps) == 0 {
		ui.PrintKV(out, "Status", "ready")
		ui.PrintStep(out, 1, "ssh "+alias)
		return nil
	}
	ui.PrintKV(out, "Order", "run in order")
	for i, step := range unique(steps) {
		ui.PrintStep(out, i+1, step)
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
