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
mac_address = "aa:bb:cc:dd:ee:ff"
broadcast = "192.0.2.255"
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
	if host.Wake.MACAddress != "aa:bb:cc:dd:ee:ff" {
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
