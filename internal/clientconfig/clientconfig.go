package clientconfig

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ed/stead/internal/config"
)

type Change struct {
	Alias      string
	Path       string
	Block      string
	State      string
	NewContent []byte
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

func WriteApply(out io.Writer, cfg *config.Config, cfgPath, sshConfigPath, alias string) error {
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

	existing, existed, mode, err := readExisting(sshConfigPath)
	if err != nil {
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
	fmt.Fprintln(out, "Mode: apply")
	if hasInclude(existing) {
		fmt.Fprintln(out, "Note: Include directive present; included files are not expanded")
	}
	fmt.Fprintln(out)

	if change.State == "unchanged" {
		fmt.Fprintln(out, "Action: no changes needed")
		fmt.Fprintln(out, "Backup: not created")
		fmt.Fprintln(out, "No files were modified.")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(sshConfigPath), 0o700); err != nil {
		return err
	}

	backupPath := ""
	if existed {
		backupPath = sshConfigPath + ".stead-backup-" + time.Now().UTC().Format("20060102T150405.000000000Z")
		if err := os.WriteFile(backupPath, existing, mode); err != nil {
			return err
		}
	}

	if err := writeAtomic(sshConfigPath, change.NewContent, mode); err != nil {
		return err
	}

	switch change.State {
	case "add":
		fmt.Fprintln(out, "Action: added managed SSH config block")
	case "replace":
		fmt.Fprintln(out, "Action: replaced existing managed SSH config block")
	default:
		fmt.Fprintf(out, "Action: %s\n", change.State)
	}
	if backupPath != "" {
		fmt.Fprintf(out, "Backup: %s\n", backupPath)
	} else {
		fmt.Fprintln(out, "Backup: not created (SSH config did not exist)")
	}
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Managed block")
	fmt.Fprint(out, indent(change.Block, "  "))
	return nil
}

func Plan(existing []byte, path, alias string, host *config.Host) (Change, error) {
	block := ManagedBlock(alias, host)
	state, newContent, err := plannedContent(existing, alias, block)
	if err != nil {
		return Change{}, err
	}
	return Change{
		Alias:      alias,
		Path:       path,
		Block:      block,
		State:      state,
		NewContent: newContent,
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

func plannedContent(existing []byte, alias, block string) (string, []byte, error) {
	current := string(existing)
	if current == "" {
		return "add", []byte(block), nil
	}

	beginIndex, endIndex, found, err := managedRange(current, alias)
	if err != nil {
		return "", nil, err
	}
	if !found {
		return "add", []byte(appendBlock(current, block)), nil
	}

	existingBlock := current[beginIndex:endIndex]
	if existingBlock == block {
		return "unchanged", existing, nil
	}
	return "replace", []byte(current[:beginIndex] + block + current[endIndex:]), nil
}

func managedRange(current, alias string) (int, int, bool, error) {
	begin := markerBegin(alias)
	end := markerEnd(alias)
	beginIndex := strings.Index(current, begin)
	endIndex := strings.Index(current, end)

	if beginIndex == -1 && endIndex == -1 {
		return 0, 0, false, nil
	}
	if beginIndex == -1 || endIndex == -1 || endIndex < beginIndex {
		return 0, 0, false, fmt.Errorf("malformed managed block for alias %q", alias)
	}

	endIndex += len(end)
	if endIndex < len(current) && current[endIndex] == '\r' {
		endIndex++
	}
	if endIndex < len(current) && current[endIndex] == '\n' {
		endIndex++
	}
	return beginIndex, endIndex, true, nil
}

func appendBlock(current, block string) string {
	var b strings.Builder
	b.WriteString(current)
	if !strings.HasSuffix(current, "\n") {
		b.WriteString("\n")
	}
	if !strings.HasSuffix(b.String(), "\n\n") {
		b.WriteString("\n")
	}
	b.WriteString(block)
	return b.String()
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

func readExisting(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, 0o600, nil
		}
		return nil, false, 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, 0, err
	}
	return data, true, info.Mode().Perm(), nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".stead-config-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func defaultPort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}
