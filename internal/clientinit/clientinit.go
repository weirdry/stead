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
	"github.com/ed/stead/internal/tailscale"
)

type Options struct {
	Alias        string
	Hostname     string
	User         string
	IdentityFile string
	ConfigPath   string
	Discover     string
	DryRun       bool
	Yes          bool
	In           io.Reader
	Out          io.Writer
	Keygen       Keygen
	Discoverer   Discoverer
}

type Keygen func(path, comment string) error
type Discoverer func(alias string) (tailscale.Peer, error)

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
	if opts.Discoverer == nil {
		opts.Discoverer = tailscale.DiscoverPeer
	}

	alias := valueOrDefault(opts.Alias, "devmac")
	localUser := valueOrDefault(opts.User, currentUserName())
	identityFile := valueOrDefault(opts.IdentityFile, "~/.ssh/stead_"+alias+"_ed25519")
	hostname, discovered, err := hostname(opts, alias)
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
	if discovered.HostName != "" || discovered.IP != "" {
		fmt.Fprintf(opts.Out, "Discovered via Tailscale: %s %s\n", valueOrUnset(discovered.HostName), valueOrUnset(discovered.IP))
	}
	fmt.Fprintf(opts.Out, "User: %s\n", localUser)
	fmt.Fprintf(opts.Out, "IdentityFile: %s\n", identityFile)
	if opts.DryRun {
		fmt.Fprintln(opts.Out, "Mode: dry-run (no files changed)")
	} else {
		fmt.Fprintln(opts.Out, "Mode: apply")
	}
	fmt.Fprintln(opts.Out, "SSH: normal OpenSSH; Tailscale SSH is not used")
	fmt.Fprintln(opts.Out)

	fmt.Fprintln(opts.Out, "Actions")
	if opts.DryRun {
		if existed {
			fmt.Fprintln(opts.Out, "  Config action: would update config")
		} else {
			fmt.Fprintln(opts.Out, "  Config action: would create config")
		}
		if keyExists {
			fmt.Fprintln(opts.Out, "  Key action: no changes needed")
		} else {
			fmt.Fprintf(opts.Out, "  Key action: would generate Ed25519 key at %s\n", identityFile)
		}
		fmt.Fprintln(opts.Out)
		fmt.Fprintln(opts.Out, "No files were modified.")
		fmt.Fprintln(opts.Out)
		fmt.Fprintln(opts.Out, "Next steps")
		fmt.Fprintln(opts.Out, "  1. Re-run without --dry-run when the plan looks right")
		return nil
	}

	generated := false
	if keyExists {
		fmt.Fprintln(opts.Out, "  Key action: no changes needed")
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
	if existed {
		fmt.Fprintln(opts.Out, "  Config action: updated config")
	} else {
		fmt.Fprintln(opts.Out, "  Config action: created config")
	}
	if generated {
		fmt.Fprintf(opts.Out, "  Key action: generated Ed25519 key at %s\n", identityFile)
	}
	if publicKey, err := os.ReadFile(publicKeyPath); err == nil {
		fmt.Fprintln(opts.Out)
		fmt.Fprintln(opts.Out, "Public key for host authorization")
		fmt.Fprint(opts.Out, string(publicKey))
	}
	fmt.Fprintln(opts.Out)
	fmt.Fprintln(opts.Out, "Next steps")
	fmt.Fprintln(opts.Out, "  1. Run the shown public key through stead host authorize on the host Mac")
	fmt.Fprintln(opts.Out, "  2. stead client apply --dry-run --alias "+alias)
	fmt.Fprintln(opts.Out, "  3. stead client apply --alias "+alias)
	fmt.Fprintln(opts.Out, "  4. stead setup --alias "+alias+" --dry-run --verify")
	return nil
}

func SSHKeygen(path, comment string) error {
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", path, "-N", "", "-C", comment)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func hostname(opts Options, alias string) (string, tailscale.Peer, error) {
	if opts.Hostname != "" {
		return opts.Hostname, tailscale.Peer{}, nil
	}
	if opts.Discover != "" {
		if opts.Discover != "tailscale" {
			return "", tailscale.Peer{}, fmt.Errorf("unsupported discovery source %q", opts.Discover)
		}
		peer, err := opts.Discoverer(alias)
		if err != nil {
			return "", tailscale.Peer{}, err
		}
		host := peer.Hostname()
		if host == "" {
			return "", tailscale.Peer{}, fmt.Errorf("Tailscale peer %q has no usable hostname or IP", alias)
		}
		return host, peer, nil
	}
	if opts.Yes {
		return "", tailscale.Peer{}, fmt.Errorf("--hostname or --discover is required with --yes")
	}
	fmt.Fprint(opts.Out, "Hostname: ")
	line, err := bufio.NewReader(opts.In).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", tailscale.Peer{}, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", tailscale.Peer{}, fmt.Errorf("hostname is required")
	}
	return line, tailscale.Peer{}, nil
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

func valueOrUnset(value string) string {
	if value == "" {
		return "(unset)"
	}
	return value
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
