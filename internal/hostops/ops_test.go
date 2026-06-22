package hostops

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateReportsOK(t *testing.T) {
	var buf bytes.Buffer
	err := Validate(ValidateOptions{
		Out:        &buf,
		DropInPath: filepath.Join(t.TempDir(), "stead.conf"),
		Runner: func(name string, args ...string) ([]byte, error) {
			if name != "sshd" || strings.Join(args, " ") != "-t" {
				t.Fatalf("runner got %s %v", name, args)
			}
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Stead host validate",
		"Mode:",
		"read-only",
		"sshd -t:",
		"ok",
		"No files were modified.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestValidateExplainsMacOSHostKeyAccess(t *testing.T) {
	got := validateSSHD(func(name string, args ...string) ([]byte, error) {
		return []byte("sshd: no hostkeys available -- exiting."), errors.New("exit status 1")
	})
	want := "unknown (sshd -t needs root-readable host keys on this macOS; no sudo attempted)"
	if got != want {
		t.Fatalf("validateSSHD = %q, want %q", got, want)
	}
}

func TestReloadPlanRequiresDryRun(t *testing.T) {
	err := ReloadPlan(ReloadOptions{Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected dry-run error")
	}
}

func TestReloadPlanPrintsManualCommands(t *testing.T) {
	var buf bytes.Buffer
	err := ReloadPlan(ReloadOptions{DryRun: true, Out: &buf})
	if err != nil {
		t.Fatalf("ReloadPlan returned error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Stead host reload plan",
		"dry-run",
		"sudo /usr/sbin/sshd -t",
		"sudo launchctl kickstart -k system/com.openssh.sshd",
		"sudo rm /etc/ssh/sshd_config.d/stead.conf",
		"No services were reloaded.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
