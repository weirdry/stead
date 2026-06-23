package connect

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ed/stead/internal/config"
)

func TestRunExecsSystemSSHForConfiguredAlias(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath := writeFixture(t, dir)
	var gotPath string
	var gotArgv []string
	var buf bytes.Buffer

	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Out:           &buf,
		Exec: func(path string, argv []string, env []string) error {
			gotPath = path
			gotArgv = append([]string(nil), argv...)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/ssh") {
		t.Fatalf("exec path = %q, want ssh", gotPath)
	}
	if len(gotArgv) != 2 || gotArgv[1] != "devmac" {
		t.Fatalf("argv = %#v", gotArgv)
	}
	for _, want := range []string{
		"Stead connect",
		"Alias:",
		"devmac",
		"Tailscale SSH is not used",
		"Exec:",
		"ssh devmac",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunUsesDefaultAlias(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath := writeFixture(t, dir)
	var gotArgv []string

	err := Run(Options{
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Out:           &bytes.Buffer{},
		Exec: func(path string, argv []string, env []string) error {
			gotArgv = append([]string(nil), argv...)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(gotArgv) != 2 || gotArgv[1] != "devmac" {
		t.Fatalf("argv = %#v", gotArgv)
	}
}

func TestRunRejectsMissingSteadHost(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath := writeFixture(t, dir)
	err := Run(Options{
		Alias:         "missing",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Out:           &bytes.Buffer{},
		Exec: func(path string, argv []string, env []string) error {
			t.Fatal("exec should not be called")
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected missing alias error")
	}
}

func TestRunRejectsMissingSSHAlias(t *testing.T) {
	dir := t.TempDir()
	cfgPath, sshConfigPath := writeFixture(t, dir)
	if err := os.WriteFile(sshConfigPath, []byte("Host other\n    HostName other.example\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	err := Run(Options{
		Alias:         "devmac",
		ConfigPath:    cfgPath,
		SSHConfigPath: sshConfigPath,
		Out:           &bytes.Buffer{},
		Exec: func(path string, argv []string, env []string) error {
			t.Fatal("exec should not be called")
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected ssh alias error")
	}
	if !strings.Contains(err.Error(), "stead client apply") {
		t.Fatalf("error = %q", err.Error())
	}
}

func writeFixture(t *testing.T, dir string) (string, string) {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": {
				Hostname:     "devmac.example",
				User:         "ed",
				Port:         22,
				IdentityFile: "~/.ssh/stead_devmac_ed25519",
			},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sshConfigPath := filepath.Join(dir, "ssh_config")
	sshConfig := "Host devmac\n    HostName devmac.example\n    User ed\n    Port 22\n    IdentityFile ~/.ssh/stead_devmac_ed25519\n"
	if err := os.WriteFile(sshConfigPath, []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return cfgPath, sshConfigPath
}
