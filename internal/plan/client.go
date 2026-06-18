package plan

import (
	"fmt"
	"io"
	"strings"

	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/ui"
)

// WriteClient prints a read-only client setup plan for one configured host.
func WriteClient(out io.Writer, cfg *config.Config, path, alias string) error {
	if alias == "" {
		alias = cfg.Defaults.Alias
	}
	if alias == "" {
		return fmt.Errorf("no alias provided and defaults.alias is unset")
	}

	host := cfg.Hosts[alias]
	if host == nil {
		return fmt.Errorf("alias %q not found in %s", alias, path)
	}

	ui.PrintTitle(out, "Stead client plan")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Config", path)
	ui.PrintKV(out, "Alias", alias)
	ui.PrintKV(out, "Mode", "read-only plan")
	fmt.Fprintln(out)

	ui.PrintSection(out, "Proposed ~/.ssh/config entry")
	fmt.Fprintf(out, "  Host %s\n", alias)
	fmt.Fprintf(out, "      HostName %s\n", configValue(host.Hostname))
	fmt.Fprintf(out, "      User %s\n", configValue(host.User))
	fmt.Fprintf(out, "      Port %d\n", defaultPort(host.Port))
	if host.IdentityFile != "" {
		fmt.Fprintf(out, "      IdentityFile %s\n", host.IdentityFile)
	}
	fmt.Fprintln(out, "      AddKeysToAgent yes")
	fmt.Fprintln(out, "      IdentitiesOnly yes")
	fmt.Fprintln(out)

	ui.PrintSection(out, "Connection behavior")
	ui.PrintKV(out, "SSH command", "system ssh")
	ui.PrintKV(out, "SSH authentication", "OpenSSH/macOS sshd, using normal SSH keys or server policy")
	ui.PrintKV(out, "Tailscale SSH", "not used")
	if host.PreferredNetwork != "" {
		ui.PrintKV(out, "Preferred network", host.PreferredNetwork)
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Wake flow")
	if host.Wake.MACAddress == "" && host.Wake.Broadcast == "" {
		ui.PrintKV(out, "Status", "not configured")
	} else {
		ui.PrintKV(out, "MAC address", displayValue(host.Wake.MACAddress))
		ui.PrintKV(out, "Broadcast", displayValue(host.Wake.Broadcast))
		ui.PrintKV(out, "Timeout", valueOrDefault(host.Wake.Timeout, "90s"))
		ui.PrintKV(out, "Interval", valueOrDefault(host.Wake.Interval, "2s"))
		ui.PrintKV(out, "Behavior", "send Wake-on-LAN, wait for SSH port, then exec system ssh")
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Session")
	if host.Session.Tmux {
		tmux := "enabled"
		if host.Session.TmuxSession != "" {
			tmux += " (" + host.Session.TmuxSession + ")"
		}
		ui.PrintKV(out, "tmux", tmux)
	} else {
		ui.PrintKV(out, "tmux", "disabled")
	}
	if host.Session.ProjectDir != "" {
		ui.PrintKV(out, "Project directory", host.Session.ProjectDir)
	}
	fmt.Fprintln(out)

	findings := findings(host)
	ui.PrintSection(out, "Readiness")
	if len(findings) == 0 {
		ui.PrintKV(out, "Status", ui.State(out, "ok"))
	} else {
		for _, finding := range findings {
			ui.PrintKV(out, "Incomplete", finding)
		}
	}
	notes := notes(host)
	if len(notes) > 0 {
		fmt.Fprintln(out)
		ui.PrintSection(out, "Notes")
		for _, note := range notes {
			ui.PrintListItem(out, note)
		}
	}

	return nil
}

func findings(host *config.Host) []string {
	out := make([]string, 0)
	if host.Hostname == "" {
		out = append(out, "hostname missing")
	} else if isPlaceholder(host.Hostname) {
		out = append(out, "hostname placeholder")
	}
	if host.User == "" {
		out = append(out, "user missing")
	}
	if host.IdentityFile == "" {
		out = append(out, "identity_file missing")
	}
	return out
}

func notes(host *config.Host) []string {
	out := make([]string, 0)
	if isPlaceholder(host.Wake.MACAddress) {
		out = append(out, "wake MAC placeholder; wake flow is not ready")
	}
	if isPlaceholder(host.Wake.Broadcast) {
		out = append(out, "wake broadcast placeholder; wake flow is not ready")
	}
	return out
}

func displayValue(value string) string {
	if value == "" {
		return "(unset)"
	}
	if isPlaceholder(value) {
		return "(placeholder: " + value + ")"
	}
	return value
}

func configValue(value string) string {
	if value == "" {
		return "(unset)"
	}
	return value
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultPort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}

func isPlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")
}
