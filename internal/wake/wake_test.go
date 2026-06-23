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
		"stead wake --alias devmac",
		"stead connect --alias devmac --wake",
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
	cfgPath := writeWakeFixture(t, t.TempDir(), config.Wake{})
	err := Run(Options{
		ConfigPath: cfgPath,
		Out:        &bytes.Buffer{},
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("offline")
		},
	})
	if err == nil {
		t.Fatal("expected wake config error")
	}
}

func TestRunApplySkipsPacketWhenReachable(t *testing.T) {
	cfgPath := writeWakeFixture(t, t.TempDir(), config.Wake{
		MACAddress: validTestMAC(),
		Broadcast:  "configured-broadcast-address",
	})
	var sent bool
	var buf bytes.Buffer
	err := Run(Options{
		Alias:      "devmac",
		ConfigPath: cfgPath,
		Out:        &buf,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return noopConn{}, nil
		},
		Send: func(network, address string, payload []byte) error {
			sent = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if sent {
		t.Fatal("wake packet sent even though SSH was reachable")
	}
	if !strings.Contains(buf.String(), "skipped; SSH already reachable") {
		t.Fatalf("output missing skip:\n%s", buf.String())
	}
}

func TestRunApplySendsPacketAndWaits(t *testing.T) {
	cfgPath := writeWakeFixture(t, t.TempDir(), config.Wake{
		MACAddress: validTestMAC(),
		Broadcast:  "192.0.2.255",
		Timeout:    "20ms",
		Interval:   "1ms",
	})
	var dialCount int
	var sendAddress string
	var payloadLen int
	var buf bytes.Buffer
	err := Run(Options{
		Alias:      "devmac",
		ConfigPath: cfgPath,
		Out:        &buf,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialCount++
			if dialCount == 1 {
				return nil, errors.New("offline")
			}
			return noopConn{}, nil
		},
		Send: func(network, address string, payload []byte) error {
			sendAddress = address
			payloadLen = len(payload)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if sendAddress != "192.0.2.255:9" {
		t.Fatalf("send address = %q", sendAddress)
	}
	if payloadLen != 102 {
		t.Fatalf("payload length = %d, want 102", payloadLen)
	}
	for _, want := range []string{
		"Wake-on-LAN:",
		"packet sent",
		"SSH port:",
		"reachable",
		"stead connect --alias devmac",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
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

func validTestMAC() string {
	return strings.Join([]string{"02", "00", "00", "00", "00", "01"}, ":")
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
