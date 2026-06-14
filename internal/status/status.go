package status

import (
	"context"
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
	OS              string
	Arch            string
	User            string
	Home            string
	SSHPath         check
	SSHDPath        check
	TailscalePath   check
	TailscaleApp    check
	TailscaleIP     check
	HostSSHConfig   check
	SSHDConfigD     check
	LaunchdSSHD     check
	RemoteLogin     check
	SSHDActiveLines check
	UserSSHDir      check
	UserSSHConfig   check
	AuthorizedKeys  check
	Tmux            check
	TmuxAutoAttach  check
	ConfigPath      string
	DefaultHostLike bool
}

type check struct {
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
		ConfigPath: configPath(home),
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
	fmt.Fprintln(out)
}

func printStead(out io.Writer, s snapshot) {
	fmt.Fprintf(out, "Stead\n")
	fmt.Fprintf(out, "  Config path: %s\n", s.ConfigPath)
	fmt.Fprintf(out, "  Mode: read-only status\n")
}

func printCheck(out io.Writer, label string, c check) {
	if c.Detail == "" {
		fmt.Fprintf(out, "  %s: %s\n", label, c.State)
		return
	}
	fmt.Fprintf(out, "  %s: %s (%s)\n", label, c.State, c.Detail)
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
	paths := []string{"/etc/ssh/sshd_config"}
	entries, err := os.ReadDir("/etc/ssh/sshd_config.d")
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				paths = append(paths, filepath.Join("/etc/ssh/sshd_config.d", entry.Name()))
			}
		}
	}

	keys := map[string]bool{
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

	matches := make([]string, 0)
	for _, path := range paths {
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
			if keys[strings.ToLower(fields[0])] {
				matches = append(matches, filepath.Base(path)+": "+redactConfigLine(trimmed))
			}
		}
	}
	if len(matches) == 0 {
		return check{State: "ok", Detail: "none"}
	}
	return check{State: "ok", Detail: strings.Join(matches, "; ")}
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
