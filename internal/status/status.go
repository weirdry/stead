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
	print(out, s)
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
	UserSSHDir      check
	UserSSHConfig   check
	AuthorizedKeys  check
	Tmux            check
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
	s.UserSSHDir = fileExists(filepath.Join(home, ".ssh"))
	s.UserSSHConfig = fileExists(filepath.Join(home, ".ssh", "config"))
	s.AuthorizedKeys = authorizedKeys(filepath.Join(home, ".ssh", "authorized_keys"))
	s.Tmux = lookPath("tmux")
	s.DefaultHostLike = s.SSHDPath.State == "ok" && s.HostSSHConfig.State == "ok"

	return s
}

func print(out io.Writer, s snapshot) {
	fmt.Fprintln(out, "Stead status")
	fmt.Fprintln(out)

	fmt.Fprintf(out, "System\n")
	fmt.Fprintf(out, "  OS: %s\n", value(s.OS))
	fmt.Fprintf(out, "  Arch: %s\n", value(s.Arch))
	fmt.Fprintf(out, "  User: %s\n", value(s.User))
	fmt.Fprintf(out, "  Home: %s\n", value(s.Home))
	fmt.Fprintln(out)

	fmt.Fprintf(out, "OpenSSH\n")
	printCheck(out, "ssh client", s.SSHPath)
	printCheck(out, "sshd server", s.SSHDPath)
	printCheck(out, "sshd_config", s.HostSSHConfig)
	fmt.Fprintf(out, "  Host-capable: %s\n", yesNo(s.DefaultHostLike))
	fmt.Fprintln(out)

	fmt.Fprintf(out, "User SSH\n")
	printCheck(out, "~/.ssh", s.UserSSHDir)
	printCheck(out, "~/.ssh/config", s.UserSSHConfig)
	printCheck(out, "~/.ssh/authorized_keys", s.AuthorizedKeys)
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Tailscale network metadata\n")
	printCheck(out, "tailscale CLI", s.TailscalePath)
	printCheck(out, "Tailscale.app", s.TailscaleApp)
	printCheck(out, "tailscale IP", s.TailscaleIP)
	fmt.Fprintf(out, "  Tailscale SSH: unmanaged by stead\n")
	fmt.Fprintln(out)

	fmt.Fprintf(out, "Session\n")
	printCheck(out, "tmux", s.Tmux)
	fmt.Fprintln(out)

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
