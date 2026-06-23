package clientwake

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ed/stead/internal/config"
)

func TestRunDryRunDoesNotWriteWakeConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeFixture(t, dir)
	var buf bytes.Buffer

	err := Run(Options{
		Alias:      "devmac",
		ConfigPath: cfgPath,
		MACAddress: validTestMAC(),
		Broadcast:  "192.0.2.255",
		DryRun:     true,
		Out:        &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Hosts["devmac"].Wake.MACAddress != "<host-mac-address>" {
		t.Fatalf("wake MAC was modified during dry-run")
	}
	for _, want := range []string{
		"Stead client wake-config",
		"dry-run",
		"would update wake config",
		"No files were modified.",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunApplyWritesWakeConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeFixture(t, dir)
	var buf bytes.Buffer

	err := Run(Options{
		Alias:      "devmac",
		ConfigPath: cfgPath,
		MACAddress: validTestMAC(),
		Broadcast:  "192.0.2.255",
		Timeout:    "2m",
		Interval:   "3s",
		Out:        &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	wake := cfg.Hosts["devmac"].Wake
	if wake.MACAddress != validTestMAC() {
		t.Fatalf("wake MAC = %q", wake.MACAddress)
	}
	if wake.Broadcast != "192.0.2.255" || wake.Timeout != "2m" || wake.Interval != "3s" {
		t.Fatalf("wake config = %#v", wake)
	}
	if !strings.Contains(buf.String(), "updated wake config") {
		t.Fatalf("output missing update:\n%s", buf.String())
	}
}

func TestRunRejectsInvalidWakeConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeFixture(t, dir)
	for _, tc := range []Options{
		{MACAddress: "not-a-mac"},
		{Broadcast: "not-an-ip"},
		{Timeout: "not-a-duration"},
		{Interval: "not-a-duration"},
	} {
		tc.Alias = "devmac"
		tc.ConfigPath = cfgPath
		tc.Out = &bytes.Buffer{}
		if err := Run(tc); err == nil {
			t.Fatalf("expected error for %#v", tc)
		}
	}
}

func writeFixture(t *testing.T, dir string) string {
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
				Wake: config.Wake{
					MACAddress: "<host-mac-address>",
					Broadcast:  "<lan-broadcast-address>",
					Timeout:    "90s",
					Interval:   "2s",
				},
			},
		},
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	return cfgPath
}

func validTestMAC() string {
	return strings.Join([]string{"02", "00", "00", "00", "00", "01"}, ":")
}
