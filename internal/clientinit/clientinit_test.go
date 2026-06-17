package clientinit

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ed/stead/internal/config"
)

func TestRunDryRunDoesNotWriteFiles(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	keyPath := filepath.Join(dir, "stead_devmac_ed25519")
	var buf bytes.Buffer

	err := Run(Options{
		Alias:        "devmac",
		Hostname:     "devmac.tailnet.example",
		User:         "ed",
		IdentityFile: keyPath,
		ConfigPath:   cfgPath,
		DryRun:       true,
		Out:          &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run created config: %v", err)
	}
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run created key: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Mode: dry-run",
		"Config action: would create config",
		"Key action: would generate Ed25519 key",
		"No files were modified.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunWritesConfigAndGeneratesKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	keyPath := filepath.Join(dir, "stead_devmac_ed25519")
	publicKey := keyPath + ".pub"
	var generatedPath string

	err := Run(Options{
		Alias:        "devmac",
		Hostname:     "devmac.tailnet.example",
		User:         "ed",
		IdentityFile: keyPath,
		ConfigPath:   cfgPath,
		Out:          &bytes.Buffer{},
		Keygen: func(path, comment string) error {
			generatedPath = path
			if comment != "stead devmac" {
				t.Fatalf("comment = %q", comment)
			}
			if err := os.WriteFile(path, []byte("private test key"), 0o600); err != nil {
				return err
			}
			return os.WriteFile(publicKey, []byte("ssh-ed25519 test-public-key stead devmac\n"), 0o644)
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if generatedPath != keyPath {
		t.Fatalf("generated path = %q", generatedPath)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	host := cfg.Hosts["devmac"]
	if host == nil {
		t.Fatal("devmac host missing")
	}
	if host.Hostname != "devmac.tailnet.example" {
		t.Fatalf("hostname = %q", host.Hostname)
	}
	if host.IdentityFile != keyPath {
		t.Fatalf("identity_file = %q", host.IdentityFile)
	}
	if host.PreferredNetwork != "tailscale" {
		t.Fatalf("preferred_network = %q", host.PreferredNetwork)
	}
}

func TestRunUsesPromptedHostname(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	keyPath := filepath.Join(dir, "key")
	var buf bytes.Buffer

	err := Run(Options{
		Alias:        "devmac",
		User:         "ed",
		IdentityFile: keyPath,
		ConfigPath:   cfgPath,
		DryRun:       true,
		In:           strings.NewReader("devmac.tailnet.example\n"),
		Out:          &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Hostname: devmac.tailnet.example") {
		t.Fatalf("output missing prompted hostname:\n%s", buf.String())
	}
}

func TestRunYesRequiresHostname(t *testing.T) {
	err := Run(Options{
		Alias:  "devmac",
		Yes:    true,
		DryRun: true,
		Out:    &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected hostname error")
	}
	if !strings.Contains(err.Error(), "--hostname is required with --yes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunKeygenFailureDoesNotWriteConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	keyPath := filepath.Join(dir, "stead_devmac_ed25519")
	errKeygen := errors.New("keygen failed")

	err := Run(Options{
		Alias:        "devmac",
		Hostname:     "devmac.tailnet.example",
		User:         "ed",
		IdentityFile: keyPath,
		ConfigPath:   cfgPath,
		Out:          &bytes.Buffer{},
		Keygen: func(path, comment string) error {
			return errKeygen
		},
	})
	if !errors.Is(err, errKeygen) {
		t.Fatalf("error = %v, want %v", err, errKeygen)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("config written after keygen failure: %v", err)
	}
}
