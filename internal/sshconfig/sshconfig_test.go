package sshconfig

import (
	"strings"
	"testing"
)

func TestParseHostAlias(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
Include ~/.colima/ssh_config

Host devmac dev
    HostName devmac.example
    User ed
    Port 22
    IdentityFile ~/.ssh/stead_ed25519
`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !cfg.HasInclude {
		t.Fatal("expected Include to be detected")
	}

	status := CheckAlias(cfg, "devmac")
	if status.State != "ok" {
		t.Fatalf("state = %q; findings = %#v", status.State, status.Findings)
	}
	if status.Host.HostName != "devmac.example" {
		t.Fatalf("HostName = %q", status.Host.HostName)
	}
}

func TestCheckAliasMissing(t *testing.T) {
	cfg, err := Parse(strings.NewReader("Host other\n  HostName other.example\n"))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	status := CheckAlias(cfg, "devmac")
	if status.State != "missing" {
		t.Fatalf("state = %q", status.State)
	}
}
