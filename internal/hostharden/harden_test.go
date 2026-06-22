package hostharden

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigWithPasswordDisabled(t *testing.T) {
	got := Config("ed", true)
	for _, want := range []string{
		"PubkeyAuthentication yes",
		"PasswordAuthentication no",
		"KbdInteractiveAuthentication no",
		"PermitRootLogin no",
		"AllowUsers ed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("config missing %q:\n%s", want, got)
		}
	}
}

func TestConfigWithoutPasswordDisabledLeavesPasswordUnchanged(t *testing.T) {
	got := Config("ed", false)
	for _, unwanted := range []string{
		"PasswordAuthentication",
		"KbdInteractiveAuthentication",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("config unexpectedly contains %q:\n%s", unwanted, got)
		}
	}
}

func TestRunRequiresDryRunOrApply(t *testing.T) {
	err := Run(Options{User: "ed", Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected mode error")
	}
	if !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestRunDryRunPrintsPlanAndDoesNotWrite(t *testing.T) {
	var buf bytes.Buffer
	err := Run(Options{
		User:            "ed",
		DisablePassword: true,
		DryRun:          true,
		DropInPath:      "/tmp/stead.conf",
		Out:             &buf,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Stead host harden",
		"dry-run",
		"/tmp/stead.conf",
		"PasswordAuthentication no",
		"KbdInteractiveAuthentication no",
		"AllowUsers ed",
		"No files were modified.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunRejectsWhitespaceUser(t *testing.T) {
	err := Run(Options{
		User:   "ed admin",
		DryRun: true,
		Out:    &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected user validation error")
	}
}

func TestRunApplyRequiresKeyLoginConfirmationWhenDisablingPassword(t *testing.T) {
	err := Run(Options{
		User:            "ed",
		DisablePassword: true,
		Apply:           true,
		DropInPath:      filepath.Join(t.TempDir(), "stead.conf"),
		Out:             &bytes.Buffer{},
		Validate:        okValidator,
	})
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if !strings.Contains(err.Error(), "--confirm-key-login") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestRunApplyCreatesDropIn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stead.conf")
	var validatedPath string
	var buf bytes.Buffer

	err := Run(Options{
		User:            "ed",
		DisablePassword: true,
		Apply:           true,
		ConfirmKeyLogin: true,
		DropInPath:      path,
		Out:             &buf,
		Validate:        captureValidator(&validatedPath),
		Now:             fixedNow,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(data), "PasswordAuthentication no") {
		t.Fatalf("drop-in missing hardening:\n%s", string(data))
	}
	if validatedPath == "" || validatedPath == path {
		t.Fatalf("expected temp candidate validation path, got %q", validatedPath)
	}
	for _, want := range []string{
		"Mode:",
		"apply",
		"Validation:",
		"ok",
		"created managed drop-in",
		"Reload:",
		"not performed by stead",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunApplyReplacesDropInWithBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stead.conf")
	original := []byte("old config\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var buf bytes.Buffer
	err := Run(Options{
		User:            "ed",
		DisablePassword: true,
		Apply:           true,
		ConfirmKeyLogin: true,
		DropInPath:      path,
		Out:             &buf,
		Validate:        okValidator,
		Now:             fixedNow,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	backup := path + ".stead-backup-20260622T030405.000000000Z"
	backupData, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("ReadFile backup returned error: %v", err)
	}
	if string(backupData) != string(original) {
		t.Fatalf("backup = %q, want %q", string(backupData), string(original))
	}
	if !strings.Contains(buf.String(), "replaced managed drop-in") || !strings.Contains(buf.String(), backup) {
		t.Fatalf("output missing replacement details:\n%s", buf.String())
	}
}

func TestRunApplyNoopsWhenDropInUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stead.conf")
	content := Config("ed", true)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var buf bytes.Buffer
	err := Run(Options{
		User:            "ed",
		DisablePassword: true,
		Apply:           true,
		ConfirmKeyLogin: true,
		DropInPath:      path,
		Out:             &buf,
		Validate:        okValidator,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "no changes needed") {
		t.Fatalf("output missing no-op action:\n%s", buf.String())
	}
	matches, err := filepath.Glob(path + ".stead-backup-*")
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("unexpected backup for no-op: %#v", matches)
	}
}

func TestRunApplyDoesNotWriteWhenValidationFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stead.conf")
	err := Run(Options{
		User:            "ed",
		DisablePassword: true,
		Apply:           true,
		ConfirmKeyLogin: true,
		DropInPath:      path,
		Out:             &bytes.Buffer{},
		Validate: func(path string) error {
			return errors.New("bad config")
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("target was written despite validation failure: %v", statErr)
	}
}

func okValidator(path string) error {
	return nil
}

func captureValidator(dst *string) Validator {
	return func(path string) error {
		*dst = path
		return nil
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 22, 3, 4, 5, 0, time.UTC)
}
