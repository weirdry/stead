package connect

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/sshconfig"
	"github.com/ed/stead/internal/ui"
)

type Options struct {
	Alias         string
	ConfigPath    string
	SSHConfigPath string
	Out           io.Writer
	Exec          ExecFunc
}

type ExecFunc func(path string, argv []string, env []string) error

func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	cfg, cfgPath, err := loadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}
	alias := opts.Alias
	if alias == "" {
		alias = cfg.Defaults.Alias
	}
	if alias == "" {
		return fmt.Errorf("--alias is required")
	}
	if cfg.Hosts[alias] == nil {
		return fmt.Errorf("alias %q is not configured in %s", alias, cfgPath)
	}

	sshConfigPath, err := defaultSSHConfigPath(opts.SSHConfigPath)
	if err != nil {
		return err
	}
	sshCfg, err := sshconfig.Load(sshConfigPath)
	if err != nil {
		return err
	}
	aliasStatus := sshconfig.CheckAlias(sshCfg, alias)
	if aliasStatus.State != "ok" {
		return fmt.Errorf("ssh alias %q is %s; run stead client apply --alias %s", alias, aliasStatus.State, alias)
	}

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return err
	}

	ui.PrintTitle(out, "Stead connect")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Alias", alias)
	ui.PrintKV(out, "SSH", sshPath)
	ui.PrintKV(out, "Auth", "system OpenSSH; Tailscale SSH is not used")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Exec", "ssh "+alias)

	execFn := opts.Exec
	if execFn == nil {
		execFn = syscall.Exec
	}
	return execFn(sshPath, []string{sshPath, alias}, os.Environ())
}

func loadConfig(path string) (*config.Config, string, error) {
	if path == "" {
		return config.LoadDefault()
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
