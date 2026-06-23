package doctor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/hostharden"
	"github.com/ed/stead/internal/sshconfig"
	"github.com/ed/stead/internal/ui"
	"github.com/ed/stead/internal/verify"
)

type Options struct {
	Alias         string
	ConfigPath    string
	SSHConfigPath string
	Verify        bool
	Out           io.Writer
	Runner        verify.Runner
}

type item struct {
	Label  string
	State  string
	Detail string
}

func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	report, err := collect(opts)
	if err != nil {
		return err
	}
	print(out, report)
	return nil
}

type report struct {
	Alias string
	Items []item
	Steps []string
}

func collect(opts Options) (report, error) {
	cfg, cfgPath, cfgErr := loadConfig(opts.ConfigPath)
	alias := opts.Alias
	if alias == "" && cfg != nil {
		alias = cfg.Defaults.Alias
	}
	if alias == "" {
		alias = "devmac"
	}

	r := report{Alias: alias}
	if cfgErr != nil {
		if errors.Is(cfgErr, os.ErrNotExist) {
			r.Items = append(r.Items, item{"Stead config", "missing", cfgPath})
			r.Steps = append(r.Steps, "stead config init")
			r.Steps = append(r.Steps, "stead client init --alias "+alias+" --discover tailscale --yes")
			return r, nil
		}
		return r, cfgErr
	}
	r.Items = append(r.Items, item{"Stead config", "ok", cfgPath})

	host := cfg.Hosts[alias]
	if host == nil {
		r.Items = append(r.Items, item{"Configured host", "missing", "alias not found"})
		r.Steps = append(r.Steps, "stead client init --alias "+alias+" --discover tailscale --yes")
		return r, nil
	}
	r.Items = append(r.Items, item{"Configured host", hostState(host), hostDetail(host)})
	locality := targetLocality(host.Hostname)

	keyState, keyDetail := keyStatus(host.IdentityFile)
	r.Items = append(r.Items, item{"Client key", keyState, keyDetail})
	if keyState != "ok" {
		r.Steps = append(r.Steps, clientInitStep(alias, host.Hostname))
	}

	sshConfigPath, err := sshConfigPath(opts.SSHConfigPath)
	if err != nil {
		return r, err
	}
	aliasState, aliasDetail := sshAliasStatus(sshConfigPath, alias)
	r.Items = append(r.Items, item{"SSH alias", aliasState, aliasDetail})
	if aliasState != "ok" {
		r.Steps = append(r.Steps, "stead client apply --dry-run --alias "+alias)
		r.Steps = append(r.Steps, "stead client apply --alias "+alias)
	}

	wakeState, wakeDetail := wakeStatus(host.Wake)
	r.Items = append(r.Items, item{"Wake config", wakeState, wakeDetail})
	if wakeState != "ok" {
		r.Steps = append(r.Steps, "stead client wake-config --alias "+alias+" --mac-address <host-lan-mac> --broadcast <lan-broadcast>")
	}

	if locality == "remote" {
		r.Items = append(r.Items, item{"Host hardening", "unknown", "remote host; run doctor on host"})
		r.Items = append(r.Items, item{"tmux auto-attach", "unknown", "remote host; run doctor on host"})
	} else {
		hardenState, hardenDetail := fileStatus(hostharden.DefaultDropInPath)
		r.Items = append(r.Items, item{"Host hardening", hardenState, hardenDetail})
		if hardenState != "ok" && host.User != "" {
			r.Steps = append(r.Steps, "stead host harden --dry-run --user "+shellQuote(host.User)+" --disable-password")
		}

		tmuxState, tmuxDetail := tmuxAutoAttachStatus(defaultZshrcPath())
		r.Items = append(r.Items, item{"tmux auto-attach", tmuxState, tmuxDetail})
		if tmuxState == "missing" && host.Session.Tmux {
			session := valueOrDefault(host.Session.TmuxSession, "main")
			r.Steps = append(r.Steps, "stead host install --dry-run --tmux-session "+shellQuote(session))
		}
	}

	if opts.Verify {
		loginState, loginDetail := verifyLogin(alias, opts.Runner)
		r.Items = append(r.Items, item{"SSH login", loginState, loginDetail})
		if loginState != "ok" && keyState == "ok" {
			r.Steps = append(r.Steps, "stead host authorize --alias "+alias+" --public-key '<client-public-key>' --dry-run")
			r.Steps = append(r.Steps, "stead verify --alias "+alias)
		}
	} else if aliasState == "ok" {
		r.Items = append(r.Items, item{"SSH login", "unknown", "not checked; pass --verify"})
		r.Steps = append(r.Steps, "stead doctor --alias "+alias+" --verify")
	}

	if ready(r.Items) {
		r.Steps = append(r.Steps, "stead connect --alias "+alias)
	}
	r.Steps = unique(r.Steps)
	return r, nil
}

