package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
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

type HostStatus struct {
	Alias    string
	State    string
	Findings []string
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

// InitDefault writes a starter config at the default path if it does not exist.
func InitDefault() (string, error) {
	path := DefaultPath()
	return path, Init(path)
}

// Init writes a starter config if path does not already exist.
func Init(path string) error {
	var buf bytes.Buffer
	WriteStarter(&buf)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return err
		}
		return err
	}
	defer f.Close()

	_, err = f.Write(buf.Bytes())
	return err
}

// SaveDefault writes a complete Stead config to the default config path.
func SaveDefault(cfg *Config) (string, error) {
	path := DefaultPath()
	return path, Save(path, cfg)
}

// Save writes a complete Stead config.
func Save(path string, cfg *Config) error {
	var buf bytes.Buffer
	Write(&buf, cfg)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// Write serializes Stead config in the supported TOML subset.
func Write(out io.Writer, cfg *Config) {
	fmt.Fprintln(out, "[defaults]")
	fmt.Fprintf(out, "alias = %q\n", cfg.Defaults.Alias)
	fmt.Fprintln(out)

	for _, alias := range sortedAliases(cfg.Hosts) {
		host := cfg.Hosts[alias]
		fmt.Fprintf(out, "[hosts.%s]\n", alias)
		fmt.Fprintf(out, "hostname = %q\n", host.Hostname)
		fmt.Fprintf(out, "user = %q\n", host.User)
		fmt.Fprintf(out, "port = %d\n", defaultPort(host.Port))
		fmt.Fprintf(out, "identity_file = %q\n", host.IdentityFile)
		fmt.Fprintf(out, "preferred_network = %q\n", host.PreferredNetwork)
		fmt.Fprintln(out)

		fmt.Fprintf(out, "[hosts.%s.wake]\n", alias)
		fmt.Fprintf(out, "mac_address = %q\n", host.Wake.MACAddress)
		fmt.Fprintf(out, "broadcast = %q\n", host.Wake.Broadcast)
		fmt.Fprintf(out, "timeout = %q\n", host.Wake.Timeout)
		fmt.Fprintf(out, "interval = %q\n", host.Wake.Interval)
		fmt.Fprintln(out)

		fmt.Fprintf(out, "[hosts.%s.session]\n", alias)
		fmt.Fprintf(out, "tmux = %t\n", host.Session.Tmux)
		fmt.Fprintf(out, "tmux_session = %q\n", host.Session.TmuxSession)
		fmt.Fprintf(out, "project_dir = %q\n", host.Session.ProjectDir)
		fmt.Fprintln(out)
	}
}

// WriteStarter writes a starter config template.
func WriteStarter(out io.Writer) {
	userName := currentUserName()
	fmt.Fprintln(out, `[defaults]
alias = "devmac"

[hosts.devmac]
hostname = "<tailscale-ip-or-magicdns>"
user = "`+userName+`"
port = 22
identity_file = "~/.ssh/stead_ed25519"
preferred_network = "tailscale"

[hosts.devmac.wake]
mac_address = "<host-mac-address>"
broadcast = "<lan-broadcast-address>"
timeout = "90s"
interval = "2s"

[hosts.devmac.session]
tmux = true
tmux_session = "main"
project_dir = "`+starterProjectDir()+`"`)
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
		fmt.Fprintf(out, "    Hostname: %s\n", valueOrUnsetOrPlaceholder(host.Hostname))
		fmt.Fprintf(out, "    User: %s\n", valueOrUnset(host.User))
		fmt.Fprintf(out, "    Port: %d\n", defaultPort(host.Port))
		fmt.Fprintf(out, "    IdentityFile: %s\n", valueOrUnset(host.IdentityFile))
		fmt.Fprintf(out, "    PreferredNetwork: %s\n", valueOrUnset(host.PreferredNetwork))
		fmt.Fprintf(out, "    Wake: %s\n", wakeSummary(host.Wake))
		fmt.Fprintf(out, "    Session: %s\n", sessionSummary(host.Session))
	}
}

// HostStatuses returns stable, display-oriented config health for each host.
func HostStatuses(cfg *Config) []HostStatus {
	statuses := make([]HostStatus, 0, len(cfg.Hosts))
	for _, alias := range sortedAliases(cfg.Hosts) {
		host := cfg.Hosts[alias]
		findings := hostFindings(host)
		state := "ok"
		if len(findings) > 0 {
			state = "incomplete"
		}
		statuses = append(statuses, HostStatus{
			Alias:    alias,
			State:    state,
			Findings: findings,
		})
	}
	return statuses
}

func hostFindings(host *Host) []string {
	findings := make([]string, 0)
	if host.Hostname == "" {
		findings = append(findings, "hostname missing")
	} else if isPlaceholder(host.Hostname) {
		findings = append(findings, "hostname placeholder")
	}
	if host.User == "" {
		findings = append(findings, "user missing")
	}
	return findings
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

func valueOrUnsetOrPlaceholder(value string) string {
	if value == "" {
		return "(unset)"
	}
	if isPlaceholder(value) {
		return "(placeholder: " + value + ")"
	}
	return value
}

func isPlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")
}

func defaultPort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}

func wakeSummary(wake Wake) string {
	hasMAC := wake.MACAddress != "" && !isPlaceholder(wake.MACAddress)
	hasBroadcast := wake.Broadcast != "" && !isPlaceholder(wake.Broadcast)
	if !hasMAC && !hasBroadcast && wake.Timeout == "" && wake.Interval == "" {
		return "not configured"
	}
	parts := make([]string, 0, 4)
	if hasMAC {
		parts = append(parts, "mac configured")
	} else if isPlaceholder(wake.MACAddress) {
		parts = append(parts, "mac placeholder")
	}
	if hasBroadcast {
		parts = append(parts, "broadcast "+wake.Broadcast)
	} else if isPlaceholder(wake.Broadcast) {
		parts = append(parts, "broadcast placeholder")
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

func currentUserName() string {
	u, err := user.Current()
	if err != nil || u == nil || u.Username == "" {
		return "<local-user>"
	}
	return filepath.Base(u.Username)
}

func starterProjectDir() string {
	u, err := user.Current()
	if err != nil || u == nil || u.HomeDir == "" {
		if runtime.GOOS == "windows" {
			return "<project-dir>"
		}
		return "~/project"
	}
	return filepath.Join(u.HomeDir, "_GIT")
}
