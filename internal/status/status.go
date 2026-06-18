package status

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/sshconfig"
	"github.com/ed/stead/internal/ui"
)

// Run prints a read-only snapshot of the local machine's Stead-relevant state.
func Run(out io.Writer) error {
	s := collect()
	printCombined(out, s)
	return nil
}

// RunHost prints read-only host-side status.
func RunHost(out io.Writer) error {
	s := collect()
	printHost(out, s)
	return nil
}

// RunClient prints read-only client-side status.
func RunClient(out io.Writer) error {
	s := collect()
	printClient(out, s)
	return nil
}

type snapshot struct {
	OS               string
	Arch             string
	User             string
	Home             string
	SSHPath          check
	SSHDPath         check
	TailscalePath    check
	TailscaleApp     check
	TailscaleIP      check
	HostSSHConfig    check
	SSHDConfigD      check
	LaunchdSSHD      check
	RemoteLogin      check
	SSHDActiveLines  check
	UserSSHDir       check
	UserSSHConfig    check
	AuthorizedKeys   check
	Tmux             check
	TmuxAutoAttach   check
	Hardening        []finding
	Config           check
	ConfigAlias      string
	ConfigHosts      []config.HostStatus
	ClientAliases    []sshconfig.AliasStatus
	ClientConfigNote string
	ConfigPath       string
	DefaultHostLike  bool
}

type check struct {
	State  string
	Detail string
}

type finding struct {
	Label  string
	State  string
	Detail string
}

func collect() snapshot {
	u, _ := user.Current()
	home := ""
	username := "unknown"
	if u != nil {
		home = u.HomeDir
		username = u.Username
	}

	s := snapshot{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		User:       username,
		Home:       home,
		ConfigPath: config.DefaultPath(),
	}

	s.SSHPath = lookPath("ssh")
	s.SSHDPath = lookPath("sshd")
	s.TailscalePath = lookPath("tailscale")
	s.TailscaleApp = fileExists("/Applications/Tailscale.app")
	s.TailscaleIP = tailscaleIP(s.TailscalePath)
	s.HostSSHConfig = fileExists("/etc/ssh/sshd_config")
	s.SSHDConfigD = configDropIns("/etc/ssh/sshd_config.d")
	s.LaunchdSSHD = launchdSSHD()
	s.RemoteLogin = remoteLogin()
	s.SSHDActiveLines = sshdActiveLines()
	s.UserSSHDir = fileExists(filepath.Join(home, ".ssh"))
	s.UserSSHConfig = fileExists(filepath.Join(home, ".ssh", "config"))
	s.AuthorizedKeys = authorizedKeys(filepath.Join(home, ".ssh", "authorized_keys"))
	s.Tmux = lookPath("tmux")
	s.TmuxAutoAttach = tmuxAutoAttach(filepath.Join(home, ".zshrc"))
	s.DefaultHostLike = s.SSHDPath.State == "ok" && s.HostSSHConfig.State == "ok"
	s.Hardening = hostHardening(s)
	s.Config, s.ConfigAlias, s.ConfigHosts = steadConfig()
	s.ClientAliases, s.ClientConfigNote = clientAliases(s.UserSSHConfig, filepath.Join(home, ".ssh", "config"), s.ConfigHosts)

	return s
}

func printCombined(out io.Writer, s snapshot) {
	fmt.Fprintln(out, "Stead status")
	fmt.Fprintln(out)

	printSystem(out, s)
	printHostSections(out, s)
	printClientSections(out, s)
	printStead(out, s)
}

func printHost(out io.Writer, s snapshot) {
	fmt.Fprintln(out, "Stead host status")
	fmt.Fprintln(out)

	printSystem(out, s)
	printHostSections(out, s)
	printStead(out, s)
}

func printClient(out io.Writer, s snapshot) {
	fmt.Fprintln(out, "Stead client status")
	fmt.Fprintln(out)

	printSystem(out, s)
	printClientSections(out, s)
	printStead(out, s)
}

func printSystem(out io.Writer, s snapshot) {
	fmt.Fprintf(out, "System\n")
	fmt.Fprintf(out, "  OS: %s\n", value(s.OS))
	fmt.Fprintf(out, "  Arch: %s\n", value(s.Arch))
	fmt.Fprintf(out, "  User: %s\n", value(s.User))
	fmt.Fprintf(out, "  Home: %s\n", value(s.Home))
	fmt.Fprintln(out)
}

