package clientconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ed/stead/internal/config"
)

func TestManagedBlock(t *testing.T) {
	block := ManagedBlock("devmac", testHost())

	for _, want := range []string{
		"# BEGIN stead devmac",
		"Host devmac",
		"HostName devmac.tailnet.example",
		"User ed",
		"Port 22",
		"IdentityFile ~/.ssh/stead_ed25519",
		"AddKeysToAgent yes",
		"IdentitiesOnly yes",
		"# END stead devmac",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("block missing %q:\n%s", want, block)
		}
	}
}

func TestPlanAddReplaceUnchanged(t *testing.T) {
	host := testHost()
	block := ManagedBlock("devmac", host)

	add, err := Plan([]byte("Host other\n    HostName other.example\n"), "/tmp/config", "devmac", host)
	if err != nil {
		t.Fatalf("Plan add returned error: %v", err)
	}
	if add.State != "add" {
		t.Fatalf("add state = %q", add.State)
	}

	unchanged, err := Plan([]byte(block), "/tmp/config", "devmac", host)
	if err != nil {
		t.Fatalf("Plan unchanged returned error: %v", err)
	}
	if unchanged.State != "unchanged" {
		t.Fatalf("unchanged state = %q", unchanged.State)
	}

	replacedBlock := strings.Replace(block, "User ed", "User other", 1)
	replace, err := Plan([]byte(replacedBlock), "/tmp/config", "devmac", host)
	if err != nil {
		t.Fatalf("Plan replace returned error: %v", err)
	}
	if replace.State != "replace" {
		t.Fatalf("replace state = %q", replace.State)
	}
}

func TestPlanMalformedManagedBlock(t *testing.T) {
	_, err := Plan([]byte("# BEGIN stead devmac\nHost devmac\n"), "/tmp/config", "devmac", testHost())
	if err == nil {
		t.Fatal("expected malformed marker error")
	}
	if !strings.Contains(err.Error(), "malformed managed block") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteDryRunDoesNotModifySSHConfig(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")
	original := "Host other\n    HostName other.example\n"
	if err := os.WriteFile(sshConfig, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg := &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": testHost(),
		},
	}

	var buf bytes.Buffer
	if err := WriteDryRun(&buf, cfg, "/tmp/stead-config.toml", sshConfig, ""); err != nil {
		t.Fatalf("WriteDryRun returned error: %v", err)
	}

	data, err := os.ReadFile(sshConfig)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != original {
		t.Fatalf("dry-run modified file:\n%s", string(data))
	}

	out := buf.String()
	for _, want := range []string{
		"Mode: dry-run",
		"Action: would add managed SSH config block",
		"# BEGIN stead devmac",
		"No files were modified.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestWriteDryRunReportsIncludeDirective(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")
	if err := os.WriteFile(sshConfig, []byte("Include ~/.ssh/config.d/*\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg := &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": testHost(),
		},
	}

	var buf bytes.Buffer
	if err := WriteDryRun(&buf, cfg, "/tmp/stead-config.toml", sshConfig, "devmac"); err != nil {
		t.Fatalf("WriteDryRun returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Note: Include directive present; included files are not expanded") {
		t.Fatalf("output missing Include note:\n%s", buf.String())
	}
}

func testHost() *config.Host {
	return &config.Host{
		Hostname:     "devmac.tailnet.example",
		User:         "ed",
		Port:         22,
		IdentityFile: "~/.ssh/stead_ed25519",
	}
}
