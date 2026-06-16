package config

import (
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
