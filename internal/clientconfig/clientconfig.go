package clientconfig

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ed/stead/internal/config"
)

type Change struct {
	Alias string
	Path  string
	Block string
	State string
}

func WriteDryRun(out io.Writer, cfg *config.Config, cfgPath, sshConfigPath, alias string) error {
	if alias == "" {
		alias = cfg.Defaults.Alias
	}
	if alias == "" {
		return fmt.Errorf("no alias provided and defaults.alias is unset")
	}

	host := cfg.Hosts[alias]
	if host == nil {
		return fmt.Errorf("alias %q not found in %s", alias, cfgPath)
	}

	existing, err := os.ReadFile(sshConfigPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	change, err := Plan(existing, sshConfigPath, alias, host)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "Stead client apply")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Config: %s\n", cfgPath)
	fmt.Fprintf(out, "SSH config: %s\n", sshConfigPath)
	fmt.Fprintf(out, "Alias: %s\n", alias)
	fmt.Fprintln(out, "Mode: dry-run")
	if hasInclude(existing) {
		fmt.Fprintln(out, "Note: Include directive present; included files are not expanded")
	}
	fmt.Fprintln(out)

	switch change.State {
	case "add":
		fmt.Fprintln(out, "Action: would add managed SSH config block")
	case "replace":
		fmt.Fprintln(out, "Action: would replace existing managed SSH config block")
	case "unchanged":
		fmt.Fprintln(out, "Action: no changes needed")
	default:
		fmt.Fprintf(out, "Action: %s\n", change.State)
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Managed block")
	fmt.Fprint(out, indent(change.Block, "  "))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "No files were modified.")
	return nil
}

func Plan(existing []byte, path, alias string, host *config.Host) (Change, error) {
	block := ManagedBlock(alias, host)
	state, err := state(existing, alias, block)
	if err != nil {
		return Change{}, err
	}
	return Change{
		Alias: alias,
		Path:  path,
		Block: block,
		State: state,
	}, nil
}

func ManagedBlock(alias string, host *config.Host) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# BEGIN stead %s\n", alias)
	fmt.Fprintf(&b, "Host %s\n", alias)
	if host.Hostname != "" {
		fmt.Fprintf(&b, "    HostName %s\n", host.Hostname)
	}
	if host.User != "" {
		fmt.Fprintf(&b, "    User %s\n", host.User)
	}
	fmt.Fprintf(&b, "    Port %d\n", defaultPort(host.Port))
	if host.IdentityFile != "" {
		fmt.Fprintf(&b, "    IdentityFile %s\n", host.IdentityFile)
	}
	fmt.Fprintln(&b, "    AddKeysToAgent yes")
	fmt.Fprintln(&b, "    IdentitiesOnly yes")
	fmt.Fprintf(&b, "# END stead %s\n", alias)
	return b.String()
}

func DefaultSSHConfigPath(home string) string {
	return filepath.Join(home, ".ssh", "config")
}

func state(existing []byte, alias, block string) (string, error) {
	current := string(existing)
	if current == "" {
		return "add", nil
	}

	begin := markerBegin(alias)
	end := markerEnd(alias)
	beginIndex := strings.Index(current, begin)
	endIndex := strings.Index(current, end)

	if beginIndex == -1 && endIndex == -1 {
		return "add", nil
	}
	if beginIndex == -1 || endIndex == -1 || endIndex < beginIndex {
		return "", fmt.Errorf("malformed managed block for alias %q", alias)
	}

	endIndex += len(end)
	if endIndex < len(current) && current[endIndex] == '\r' {
		endIndex++
	}
	if endIndex < len(current) && current[endIndex] == '\n' {
		endIndex++
	}

	existingBlock := current[beginIndex:endIndex]
	if existingBlock == block {
		return "unchanged", nil
	}
	return "replace", nil
}

func markerBegin(alias string) string {
	return "# BEGIN stead " + alias
}

func markerEnd(alias string) string {
	return "# END stead " + alias
}

func indent(value, prefix string) string {
	lines := strings.SplitAfter(value, "\n")
	var b strings.Builder
	for _, line := range lines {
		if line == "" {
			continue
		}
		b.WriteString(prefix)
		b.WriteString(line)
	}
	return b.String()
}

func hasInclude(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(stripComment(line))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.EqualFold(fields[0], "Include") {
			return true
		}
	}
	return false
}

func stripComment(line string) string {
	inQuote := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inQuote {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == '#' && !inQuote {
			return line[:i]
		}
	}
	return line
}

func defaultPort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}