func printHostSections(out io.Writer, s snapshot) {
	fmt.Fprintf(out, "OpenSSH\n")
	printCheck(out, "sshd server", s.SSHDPath)
	printCheck(out, "sshd_config", s.HostSSHConfig)
	printCheck(out, "sshd_config.d", s.SSHDConfigD)
	printCheck(out, "launchd sshd", s.LaunchdSSHD)
	printCheck(out, "Remote Login", s.RemoteLogin)
	printCheck(out, "active config lines", s.SSHDActiveLines)
	fmt.Fprintf(out, "  Host-capable: %s\n", yesNo(s.DefaultHostLike))
	fmt.Fprintln(out)

	fmt.Fprintf(out, "User SSH\n")
	printCheck(out, "~/.ssh", s.UserSSHDir)
	printCheck(out, "~/.ssh/config", s.UserSSHConfig)
	printCheck(out, "~/.ssh/authorized_keys", s.AuthorizedKeys)
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Session\n")
	printCheck(out, "tmux", s.Tmux)
	printCheck(out, "tmux auto-attach", s.TmuxAutoAttach)
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Host hardening\n")
	for _, finding := range s.Hardening {
		printFinding(out, finding)
	}
	fmt.Fprintln(out)
}

func printClientSections(out io.Writer, s snapshot) {
	fmt.Fprintf(out, "OpenSSH client\n")
	printCheck(out, "ssh client", s.SSHPath)
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Tailscale network metadata\n")
	printCheck(out, "tailscale CLI", s.TailscalePath)
	printCheck(out, "Tailscale.app", s.TailscaleApp)
	printCheck(out, "tailscale IP", s.TailscaleIP)
	fmt.Fprintf(out, "  Tailscale SSH: unmanaged by stead\n")
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Client SSH config\n")
	printCheck(out, "~/.ssh/config", s.UserSSHConfig)
	if s.ClientConfigNote != "" {
		fmt.Fprintf(out, "  Note: %s\n", s.ClientConfigNote)
	}
	for _, alias := range s.ClientAliases {
		detail := alias.State
		if len(alias.Findings) > 0 {
			detail += " (" + strings.Join(alias.Findings, ", ") + ")"
		}
		fmt.Fprintf(out, "  Alias %s: %s\n", alias.Alias, colorStatePrefix(out, detail, alias.State))
		if alias.State != "missing" {
			if alias.Host.HostName != "" {
				fmt.Fprintf(out, "    HostName: %s\n", alias.Host.HostName)
			}
			if alias.Host.User != "" {
				fmt.Fprintf(out, "    User: %s\n", alias.Host.User)
			}
			if alias.Host.Port != "" {
				fmt.Fprintf(out, "    Port: %s\n", alias.Host.Port)
			}
			if alias.Host.IdentityFile != "" {
				fmt.Fprintf(out, "    IdentityFile: %s\n", alias.Host.IdentityFile)
			}
		}
	}
	fmt.Fprintln(out)
}

func printStead(out io.Writer, s snapshot) {
	fmt.Fprintf(out, "Stead\n")
	fmt.Fprintf(out, "  Config path: %s\n", s.ConfigPath)
	printCheck(out, "Config", s.Config)
	if s.Config.State == "ok" {
		fmt.Fprintf(out, "  Default alias: %s\n", value(s.ConfigAlias))
		if len(s.ConfigHosts) == 0 {
			fmt.Fprintf(out, "  Configured hosts: none\n")
		} else {
			fmt.Fprintf(out, "  Configured hosts:\n")
			for _, host := range s.ConfigHosts {
				detail := host.State
				if len(host.Findings) > 0 {
					detail += " (" + strings.Join(host.Findings, ", ") + ")"
				}
				fmt.Fprintf(out, "    %s: %s\n", host.Alias, colorStatePrefix(out, detail, host.State))
			}
		}
	}
	fmt.Fprintf(out, "  Mode: read-only status\n")
}

func printCheck(out io.Writer, label string, c check) {
	if c.Detail == "" {
		fmt.Fprintf(out, "  %s: %s\n", label, ui.State(out, c.State))
		return
	}
	fmt.Fprintf(out, "  %s: %s (%s)\n", label, ui.State(out, c.State), c.Detail)
}

