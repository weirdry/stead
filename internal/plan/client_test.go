package plan

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ed/stead/internal/config"
)

func TestWriteClientUsesConfiguredAlias(t *testing.T) {
	cfg := testConfig()
	var buf bytes.Buffer

	if err := WriteClient(&buf, cfg, "/tmp/config.toml", "devmac"); err != nil {
		t.Fatalf("WriteClient returned error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Stead client plan",
		"Alias: devmac",
		"Host devmac",
		"HostName devmac.tailnet.example",
		"User ed",
		"Port 22",
		"IdentityFile ~/.ssh/stead_ed25519",
		"Tailscale SSH: not used",
		"Behavior: send Wake-on-LAN, wait for SSH port, then exec system ssh",
		"tmux: enabled (main)",
		"Readiness\n  ok",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestWriteClientUsesDefaultAlias(t *testing.T) {
	cfg := testConfig()
	var buf bytes.Buffer

	if err := WriteClient(&buf, cfg, "/tmp/config.toml", ""); err != nil {
		t.Fatalf("WriteClient returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Alias: devmac") {
		t.Fatalf("output did not use default alias:\n%s", buf.String())
	}
}

func TestWriteClientMissingAlias(t *testing.T) {
	cfg := testConfig()
	var buf bytes.Buffer

	err := WriteClient(&buf, cfg, "/tmp/config.toml", "other")
	if err == nil {
		t.Fatal("expected missing alias error")
	}
	if !strings.Contains(err.Error(), `alias "other" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": {
				Hostname:         "devmac.tailnet.example",
				User:             "ed",
				Port:             22,
				IdentityFile:     "~/.ssh/stead_ed25519",
				PreferredNetwork: "tailscale",
				Wake: config.Wake{
					MACAddress: "configured-mac-address",
					Broadcast:  "configured-broadcast-address",
					Timeout:    "90s",
					Interval:   "2s",
				},
				Session: config.Session{
					Tmux:        true,
					TmuxSession: "main",
					ProjectDir:  "/Users/ed/_GIT",
				},
			},
		},
	}
}
