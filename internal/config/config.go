package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config is Stead's local configuration schema.
type Config struct {
	Defaults Defaults
	Hosts    map[string]*Host
}

type Defaults struct {
	Alias string
}

type Host struct {
	Hostname         string
	User             string
	Port             int
	IdentityFile     string
	PreferredNetwork string
	Wake             Wake
	Session          Session
}

type Wake struct {
	MACAddress string
	Broadcast  string
	Timeout    string
	Interval   string
}

type Session struct {
	Tmux        bool
	TmuxSession string
	ProjectDir  string
}

// DefaultPath returns ~/.config/stead/config.toml for the current user.
func DefaultPath() string {
	u, err := user.Current()
	if err != nil || u == nil || u.HomeDir == "" {
		return filepath.Join("~", ".config", "stead", "config.toml")
	}
	return filepath.Join(u.HomeDir, ".config", "stead", "config.toml")
}

// LoadDefault loads the default config path.
func LoadDefault() (*Config, string, error) {
	path := DefaultPath()
	cfg, err := Load(path)
	return cfg, path, err
}

// Load reads a Stead config file.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return Parse(f)
}

// Parse parses the subset of TOML used by Stead's config schema.
func Parse(r io.Reader) (*Config, error) {
	cfg := &Config{Hosts: make(map[string]*Host)}
	scanner := bufio.NewScanner(r)
	section := ""
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := stripComment(scanner.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if section == "" {
				return nil, fmt.Errorf("line %d: empty section", lineNo)
			}
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNo)
		}
		if err := assign(cfg, section, key, value, lineNo); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// WriteSummary prints a redacted, stable summary of the config.
func WriteSummary(out io.Writer, cfg *Config, path string) {
	fmt.Fprintln(out, "Stead config")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Path: %s\n", path)
	fmt.Fprintf(out, "Default alias: %s\n", valueOrUnset(cfg.Defaults.Alias))
	fmt.Fprintln(out)

	if len(cfg.Hosts) == 0 {
		fmt.Fprintln(out, "Hosts: none")
		return
	}

	fmt.Fprintln(out, "Hosts:")
	for _, alias := range sortedAliases(cfg.Hosts) {
		host := cfg.Hosts[alias]
		fmt.Fprintf(out, "  %s\n", alias)
		fmt.Fprintf(out, "    Hostname: %s\n", valueOrUnset(host.Hostname))
		fmt.Fprintf(out, "    User: %s\n", valueOrUnset(host.User))
		fmt.Fprintf(out, "    Port: %d\n", defaultPort(host.Port))
		fmt.Fprintf(out, "    IdentityFile: %s\n", valueOrUnset(host.IdentityFile))
		fmt.Fprintf(out, "    PreferredNetwork: %s\n", valueOrUnset(host.PreferredNetwork))
		fmt.Fprintf(out, "    Wake: %s\n", wakeSummary(host.Wake))
		fmt.Fprintf(out, "    Session: %s\n", sessionSummary(host.Session))
	}
}

func assign(cfg *Config, section, key, raw string, lineNo int) error {
	switch {
	case section == "defaults":
		return assignDefaults(&cfg.Defaults, key, raw, lineNo)
	case strings.HasPrefix(section, "hosts."):
		return assignHost(cfg, strings.TrimPrefix(section, "hosts."), key, raw, lineNo)
	default:
		return fmt.Errorf("line %d: unsupported section %q", lineNo, section)
	}
}

func assignDefaults(defaults *Defaults, key, raw string, lineNo int) error {
	value, err := parseString(raw)
	if err != nil {
		return fmt.Errorf("line %d: %w", lineNo, err)
	}
	switch key {
	case "alias":
		defaults.Alias = value
	default:
		return fmt.Errorf("line %d: unsupported defaults key %q", lineNo, key)
	}
	return nil
}