func printFinding(out io.Writer, f finding) {
	if f.Detail == "" {
		fmt.Fprintf(out, "  %s: %s\n", f.Label, ui.State(out, f.State))
		return
	}
	fmt.Fprintf(out, "  %s: %s (%s)\n", f.Label, ui.State(out, f.State), f.Detail)
}

func colorStatePrefix(out io.Writer, detail, state string) string {
	colored := ui.State(out, state)
	if colored == state {
		return detail
	}
	return colored + strings.TrimPrefix(detail, state)
}

func lookPath(name string) check {
	path, err := exec.LookPath(name)
	if err != nil {
		return check{State: "missing"}
	}
	return check{State: "ok", Detail: path}
}

func fileExists(path string) check {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return check{State: "missing"}
		}
		return check{State: "unknown", Detail: err.Error()}
	}
	if info.IsDir() {
		return check{State: "ok", Detail: "directory"}
	}
	return check{State: "ok", Detail: fmt.Sprintf("%d bytes", info.Size())}
}

func configDropIns(path string) check {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return check{State: "missing"}
		}
		return check{State: "unknown", Detail: err.Error()}
	}

	count := 0
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		count++
		names = append(names, entry.Name())
	}
	if count == 0 {
		return check{State: "ok", Detail: "0 files"}
	}
	return check{State: "ok", Detail: fmt.Sprintf("%d file(s): %s", count, strings.Join(names, ", "))}
}

func launchdSSHD() check {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "launchctl", "print", "system/com.openssh.sshd")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return check{State: "unknown", Detail: shortCommandError(err, out)}
	}

	text := string(out)
	state := findLaunchdValue(text, "state")
	service := findLaunchdValue(text, "service name")
	socket := "socket unknown"
	if strings.Contains(text, "passive = 1") {
		socket = "passive socket"
	}
	if state == "" {
		state = "state unknown"
	} else {
		state = "state " + state
	}
	if service != "" {
		return check{State: "ok", Detail: fmt.Sprintf("%s, %s, service %s", state, socket, service)}
	}
	return check{State: "ok", Detail: fmt.Sprintf("%s, %s", state, socket)}
}

func remoteLogin() check {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "launchctl", "print-disabled", "system")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return check{State: "unknown", Detail: shortCommandError(err, out)}
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, `"com.openssh.sshd"`) {
			continue
		}
		switch {
		case strings.Contains(line, "=> enabled"):
			return check{State: "ok", Detail: "enabled via launchd"}
		case strings.Contains(line, "=> disabled"):
			return check{State: "disabled", Detail: "disabled via launchd"}
		default:
			return check{State: "unknown", Detail: line}
		}
	}

	return check{State: "unknown", Detail: "com.openssh.sshd not listed"}
}

func findLaunchdValue(text, key string) string {
	prefix := key + " = "
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func sshdActiveLines() check {
	matches := make([]string, 0)
	for _, path := range sshdConfigPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) == 0 {
				continue
			}
			if trackedSSHDKeys()[strings.ToLower(fields[0])] {
				matches = append(matches, filepath.Base(path)+": "+formatConfigLine(fields))
			}
		}
	}
	if len(matches) == 0 {
		return check{State: "ok", Detail: "none"}
	}
	return check{State: "ok", Detail: strings.Join(matches, "; ")}
}

func formatConfigLine(fields []string) string {
	return redactConfigLine(strings.Join(fields, " "))
}

func hostHardening(s snapshot) []finding {
	directives := sshdDirectives()
	findings := make([]finding, 0, 8)

	findings = append(findings, assessBooleanDirective(
		"Password auth",
		directives,
		"passwordauthentication",
		finding{State: "ok", Detail: "explicitly disabled"},
		finding{State: "risk", Detail: "explicitly enabled"},
		finding{State: "warn", Detail: "not explicitly disabled; likely enabled by OpenSSH/macOS defaults"},
	))

	findings = append(findings, assessBooleanDirective(
		"Keyboard-interactive auth",
		directives,
		"kbdinteractiveauthentication",
		finding{State: "ok", Detail: "explicitly disabled"},
		finding{State: "risk", Detail: "explicitly enabled"},
		finding{State: "warn", Detail: "not explicitly disabled; password-style prompts may be available"},
	))

	findings = append(findings, assessBooleanDirective(
		"Pubkey auth",
		directives,
		"pubkeyauthentication",
		finding{State: "risk", Detail: "explicitly disabled"},
		finding{State: "ok", Detail: "explicitly enabled"},
		finding{State: "ok", Detail: "not explicitly configured; likely enabled by OpenSSH defaults"},
	))

	findings = append(findings, assessRootLogin(directives))
	findings = append(findings, assessUserRestriction(directives))
	findings = append(findings, assessKeyMaterial(s.AuthorizedKeys))
	findings = append(findings, assessSteadHostConfig())
	findings = append(findings, finding{
		Label:  "Effective sshd config",
		State:  "unknown",
		Detail: "not evaluated yet; future host status should use sshd -T when permissions allow",
	})

	return findings
}

