package clientuninstall

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ed/stead/internal/clientconfig"
	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/ui"
)

type Options struct {
	Alias         string
	ConfigPath    string
	SSHConfigPath string
	DryRun        bool
	Apply         bool
	Confirm       bool
	Out           io.Writer
}

func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	if opts.DryRun == opts.Apply {
		return fmt.Errorf("client uninstall requires exactly one of --dry-run or --apply")
	}
	if opts.Apply && !opts.Confirm {
		return fmt.Errorf("client uninstall --apply requires --confirm")
	}

	cfg, cfgPath, cfgErr := loadConfig(opts.ConfigPath)
	alias := opts.Alias
	if alias == "" && cfg != nil {
		alias = cfg.Defaults.Alias
	}
	if alias == "" {
		return fmt.Errorf("--alias is required")
	}
	sshConfigPath, err := defaultSSHConfigPath(opts.SSHConfigPath)
	if err != nil {
		return err
	}

	host := (*config.Host)(nil)
	if cfgErr == nil {
		host = cfg.Hosts[alias]
	} else if !errors.Is(cfgErr, os.ErrNotExist) {
		return cfgErr
	}

	ui.PrintTitle(out, "Stead client uninstall")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Alias", alias)
	ui.PrintKV(out, "Config", configDetail(cfgErr, cfgPath))
	ui.PrintKV(out, "SSH config", sshConfigPath)
	if opts.DryRun {
		ui.PrintKV(out, "Mode", "dry-run (no files changed)")
	} else {
		ui.PrintKV(out, "Mode", "apply")
	}
	fmt.Fprintln(out)

	existing, existed, mode, err := readExisting(sshConfigPath)
	if err != nil {
		return err
	}
	change, err := clientconfig.PlanRemoval(existing, sshConfigPath, alias)
	if err != nil {
		return err
	}

	ui.PrintSection(out, "Changes")
	switch change.State {
	case "remove":
		if opts.DryRun {
			ui.PrintKV(out, "SSH config block", "would remove managed block")
		} else {
			ui.PrintKV(out, "SSH config block", "removed managed block")
		}
	case "absent":
		ui.PrintKV(out, "SSH config block", "no changes needed")
	default:
		ui.PrintKV(out, "SSH config block", change.State)
	}

	if opts.DryRun {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "No files were modified.")
	} else if change.State == "remove" {
		if !existed {
			return fmt.Errorf("SSH config does not exist")
		}
		backupPath := sshConfigPath + ".stead-backup-" + time.Now().UTC().Format("20060102T150405.000000000Z")
		if err := os.WriteFile(backupPath, existing, mode); err != nil {
			return err
		}
		if err := writeAtomic(sshConfigPath, change.NewContent, mode); err != nil {
			return err
		}
		ui.PrintKV(out, "Backup", backupPath)
	} else {
		fmt.Fprintln(out, "No files were modified.")
	}

	fmt.Fprintln(out)
	ui.PrintSection(out, "Left untouched")
	ui.PrintKV(out, "Stead config", cfgPath)
	if host == nil {
		ui.PrintKV(out, "Configured host", ui.State(out, "missing"))
	} else {
		ui.PrintKV(out, "Configured host", ui.State(out, "present"))
		printIdentity(out, host.IdentityFile)
	}
	fmt.Fprintln(out)
	ui.PrintSection(out, "Next steps")
	ui.PrintStep(out, 1, "Remove config/key files manually only if you no longer need them")
	ui.PrintStep(out, 2, "Run host unauthorize on the host if this client key should no longer log in")
	return nil
}

func configDetail(err error, path string) string {
	if err == nil {
		return "ok (" + path + ")"
	}
	if errors.Is(err, os.ErrNotExist) {
		return "missing (" + path + ")"
	}
	return "unknown (" + err.Error() + ")"
}

func readExisting(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, 0o600, nil
		}
		return nil, false, 0, err
	}
	if info.IsDir() {
		return nil, false, 0, fmt.Errorf("%s is a directory", path)
	}
	data, err := os.ReadFile(path)
	return data, true, info.Mode().Perm(), err
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".stead-ssh-config-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	remove = false
	return nil
}

func printIdentity(out io.Writer, identityFile string) {
	if strings.TrimSpace(identityFile) == "" {
		ui.PrintKV(out, "IdentityFile", ui.State(out, "missing"))
		return
	}
	ui.PrintKV(out, "IdentityFile", identityFile)
	keyPath, err := expandHome(identityFile)
	if err != nil {
		ui.PrintKV(out, "Private key", ui.StateDetail(out, "unknown", err.Error()))
		return
	}
	ui.PrintKV(out, "Private key", fileState(out, keyPath))
	ui.PrintKV(out, "Public key", fileState(out, keyPath+".pub"))
}

func fileState(out io.Writer, path string) string {
	_, err := os.Stat(path)
	if err == nil {
		return ui.StateDetail(out, "present", path)
	}
	if errors.Is(err, os.ErrNotExist) {
		return ui.StateDetail(out, "missing", path)
	}
	return ui.StateDetail(out, "unknown", err.Error())
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
	return clientconfig.DefaultSSHConfigPath(home), nil
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