func assignHost(cfg *Config, section, key, raw string, lineNo int) error {
	parts := strings.Split(section, ".")
	if len(parts) == 0 || parts[0] == "" {
		return fmt.Errorf("line %d: missing host alias", lineNo)
	}
	alias := parts[0]
	host := cfg.Hosts[alias]
	if host == nil {
		host = &Host{}
		cfg.Hosts[alias] = host
	}

	switch len(parts) {
	case 1:
		return assignHostRoot(host, key, raw, lineNo)
	case 2:
		switch parts[1] {
		case "wake":
			return assignWake(&host.Wake, key, raw, lineNo)
		case "session":
			return assignSession(&host.Session, key, raw, lineNo)
		default:
			return fmt.Errorf("line %d: unsupported host subsection %q", lineNo, parts[1])
		}
	default:
		return fmt.Errorf("line %d: unsupported nested section %q", lineNo, section)
	}
}

func assignHostRoot(host *Host, key, raw string, lineNo int) error {
	switch key {
	case "hostname":
		value, err := parseString(raw)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		host.Hostname = value
	case "user":
		value, err := parseString(raw)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		host.User = value
	case "port":
		value, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("line %d: invalid port %q", lineNo, raw)
		}
		host.Port = value
	case "identity_file":
		value, err := parseString(raw)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		host.IdentityFile = value
	case "preferred_network":
		value, err := parseString(raw)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		host.PreferredNetwork = value
	default:
		return fmt.Errorf("line %d: unsupported host key %q", lineNo, key)
	}
	return nil
}

func assignWake(wake *Wake, key, raw string, lineNo int) error {
	value, err := parseString(raw)
	if err != nil {
		return fmt.Errorf("line %d: %w", lineNo, err)
	}
	switch key {
	case "mac_address":
		wake.MACAddress = value
	case "broadcast":
		wake.Broadcast = value
	case "timeout":
		wake.Timeout = value
	case "interval":
		wake.Interval = value
	default:
		return fmt.Errorf("line %d: unsupported wake key %q", lineNo, key)
	}
	return nil
}

func assignSession(session *Session, key, raw string, lineNo int) error {
	switch key {
	case "tmux":
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("line %d: invalid bool %q", lineNo, raw)
		}
		session.Tmux = value
	case "tmux_session":
		value, err := parseString(raw)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		session.TmuxSession = value
	case "project_dir":
		value, err := parseString(raw)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		session.ProjectDir = value
	default:
		return fmt.Errorf("line %d: unsupported session key %q", lineNo, key)
	}
	return nil
}

func parseString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", fmt.Errorf("expected quoted string")
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return "", fmt.Errorf("invalid quoted string: %w", err)
	}
	return value, nil
}

func stripComment(line string) string {
	inString := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if r == '#' && !inString {
			return line[:i]
		}
	}
	return line
}

func sortedAliases(hosts map[string]*Host) []string {
	aliases := make([]string, 0, len(hosts))
	for alias := range hosts {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

func valueOrUnset(value string) string {
	if value == "" {
		return "(unset)"
	}
	return value
}

func defaultPort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}

func wakeSummary(wake Wake) string {
	if wake.MACAddress == "" && wake.Broadcast == "" && wake.Timeout == "" && wake.Interval == "" {
		return "not configured"
	}
	parts := make([]string, 0, 4)
	if wake.MACAddress != "" {
		parts = append(parts, "mac configured")
	}
	if wake.Broadcast != "" {
		parts = append(parts, "broadcast "+wake.Broadcast)
	}
	if wake.Timeout != "" {
		parts = append(parts, "timeout "+wake.Timeout)
	}
	if wake.Interval != "" {
		parts = append(parts, "interval "+wake.Interval)
	}
	return strings.Join(parts, ", ")
}

func sessionSummary(session Session) string {
	parts := make([]string, 0, 3)
	if session.Tmux {
		parts = append(parts, "tmux enabled")
	} else {
		parts = append(parts, "tmux disabled")
	}
	if session.TmuxSession != "" {
		parts = append(parts, "session "+session.TmuxSession)
	}
	if session.ProjectDir != "" {
		parts = append(parts, "project "+session.ProjectDir)
	}
	return strings.Join(parts, ", ")
}
