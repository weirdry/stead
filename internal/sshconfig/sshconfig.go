package sshconfig

import (
	"bufio"
	"io"
	"os"
	"strings"
)

type Config struct {
	Hosts      map[string]Host
	HasInclude bool
}

type Host struct {
	Patterns     []string
	HostName     string
	User         string
	Port         string
	IdentityFile string
}

type AliasStatus struct {
	Alias    string
	State    string
	Findings []string
	Host     Host
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return Parse(f)
}

func Parse(r io.Reader) (*Config, error) {
	cfg := &Config{Hosts: make(map[string]Host)}
	scanner := bufio.NewScanner(r)
	var current []string

	for scanner.Scan() {
		line := stripComment(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		key := strings.ToLower(fields[0])
		values := fields[1:]

		switch key {
		case "include":
			cfg.HasInclude = true
		case "host":
			current = values
			for _, pattern := range current {
				host := cfg.Hosts[pattern]
				host.Patterns = current
				cfg.Hosts[pattern] = host
			}
		case "hostname", "user", "port", "identityfile":
			if len(current) == 0 || len(values) == 0 {
				continue
			}
			value := strings.Join(values, " ")
			for _, pattern := range current {
				host := cfg.Hosts[pattern]
				host.Patterns = current
				switch key {
				case "hostname":
					host.HostName = value
				case "user":
					host.User = value
				case "port":
					host.Port = value
				case "identityfile":
					host.IdentityFile = value
				}
				cfg.Hosts[pattern] = host
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func CheckAlias(cfg *Config, alias string) AliasStatus {
	host, ok := cfg.Hosts[alias]
	if !ok {
		return AliasStatus{Alias: alias, State: "missing", Findings: []string{"Host entry not found"}}
	}

	findings := make([]string, 0)
	if host.HostName == "" {
		findings = append(findings, "HostName missing")
	}
	if host.User == "" {
		findings = append(findings, "User missing")
	}
	if host.IdentityFile == "" {
		findings = append(findings, "IdentityFile missing")
	}

	state := "ok"
	if len(findings) > 0 {
		state = "incomplete"
	}
	return AliasStatus{Alias: alias, State: state, Findings: findings, Host: host}
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
