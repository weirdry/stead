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
	err := Reload(ReloadOptions{Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected mode error")
	}
}

func TestReloadApplyRequiresConfirm(t *testing.T) {
	err := Reload(ReloadOptions{Apply: true, Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected confirm error")
	}
	if !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestReloadPlanPrintsManualCommands(t *testing.T) {
	var buf bytes.Buffer
	err := Reload(ReloadOptions{DryRun: true, Out: &buf})
	if err != nil {
		t.Fatalf("Reload returned error: %v", err)
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

func TestReloadApplyRunsValidationThenKickstart(t *testing.T) {
	var calls []string
	var buf bytes.Buffer
	err := Reload(ReloadOptions{
		Apply:   true,
		Confirm: true,
		Out:     &buf,
		Runner: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}
	wantCalls := []string{
		"/usr/sbin/sshd -t",
		"launchctl kickstart -k system/com.openssh.sshd",
	}
	if strings.Join(calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
	for _, want := range []string{
		"Stead host reload",
		"Mode:",
		"apply",
		"sshd -t:",
		"ok",
		"launchctl kickstart:",
		"ok",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestReloadApplyStopsWhenValidationFails(t *testing.T) {
	var calls []string
	err := Reload(ReloadOptions{
		Apply:   true,
		Confirm: true,
		Out:     &bytes.Buffer{},
		Runner: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil, errors.New("bad sshd config")
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(calls) != 1 || calls[0] != "/usr/sbin/sshd -t" {
		t.Fatalf("unexpected calls after validation failure: %#v", calls)
	}
}
