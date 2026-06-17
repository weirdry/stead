package clientinit

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ed/stead/internal/config"
)

type Options struct {
	Alias        string
	Hostname     string
	User         string
	IdentityFile string
	ConfigPath   string
	DryRun       bool
	Yes          bool
	In           io.Reader
	Out          io.Writer
	Keygen       Keygen
}

type Keygen func(path, comment string) error

func Run(opts Options) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.In == nil {
		opts.In = strings.NewReader("")
	}
	if opts.Keygen == nil {
		opts.Keygen = SSHKeygen
	}

	alias := valueOrDefault(opts.Alias, "devmac")
	localUser := valueOrDefault(opts.User, currentUserName())
	identityFile := valueOrDefault(opts.IdentityFile, "~/.ssh/stead_"+alias+"_ed25519")
	hostname, err := hostname(opts)
	if err != nil {
		return err
	}

	cfg, path, existed, err := loadOrNew(opts.ConfigPath)
	if err != nil {
		return err
	}
	if cfg.Defaults.Alias == "" {
		cfg.Defaults.Alias = alias
	}
	cfg.Hosts[alias] = &config.Host{
		Hostname:         hostname,
		User:             localUser,
		Port:             22,
		IdentityFile:     identityFile,
		PreferredNetwork: "tailscale",
		Wake: config.Wake{
			MACAddress: "<host-mac-address>",
			Broadcast:  "<lan-broadcast-address>",
			Timeout:    "90s",
			Interval:   "2s",
		},
		Session: config.Session{
			Tmux:        true,
			TmuxSession: "main",
			ProjectDir:  starterProjectDir(),
		},
	}

	keyPath, err := expandHome(identityFile)
	if err != nil {
		return err
	}
	keyExists := fileExists(keyPath)
	publicKeyPath := keyPath + ".pub"

	fmt.Fprintln(opts.Out, "Stead client init")
	fmt.Fprintln(opts.Out)
	fmt.Fprintf(opts.Out, "Config: %s\n", path)
	fmt.Fprintf(opts.Out, "Alias: %s\n", alias)
	fmt.Fprintf(opts.Out, "Hostname: %s\n", hostname)
	fmt.Fprintf(opts.Out, "User: %s\n", localUser)
	fmt.Fprintf(opts.Out, "IdentityFile: %s\n", identityFile)
	if opts.DryRun {
		fmt.Fprintln(opts.Out, "Mode: dry-run")
	} else {
		fmt.Fprintln(opts.Out, "Mode: apply")
	}
	fmt.Fprintln(opts.Out)

	if opts.DryRun {
		if existed {
			fmt.Fprintln(opts.Out, "Config action: would update config")
		} else {
			fmt.Fprintln(opts.Out, "Config action: would create config")
		}
		if keyExists {
			fmt.Fprintln(opts.Out, "Key action: no changes needed")
		} else {
			fmt.Fprintf(opts.Out, "Key action: would generate Ed25519 key at %s\n", identityFile)
		}
		fmt.Fprintln(opts.Out, "No files were modified.")
		return nil
	}

	generated := false
	if keyExists {
		fmt.Fprintln(opts.Out, "Key action: no changes needed")
	} else {
		if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
			return err
		}
		if err := opts.Keygen(keyPath, "stead "+alias); err != nil {
			return err
		}
		generated = true
	}

	if err := config.Save(path, cfg); err != nil {
		return err
	}
	if generated {
		fmt.Fprintf(opts.Out, "Key action: generated Ed25519 key at %s\n", identityFile)
	}
	if publicKey, err := os.ReadFile(publicKeyPath); err == nil {
		fmt.Fprintln(opts.Out)
		fmt.Fprintln(opts.Out, "Public key")
		fmt.Fprint(opts.Out, string(publicKey))
	}
	fmt.Fprintln(opts.Out)
	fmt.Fprintln(opts.Out, "Next: run stead client apply --dry-run --alias "+alias)
	return nil
}

func SSHKeygen(path, comment string) error {
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", path, "-N", "", "-C", comment)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func hostname(opts Options) (string, error) {
	if opts.Hostname != "" {
		return opts.Hostname, nil
	}
	if opts.Yes {
		return "", fmt.Errorf("--hostname is required with --yes")
	}
	fmt.Fprint(opts.Out, "Hostname: ")
	line, err := bufio.NewReader(opts.In).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", fmt.Errorf("hostname is required")
	}
	return line, nil
}

func loadOrNew(path string) (*config.Config, string, bool, error) {
	if path == "" {
		path = config.DefaultPath()
	}
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, path, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return &config.Config{Hosts: make(map[string]*config.Host)}, path, false, nil
	}
	return nil, path, false, err
}

func expandHome(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		return home, err
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

func valueOrDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func currentUserName() string {
	u, err := user.Current()
	if err != nil || u == nil || u.Username == "" {
		return "local-user"
	}
	return filepath.Base(u.Username)
}

func starterProjectDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "~/project"
	}
	return filepath.Join(home, "_GIT")
}
