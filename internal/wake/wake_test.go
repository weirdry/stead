package wake

import (
	"bytes"
	"context"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ed/stead/internal/config"
)

func TestRunDryRunReportsReachable(t *testing.T) {
	cfgPath := writeWakeFixture(t, t.TempDir(), config.Wake{
		MACAddress: "configured-mac-address",
		Broadcast:  "configured-broadcast-address",
		Timeout:    "90s",
		Interval:   "2s",
	})
	var buf bytes.Buffer
	err := Run(Options{
		Alias:      "devmac",
		ConfigPath: cfgPath,
		DryRun:     true,
		Out:        &buf,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != "devmac.example:22" {
				t.Fatalf("address = %q", address)
			}
			return noopConn{}, nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{
		"Stead wake",
		"dry-run",
		"SSH port:",
		"ok",
		"MAC address:",
		"ok",
		"stead connect --alias devmac",
		"No Wake-on-LAN packet was sent.",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunDryRunReportsUnreachable(t *testing.T) {
	cfgPath := writeWakeFixture(t, t.TempDir(), config.Wake{
		MACAddress: "configured-mac-address",
		Broadcast:  "configured-broadcast-address",
	})
	var buf bytes.Buffer
	err := Run(Options{
		ConfigPath: cfgPath,
		DryRun:     true,
		Out:        &buf,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("connection refused")
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{
		"unreachable",
		"connection refused",
		"future wake apply will send Wake-on-LAN",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunDryRunReportsMissingWakeConfig(t *testing.T) {
	cfgPath := writeWakeFixture(t, t.TempDir(), config.Wake{})
	var buf bytes.Buffer
	err := Run(Options{
		ConfigPath: cfgPath,
		DryRun:     true,
		Out:        &buf,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("no route")
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{
		"MAC address:",
		"missing",
		"configure hosts.devmac.wake",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunRequiresDryRun(t *testing.T) {
	err := Run(Options{Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected dry-run error")
	}
}

func TestRunRejectsPlaceholderHostname(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := baseConfig(config.Wake{})
	cfg.Hosts["devmac"].Hostname = "<tailscale-ip-or-magicdns>"
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	err := Run(Options{
		ConfigPath: cfgPath,
		DryRun:     true,
		Out:        &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected hostname error")
	}
}

func writeWakeFixture(t *testing.T, dir string, wake config.Wake) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := config.Save(cfgPath, baseConfig(wake)); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	return cfgPath
}

func baseConfig(wake config.Wake) *config.Config {
	return &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": {
				Hostname: "devmac.example",
				User:     "ed",
				Port:     22,
				Wake:     wake,
			},
		},
	}
}

type noopConn struct{}

func (noopConn) Read(b []byte) (int, error)         { return 0, nil }
func (noopConn) Write(b []byte) (int, error)        { return len(b), nil }
func (noopConn) Close() error                       { return nil }
func (noopConn) LocalAddr() net.Addr                { return nil }
func (noopConn) RemoteAddr() net.Addr               { return nil }
func (noopConn) SetDeadline(t time.Time) error      { return nil }
func (noopConn) SetReadDeadline(t time.Time) error  { return nil }
func (noopConn) SetWriteDeadline(t time.Time) error { return nil }
