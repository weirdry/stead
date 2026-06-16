package plan

import (
	"fmt"
	"io"
	"strings"

	"github.com/ed/stead/internal/config"
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

	fmt.Fprintln(out, "Stead client plan")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Config: %s\n", path)
	fmt.Fprintf(out, "Alias: %s\n", alias)
	fmt.Fprintln(out, "Mode: read-only plan")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Proposed ~/.ssh/config entry")
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

	fmt.Fprintln(out, "Connection behavior")
	fmt.Fprintln(out, "  SSH command: system ssh")
	fmt.Fprintln(out, "  SSH authentication: OpenSSH/macOS sshd, using normal SSH keys or server policy")
	fmt.Fprintln(out, "  Tailscale SSH: not used")
	if host.PreferredNetwork != "" {
		fmt.Fprintf(out, "  Preferred network: %s\n", host.PreferredNetwork)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Wake flow")
	if host.Wake.MACAddress == "" && host.Wake.Broadcast == "" {
		fmt.Fprintln(out, "  not configured")
	} else {
		fmt.Fprintf(out, "  MAC address: %s\n", displayValue(host.Wake.MACAddress))
		fmt.Fprintf(out, "  Broadcast: %s\n", displayValue(host.Wake.Broadcast))
		fmt.Fprintf(out, "  Timeout: %s\n", valueOrDefault(host.Wake.Timeout, "90s"))
		fmt.Fprintf(out, "  Interval: %s\n", valueOrDefault(host.Wake.Interval, "2s"))
		fmt.Fprintln(out, "  Behavior: send Wake-on-LAN, wait for SSH port, then exec system ssh")
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Session")
	if host.Session.Tmux {
		fmt.Fprintf(out, "  tmux: enabled")
		if host.Session.TmuxSession != "" {
			fmt.Fprintf(out, " (%s)", host.Session.TmuxSession)
		}
		fmt.Fprintln(out)
	} else {
		fmt.Fprintln(out, "  tmux: disabled")
	}
	if host.Session.ProjectDir != "" {
		fmt.Fprintf(out, "  Project directory: %s\n", host.Session.ProjectDir)
	}
	fmt.Fprintln(out)

	findings := findings(host)
	fmt.Fprintln(out, "Readiness")
	if len(findings) == 0 {
		fmt.Fprintln(out, "  ok")
	} else {
		for _, finding := range findings {
			fmt.Fprintf(out, "  incomplete: %s\n", finding)
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
	if isPlaceholder(host.Wake.MACAddress) {
		out = append(out, "wake MAC placeholder")
	}
	if isPlaceholder(host.Wake.Broadcast) {
		out = append(out, "wake broadcast placeholder")
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
