package setup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ed/stead/internal/config"
)

func TestWritePlanMissingConfig(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer

	if err := WritePlan(Options{
		Alias:         "devmac",
		ConfigPath:    filepath.Join(dir, "missing.toml"),
		SSHConfigPath: filepath.Join(dir, "ssh_config"),
		Out:           &buf,
	}); err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Stead setup",
		"Alias: devmac",
		"Config: missing",
		"stead client init --alias devmac --discover tailscale --yes",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestWritePlanCompleteClientSetup(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath, keyPath := writeCompleteFixture(t, dir)
	var buf bytes.Buffer

	if err := WritePlan(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Out:           &buf,
	}); err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Config: ok",
		"Private key: ok",
		"Public key: ok",
		"Host devmac: ok",
		"stead host authorize --alias devmac --public-key 'ssh-ed25519 test-public-key stead devmac' --dry-run",
		"stead verify --alias devmac",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, keyPath) {
		t.Fatalf("output missing key path %q:\n%s", keyPath, out)
	}
}

func TestWritePlanVerifyOKSuppressesHostAuthorizeSteps(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath, _ := writeCompleteFixture(t, dir)
	var buf bytes.Buffer

	if err := WritePlan(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Verify:        true,
		Out:           &buf,
		VerifyRunner: func(ctx context.Context, alias string) error {
			if alias != "devmac" {
				t.Fatalf("alias = %q", alias)
			}
			return nil
		},
	}); err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"SSH login: ok",
		"ssh devmac",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "stead host authorize") {
		t.Fatalf("verified setup should not suggest host authorize:\n%s", out)
	}
}

func TestWritePlanVerifyFailureKeepsHostAuthorizeSteps(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath, _ := writeCompleteFixture(t, dir)
	var buf bytes.Buffer

	if err := WritePlan(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Verify:        true,
		Out:           &buf,
		VerifyRunner: func(ctx context.Context, alias string) error {
			return errors.New("ssh failed")
		},
	}); err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"SSH login: failed",
		"stead host authorize --alias devmac",
		"stead verify --alias devmac",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestWritePlanMissingSSHAlias(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath, _ := writeCompleteFixture(t, dir)
	if err := os.WriteFile(sshConfigPath, []byte("Host other\n    HostName other.example\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var buf bytes.Buffer

	if err := WritePlan(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Out:           &buf,
	}); err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Host devmac: missing",
		"stead client apply --dry-run --alias devmac",
		"stead client apply --alias devmac",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestWritePlanPlaceholderHostnameSuggestsDiscovery(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath, _ := writeCompleteFixture(t, dir)
	cfg := &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": {
				Hostname:     "<tailscale-ip-or-magicdns>",
				User:         "ed",
				IdentityFile: filepath.Join(dir, "missing-key"),
			},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	var buf bytes.Buffer

	if err := WritePlan(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Out:           &buf,
	}); err != nil {
		t.Fatalf("WritePlan returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "stead client init --alias devmac --discover tailscale --yes") {
		t.Fatalf("output did not suggest discovery for placeholder hostname:\n%s", out)
	}
	if strings.Contains(out, "--hostname '<tailscale-ip-or-magicdns>'") {
		t.Fatalf("output repeated placeholder hostname:\n%s", out)
	}
}

func writeCompleteFixture(t *testing.T, dir string) (string, string, string) {
	t.Helper()
	keyPath := filepath.Join(dir, "stead_devmac_ed25519")
	if err := os.WriteFile(keyPath, []byte("private test key"), 0o600); err != nil {
		t.Fatalf("WriteFile key returned error: %v", err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte("ssh-ed25519 test-public-key stead devmac\n"), 0o644); err != nil {
		t.Fatalf("WriteFile pub returned error: %v", err)
	}

	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": {
				Hostname:         "devmac.tailnet.example",
				User:             "ed",
				Port:             22,
				IdentityFile:     keyPath,
				PreferredNetwork: "tailscale",
			},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	sshConfigPath := filepath.Join(dir, "ssh_config")
	sshConfig := "Host devmac\n    HostName devmac.tailnet.example\n    User ed\n    Port 22\n    IdentityFile " + keyPath + "\n"
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("WriteFile ssh config returned error: %v", err)
	}
	return cfgPath, sshConfigPath, keyPath
}
