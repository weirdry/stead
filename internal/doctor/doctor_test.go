package doctor

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

func TestRunReportsMissingConfig(t *testing.T) {
	var buf bytes.Buffer
	err := Run(Options{
		ConfigPath:    filepath.Join(t.TempDir(), "missing.toml"),
		SSHConfigPath: filepath.Join(t.TempDir(), "ssh_config"),
		Out:           &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{
		"Stead doctor",
		"Stead config:",
		"missing",
		"stead config init",
		"stead client init --alias devmac --discover tailscale --yes",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunReportsReadyConfigWithoutVerify(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath := writeFixture(t, dir, "devmac")
	var buf bytes.Buffer
	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Out:           &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{
		"Configured host:",
		"Client key:",
		"SSH alias:",
		"Wake config:",
		"SSH login:",
		"not checked; pass --verify",
		"stead doctor --alias devmac --verify",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunSuggestsClientApplyForMissingSSHAlias(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath := writeFixture(t, dir, "devmac")
	if err := os.WriteFile(sshConfigPath, []byte("Host other\n    HostName other.example\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var buf bytes.Buffer
	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Out:           &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{
		"SSH alias:",
		"missing",
		"stead client apply --dry-run --alias devmac",
		"stead client apply --alias devmac",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunVerifyReportsSuccess(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath := writeFixture(t, dir, "devmac")
	var called bool
	var buf bytes.Buffer
	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Verify:        true,
		Out:           &buf,
		Runner: func(ctx context.Context, alias string) error {
			called = true
			if alias != "devmac" {
				t.Fatalf("alias = %q", alias)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called {
		t.Fatal("verify runner was not called")
	}
	if !strings.Contains(buf.String(), "BatchMode login succeeded") {
		t.Fatalf("output missing verify success:\n%s", buf.String())
	}
}

func TestRunVerifyReportsFailure(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath := writeFixture(t, dir, "devmac")
	var buf bytes.Buffer
	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Verify:        true,
		Out:           &buf,
		Runner: func(ctx context.Context, alias string) error {
			return errors.New("login failed")
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{
		"SSH login:",
		"failed",
		"login failed",
		"stead verify --alias devmac",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func writeFixture(t *testing.T, dir, alias string) (string, string) {
	t.Helper()
	identity := filepath.Join(dir, "stead_ed25519")
	if err := os.WriteFile(identity, []byte("private"), 0o600); err != nil {
		t.Fatalf("WriteFile private returned error: %v", err)
	}
	if err := os.WriteFile(identity+".pub", []byte("public"), 0o644); err != nil {
		t.Fatalf("WriteFile public returned error: %v", err)
	}
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{
		Defaults: config.Defaults{Alias: alias},
		Hosts: map[string]*config.Host{
			alias: {
				Hostname:     "devmac.example",
				User:         "ed",
				Port:         22,
				IdentityFile: identity,
				Wake: config.Wake{
					MACAddress: "configured-mac",
					Broadcast:  "192.0.2.255",
					Timeout:    "90s",
					Interval:   "2s",
				},
				Session: config.Session{
					Tmux:        true,
					TmuxSession: "main",
				},
			},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sshConfigPath := filepath.Join(dir, "ssh_config")
	sshConfig := "Host " + alias + "\n    HostName devmac.example\n    User ed\n    Port 22\n    IdentityFile " + identity + "\n"
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("WriteFile ssh config returned error: %v", err)
	}
	return cfgPath, sshConfigPath
}
