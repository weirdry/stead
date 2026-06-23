package clientuninstall

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ed/stead/internal/clientconfig"
	"github.com/ed/stead/internal/config"
)

func TestRunDryRunDoesNotModifySSHConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath, original := writeFixture(t, dir)
	var buf bytes.Buffer

	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		DryRun:        true,
		Out:           &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	data, err := os.ReadFile(sshConfigPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != original {
		t.Fatalf("dry-run modified SSH config:\n%s", string(data))
	}
	for _, want := range []string{
		"Stead client uninstall",
		"would remove managed block",
		"No files were modified.",
		"Left untouched",
		"Private key:",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunApplyRequiresConfirm(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath, _ := writeFixture(t, dir)
	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Apply:         true,
		Out:           &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected confirm error")
	}
}

func TestRunApplyRemovesManagedBlockAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath, _ := writeFixture(t, dir)
	var buf bytes.Buffer
	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Apply:         true,
		Confirm:       true,
		Out:           &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	data, err := os.ReadFile(sshConfigPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "# BEGIN stead devmac") {
		t.Fatalf("managed block still present:\n%s", got)
	}
	if !strings.Contains(got, "Host other") {
		t.Fatalf("unrelated config missing:\n%s", got)
	}
	matches, err := filepath.Glob(sshConfigPath + ".stead-backup-*")
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("backup count = %d, want 1", len(matches))
	}
	if !strings.Contains(buf.String(), "removed managed block") || !strings.Contains(buf.String(), "Backup:") {
		t.Fatalf("output missing apply details:\n%s", buf.String())
	}
}

func TestRunDryRunHandlesAbsentBlock(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath, _ := writeFixture(t, dir)
	if err := os.WriteFile(sshConfigPath, []byte("Host other\n    HostName other.example\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var buf bytes.Buffer
	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		DryRun:        true,
		Out:           &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "no changes needed") {
		t.Fatalf("output missing no-op:\n%s", buf.String())
	}
}

func writeFixture(t *testing.T, dir string) (string, string, string) {
	t.Helper()
	identity := filepath.Join(dir, "stead_devmac_ed25519")
	if err := os.WriteFile(identity, []byte("private"), 0o600); err != nil {
		t.Fatalf("WriteFile private returned error: %v", err)
	}
	if err := os.WriteFile(identity+".pub", []byte("public"), 0o644); err != nil {
		t.Fatalf("WriteFile public returned error: %v", err)
	}
	host := &config.Host{
		Hostname:     "devmac.example",
		User:         "ed",
		Port:         22,
		IdentityFile: identity,
	}
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": host,
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sshConfigPath := filepath.Join(dir, "ssh_config")
	original := "Host other\n    HostName other.example\n\n" + clientconfig.ManagedBlock("devmac", host)
	if err := os.WriteFile(sshConfigPath, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile ssh config returned error: %v", err)
	}
	return cfgPath, sshConfigPath, original
}