func print(out io.Writer, r report) {
	ui.PrintTitle(out, "Stead doctor")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Alias", r.Alias)
	ui.PrintKV(out, "Mode", "read-only diagnostic")
	fmt.Fprintln(out)

	ui.PrintSection(out, "Diagnosis")
	for _, item := range r.Items {
		value := ui.State(out, item.State)
		if item.Detail != "" {
			value = ui.StateDetail(out, item.State, item.Detail)
		}
		ui.PrintKV(out, item.Label, value)
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Next steps")
	if len(r.Steps) == 0 {
		ui.PrintKV(out, "Status", "no action suggested")
		return
	}
	for i, step := range r.Steps {
		ui.PrintStep(out, i+1, step)
	}
}

func loadConfig(path string) (*config.Config, string, error) {
	if path == "" {
		return config.LoadDefault()
	}
	cfg, err := config.Load(path)
	return cfg, path, err
}

func sshConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

func hostState(host *config.Host) string {
	if strings.TrimSpace(host.Hostname) == "" || isPlaceholder(host.Hostname) ||
		strings.TrimSpace(host.User) == "" ||
		strings.TrimSpace(host.IdentityFile) == "" {
		return "incomplete"
	}
	return "ok"
}

func hostDetail(host *config.Host) string {
	findings := make([]string, 0)
	if strings.TrimSpace(host.Hostname) == "" || isPlaceholder(host.Hostname) {
		findings = append(findings, "hostname")
	}
	if strings.TrimSpace(host.User) == "" {
		findings = append(findings, "user")
	}
	if strings.TrimSpace(host.IdentityFile) == "" {
		findings = append(findings, "identity")
	}
	if len(findings) == 0 {
		return host.Hostname
	}
	return "missing " + strings.Join(findings, ", ")
}

func keyStatus(identityFile string) (string, string) {
	if strings.TrimSpace(identityFile) == "" {
		return "missing", "identity_file unset"
	}
	path, err := expandHome(identityFile)
	if err != nil {
		return "unknown", err.Error()
	}
	privateOK := exists(path)
	publicOK := exists(path + ".pub")
	switch {
	case privateOK && publicOK:
		return "ok", identityFile
	case privateOK:
		return "incomplete", "public key missing"
	case publicOK:
		return "incomplete", "private key missing"
	default:
		return "missing", identityFile
	}
}

func sshAliasStatus(path, alias string) (string, string) {
	cfg, err := sshconfig.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "missing", path
		}
		return "unknown", err.Error()
	}
	status := sshconfig.CheckAlias(cfg, alias)
	if status.State == "ok" {
		return "ok", status.Host.HostName
	}
	return status.State, strings.Join(status.Findings, ", ")
}

func wakeStatus(w config.Wake) (string, string) {
	hasMAC := strings.TrimSpace(w.MACAddress) != "" && !isPlaceholder(w.MACAddress)
	hasBroadcast := strings.TrimSpace(w.Broadcast) != "" && !isPlaceholder(w.Broadcast)
	switch {
	case hasMAC && hasBroadcast:
		return "ok", "ready"
	case !hasMAC && !hasBroadcast:
		return "missing", "optional; WOL not ready"
	case !hasMAC:
		return "incomplete", "MAC missing"
	default:
		return "incomplete", "broadcast missing"
	}
}

func fileStatus(path string) (string, string) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "missing", path
		}
		return "unknown", err.Error()
	}
	if info.IsDir() {
		return "unknown", "is a directory"
	}
	return "ok", path
}

func tmuxAutoAttachStatus(path string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "missing", "~/.zshrc not found"
		}
		return "unknown", err.Error()
	}
	text := string(data)
	if strings.Contains(text, "stead managed block: tmux auto-attach") {
		return "ok", "stead managed"
	}
	if strings.Contains(text, "SSH_CONNECTION") && strings.Contains(text, "tmux new-session") {
		return "ok", "custom present"
	}
	return "missing", "not configured"
}

func verifyLogin(alias string, runner verify.Runner) (string, string) {
	if runner == nil {
		runner = verify.SSHRunner
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := runner(ctx, alias)
	if err == nil {
		return "ok", "BatchMode login succeeded"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "failed", "timed out after 10s"
	}
	return "failed", shortError(err)
}

func ready(items []item) bool {
	for _, item := range items {
		switch item.Label {
		case "Configured host", "Client key", "SSH alias":
			if item.State != "ok" {
				return false
			}
		case "Host hardening", "tmux auto-attach":
			if item.State != "ok" && item.State != "unknown" {
				return false
			}
		case "SSH login":
			if item.State != "ok" && item.State != "unknown" {
				return false
			}
		}
	}
	return true
}

func targetLocality(hostname string) string {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" || isPlaceholder(hostname) {
		return "unknown"
	}
	ip := net.ParseIP(hostname)
	if ip == nil {
		return "unknown"
	}
	if ip.IsLoopback() {
		return "local"
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		var localIP net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			localIP = v.IP
		case *net.IPAddr:
			localIP = v.IP
		}
		if localIP != nil && localIP.Equal(ip) {
			return "local"
		}
	}
	return "remote"
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

func defaultZshrcPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.zshrc"
	}
	return filepath.Join(home, ".zshrc")
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isPlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")
}

func clientInitStep(alias, hostname string) string {
	if hostname == "" || isPlaceholder(hostname) {
		return "stead client init --alias " + alias + " --discover tailscale --yes"
	}
	return "stead client init --alias " + alias + " --hostname " + shellQuote(hostname) + " --yes"
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func valueOrDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
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

func shortError(err error) string {
	if exitErr, ok := err.(*exec.ExitError); ok {
		msg := strings.TrimSpace(string(exitErr.Stderr))
		if msg != "" {
			if len(msg) > 120 {
				msg = msg[:120] + "..."
			}
			return msg
		}
	}
	return err.Error()
}