func assessBooleanDirective(label string, directives map[string][]string, key string, onNo finding, onYes finding, missing finding) finding {
	value, ok := lastDirectiveValue(directives, key)
	if !ok {
		missing.Label = label
		return missing
	}

	switch strings.ToLower(value) {
	case "no":
		onNo.Label = label
		return onNo
	case "yes":
		onYes.Label = label
		return onYes
	default:
		return finding{Label: label, State: "unknown", Detail: "configured as " + value}
	}
}

func assessRootLogin(directives map[string][]string) finding {
	value, ok := lastDirectiveValue(directives, "permitrootlogin")
	if !ok {
		return finding{
			Label:  "Root login",
			State:  "unknown",
			Detail: "not explicitly configured",
		}
	}

	switch strings.ToLower(value) {
	case "no":
		return finding{Label: "Root login", State: "ok", Detail: "explicitly disabled"}
	case "prohibit-password", "without-password", "forced-commands-only":
		return finding{Label: "Root login", State: "warn", Detail: "restricted but not fully disabled: " + value}
	case "yes":
		return finding{Label: "Root login", State: "risk", Detail: "explicitly enabled"}
	default:
		return finding{Label: "Root login", State: "unknown", Detail: "configured as " + value}
	}
}

func assessUserRestriction(directives map[string][]string) finding {
	if values, ok := directives["allowusers"]; ok && len(values) > 0 {
		return finding{Label: "User restriction", State: "ok", Detail: "AllowUsers configured"}
	}
	if values, ok := directives["allowgroups"]; ok && len(values) > 0 {
		return finding{Label: "User restriction", State: "ok", Detail: "AllowGroups configured"}
	}
	return finding{
		Label:  "User restriction",
		State:  "warn",
		Detail: "no AllowUsers or AllowGroups directive found",
	}
}

func assessKeyMaterial(c check) finding {
	if c.State != "ok" {
		return finding{Label: "Authorized keys", State: "warn", Detail: c.State}
	}
	if strings.HasPrefix(c.Detail, "0 ") {
		return finding{Label: "Authorized keys", State: "warn", Detail: "no keys present"}
	}
	return finding{Label: "Authorized keys", State: "ok", Detail: c.Detail}
}

func assessSteadHostConfig() finding {
	c := fileExists("/etc/ssh/sshd_config.d/stead.conf")
	if c.State == "ok" {
		return finding{Label: "Stead host config", State: "ok", Detail: "/etc/ssh/sshd_config.d/stead.conf present"}
	}
	if c.State == "missing" {
		return finding{Label: "Stead host config", State: "missing", Detail: "not installed"}
	}
	return finding{Label: "Stead host config", State: "unknown", Detail: c.Detail}
}

func sshdDirectives() map[string][]string {
	directives := make(map[string][]string)
	for _, path := range sshdConfigPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) < 2 {
				continue
			}
			key := strings.ToLower(fields[0])
			directives[key] = append(directives[key], strings.Join(fields[1:], " "))
		}
	}
	return directives
}

func lastDirectiveValue(directives map[string][]string, key string) (string, bool) {
	values, ok := directives[key]
	if !ok || len(values) == 0 {
		return "", false
	}
	return values[len(values)-1], true
}

func sshdConfigPaths() []string {
	paths := []string{"/etc/ssh/sshd_config"}
	entries, err := os.ReadDir("/etc/ssh/sshd_config.d")
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				paths = append(paths, filepath.Join("/etc/ssh/sshd_config.d", entry.Name()))
			}
		}
	}
	return paths
}

