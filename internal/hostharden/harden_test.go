package hostharden

import (
	"bytes"
	"strings"
	"testing"
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

func TestRunRequiresDryRun(t *testing.T) {
	err := Run(Options{User: "ed", Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected dry-run error")
	}
	if !strings.Contains(err.Error(), "requires --dry-run") {
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
