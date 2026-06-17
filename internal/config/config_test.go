package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	input := `
[defaults]
alias = "devmac"

[hosts.devmac]
hostname = "devmac.example"
user = "ed"
port = 22
identity_file = "~/.ssh/stead_ed25519"
preferred_network = "tailscale"

[hosts.devmac.wake]
mac_address = "configured-mac-address"
broadcast = "configured-broadcast-address"
timeout = "90s"
interval = "2s"

[hosts.devmac.session]
tmux = true
tmux_session = "main"
project_dir = "/Users/ed/_GIT/example"
`

	cfg, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cfg.Defaults.Alias != "devmac" {
		t.Fatalf("default alias = %q", cfg.Defaults.Alias)
	}

	host := cfg.Hosts["devmac"]
	if host == nil {
		t.Fatal("devmac host missing")
	}
	if host.Hostname != "devmac.example" {
		t.Fatalf("hostname = %q", host.Hostname)
	}
	if host.Port != 22 {
		t.Fatalf("port = %d", host.Port)
	}
	if host.Wake.MACAddress != "configured-mac-address" {
		t.Fatalf("wake MAC = %q", host.Wake.MACAddress)
	}
	if !host.Session.Tmux {
		t.Fatal("tmux should be enabled")
	}
}

func TestParseRejectsUnknownSection(t *testing.T) {
	_, err := Parse(strings.NewReader("[unknown]\nvalue = \"x\"\n"))
	if err == nil {
		t.Fatal("expected unknown section error")
	}
}

func TestStarterConfigIsParseable(t *testing.T) {
	var buf bytes.Buffer
	WriteStarter(&buf)

	cfg, err := Parse(&buf)
	if err != nil {
		t.Fatalf("starter config should parse: %v", err)
	}
	if cfg.Defaults.Alias != "devmac" {
		t.Fatalf("default alias = %q", cfg.Defaults.Alias)
	}
	if cfg.Hosts["devmac"] == nil {
		t.Fatal("starter devmac host missing")
	}
}

func TestInitWritesConfigAndRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stead", "config.toml")

	if err := Init(path); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if _, err := Parse(bytes.NewReader(data)); err != nil {
		t.Fatalf("written config should parse: %v", err)
	}

	err = Init(path)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("second Init error = %v, want os.ErrExist", err)
	}
}

func TestWriteRoundTrip(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{Alias: "devmac"},
		Hosts: map[string]*Host{
			"devmac": {
				Hostname:         "devmac.tailnet.example",
				User:             "ed",
				Port:             22,
				IdentityFile:     "~/.ssh/stead_devmac_ed25519",
				PreferredNetwork: "tailscale",
				Wake: Wake{
					MACAddress: "configured-mac-address",
					Broadcast:  "configured-broadcast-address",
					Timeout:    "90s",
					Interval:   "2s",
				},
				Session: Session{
					Tmux:        true,
					TmuxSession: "main",
					ProjectDir:  "/Users/ed/_GIT",
				},
			},
		},
	}

	var buf bytes.Buffer
	Write(&buf, cfg)

	parsed, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	host := parsed.Hosts["devmac"]
	if host == nil {
		t.Fatal("devmac host missing")
	}
	if host.Hostname != "devmac.tailnet.example" {
		t.Fatalf("hostname = %q", host.Hostname)
	}
	if !host.Session.Tmux {
		t.Fatal("tmux should be enabled")
	}
}

func TestWriteSummaryMarksPlaceholders(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{Alias: "devmac"},
		Hosts: map[string]*Host{
			"devmac": {
				Hostname: "<tailscale-ip-or-magicdns>",
				User:     "ed",
				Wake: Wake{
					MACAddress: "<host-mac-address>",
					Broadcast:  "<lan-broadcast-address>",
					Timeout:    "90s",
					Interval:   "2s",
				},
			},
		},
	}

	var buf bytes.Buffer
	WriteSummary(&buf, cfg, "/tmp/config.toml")
	out := buf.String()

	if !strings.Contains(out, "Hostname: (placeholder: <tailscale-ip-or-magicdns>)") {
		t.Fatalf("summary did not mark hostname placeholder:\n%s", out)
	}
	if !strings.Contains(out, "Wake: mac placeholder, broadcast placeholder, timeout 90s, interval 2s") {
		t.Fatalf("summary did not mark wake placeholders:\n%s", out)
	}
}

func TestHostStatusesClassifiesRequiredPlaceholders(t *testing.T) {
	cfg := &Config{
		Hosts: map[string]*Host{
			"devmac": {
				Hostname: "<tailscale-ip-or-magicdns>",
				User:     "ed",
				Wake: Wake{
					MACAddress: "<host-mac-address>",
					Broadcast:  "<lan-broadcast-address>",
				},
			},
		},
	}

	statuses := HostStatuses(cfg)
	if len(statuses) != 1 {
		t.Fatalf("status count = %d", len(statuses))
	}
	if statuses[0].State != "incomplete" {
		t.Fatalf("state = %q", statuses[0].State)
	}
	want := []string{"hostname placeholder"}
	for _, finding := range want {
		if !contains(statuses[0].Findings, finding) {
			t.Fatalf("missing finding %q in %#v", finding, statuses[0].Findings)
		}
	}
	for _, finding := range []string{"wake MAC placeholder", "wake broadcast placeholder"} {
		if contains(statuses[0].Findings, finding) {
			t.Fatalf("optional wake finding %q should not block readiness: %#v", finding, statuses[0].Findings)
		}
	}
}

func TestHostStatusesIgnoresOptionalWakePlaceholders(t *testing.T) {
	cfg := &Config{
		Hosts: map[string]*Host{
			"devmac": {
				Hostname: "devmac.example",
				User:     "ed",
				Wake: Wake{
					MACAddress: "<host-mac-address>",
					Broadcast:  "<lan-broadcast-address>",
				},
			},
		},
	}

	statuses := HostStatuses(cfg)
	if len(statuses) != 1 {
		t.Fatalf("status count = %d", len(statuses))
	}
	if statuses[0].State != "ok" {
		t.Fatalf("state = %q; findings = %#v", statuses[0].State, statuses[0].Findings)
	}
}

func TestHostStatusesClassifiesCompleteHost(t *testing.T) {
	cfg := &Config{
		Hosts: map[string]*Host{
			"devmac": {
				Hostname: "devmac.example",
				User:     "ed",
			},
		},
	}

	statuses := HostStatuses(cfg)
	if len(statuses) != 1 {
		t.Fatalf("status count = %d", len(statuses))
	}
	if statuses[0].State != "ok" {
		t.Fatalf("state = %q; findings = %#v", statuses[0].State, statuses[0].Findings)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