func trackedSSHDKeys() map[string]bool {
	return map[string]bool{
		"allowgroups":                     true,
		"allowtcpforwarding":              true,
		"allowusers":                      true,
		"authenticationmethods":           true,
		"authorizedkeyscommand":           true,
		"authorizedkeyscommanduser":       true,
		"authorizedkeysfile":              true,
		"challengeresponseauthentication": true,
		"chrootdirectory":                 true,
		"denygroups":                      true,
		"denyusers":                       true,
		"forcecommand":                    true,
		"gatewayports":                    true,
		"hostkeyalgorithms":               true,
		"include":                         true,
		"kbdinteractiveauthentication":    true,
		"listenaddress":                   true,
		"match":                           true,
		"passwordauthentication":          true,
		"permitrootlogin":                 true,
		"permittunnel":                    true,
		"pubkeyacceptedalgorithms":        true,
		"pubkeyauthentication":            true,
		"subsystem":                       true,
		"usepam":                          true,
		"x11forwarding":                   true,
	}
}

func redactConfigLine(line string) string {
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "authorizedkeyscommand ") ||
		strings.HasPrefix(lower, "forcecommand ") ||
		strings.HasPrefix(lower, "match ") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return line
		}
		return fields[0] + " [redacted]"
	}
	return line
}

func tmuxAutoAttach(path string) check {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return check{State: "missing", Detail: "~/.zshrc not found"}
		}
		return check{State: "unknown", Detail: err.Error()}
	}
	text := string(data)
	if strings.Contains(text, "stead managed block: tmux auto-attach") {
		return check{State: "ok", Detail: "stead managed block present"}
	}
	if strings.Contains(text, "SSH_CONNECTION") && strings.Contains(text, "tmux new-session") {
		return check{State: "ok", Detail: "custom SSH tmux auto-attach present"}
	}
	return check{State: "missing"}
}

func shortCommandError(err error, out []byte) string {
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return err.Error()
	}
	if len(msg) > 160 {
		msg = msg[:160] + "..."
	}
	return msg
}

func steadConfig() (check, string, []config.HostStatus) {
	cfg, path, err := config.LoadDefault()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return check{State: "missing", Detail: path}, "", nil
		}
		return check{State: "unknown", Detail: err.Error()}, "", nil
	}
	return check{State: "ok", Detail: path}, cfg.Defaults.Alias, config.HostStatuses(cfg)
}

func clientAliases(userConfig check, path string, hosts []config.HostStatus) ([]sshconfig.AliasStatus, string) {
	if userConfig.State != "ok" || len(hosts) == 0 {
		return nil, ""
	}

	cfg, err := sshconfig.Load(path)
	if err != nil {
		return nil, "unable to parse ~/.ssh/config: " + err.Error()
	}

	statuses := make([]sshconfig.AliasStatus, 0, len(hosts))
	for _, host := range hosts {
		statuses = append(statuses, sshconfig.CheckAlias(cfg, host.Alias))
	}

	note := ""
	if cfg.HasInclude {
		note = "Include directive present; included files are not expanded yet"
	}
	return statuses, note
}

func authorizedKeys(path string) check {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return check{State: "missing"}
		}
		return check{State: "unknown", Detail: err.Error()}
	}

	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		count++
	}

	return check{State: "ok", Detail: fmt.Sprintf("%d key(s)", count)}
}

func tailscaleIP(cli check) check {
	if cli.State != "ok" {
		return interfaceTailscaleIP()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli.Detail, "ip", "-4")
	out, err := cmd.Output()
	if err == nil {
		ip := strings.TrimSpace(string(out))
		if ip != "" {
			return check{State: "ok", Detail: ip}
		}
	}

	return interfaceTailscaleIP()
}

func interfaceTailscaleIP() check {
	ifaces, err := net.Interfaces()
	if err != nil {
		return check{State: "unknown", Detail: err.Error()}
	}

	for _, iface := range ifaces {
		if !strings.HasPrefix(iface.Name, "utun") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil || ip == nil || ip.To4() == nil {
				continue
			}
			if strings.HasPrefix(ip.String(), "100.") {
				return check{State: "ok", Detail: ip.String() + " via " + iface.Name}
			}
		}
	}

	return check{State: "unknown", Detail: "not detected"}
}

func configPath(home string) string {
	if home == "" {
		return "~/.config/stead/config.toml"
	}
	return filepath.Join(home, ".config", "stead", "config.toml")
}

func value(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
