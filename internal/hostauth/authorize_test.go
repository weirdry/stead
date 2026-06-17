package hostauth

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPublicKey = "ssh-ed25519 test-public-key"

func TestRunDryRunDoesNotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ssh", "authorized_keys")
	var buf bytes.Buffer

	if err := Run(Options{
		Alias:              "stead-devmac",
		PublicKey:          testPublicKey,
		AuthorizedKeysPath: path,
		DryRun:             true,
		Out:                &buf,
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote authorized_keys: %v", err)
	}
	for _, want := range []string{
		"Mode: dry-run",
		"Action: would add public key",
		"No files were modified.",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRunAddsPublicKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".ssh", "authorized_keys")
	var buf bytes.Buffer

	if err := Run(Options{
		Alias:              "stead-devmac",
		PublicKey:          testPublicKey,
		AuthorizedKeysPath: path,
		Out:                &buf,
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	wantLine := testPublicKey + " stead stead-devmac"
	if strings.TrimSpace(string(data)) != wantLine {
		t.Fatalf("authorized_keys = %q, want %q", strings.TrimSpace(string(data)), wantLine)
	}
	assertMode(t, filepath.Dir(path), 0o700)
	assertMode(t, path, 0o600)
	if !strings.Contains(buf.String(), "Action: added public key") {
		t.Fatalf("output missing add action:\n%s", buf.String())
	}
}

func TestRunDetectsExistingPublicKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ssh", "authorized_keys")
	line := testPublicKey + " stead stead-devmac"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := Run(Options{
		Alias:              "stead-devmac",
		PublicKey:          line,
		AuthorizedKeysPath: path,
		Out:                &buf,
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != line+"\n" {
		t.Fatalf("authorized_keys changed:\n%s", string(data))
	}
	if !strings.Contains(buf.String(), "Action: no changes needed") {
		t.Fatalf("output missing no-op action:\n%s", buf.String())
	}
}

func TestRunDetectsExistingPublicKeyWithDifferentComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ssh", "authorized_keys")
	existing := testPublicKey + " older-comment"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(existing+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := Run(Options{
		Alias:              "stead-devmac",
		PublicKey:          testPublicKey + " stead stead-devmac",
		AuthorizedKeysPath: path,
		Out:                &buf,
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != existing+"\n" {
		t.Fatalf("authorized_keys changed:\n%s", string(data))
	}
	if !strings.Contains(buf.String(), "Action: no changes needed") {
		t.Fatalf("output missing no-op action:\n%s", buf.String())
	}
}

func TestRunRejectsInvalidPublicKeys(t *testing.T) {
	tests := []string{
		"",
		"-----BEGIN OPENSSH PRIVATE KEY-----",
		"ssh-ed25519 part1\npart2",
		"not-a-key value",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			err := Run(Options{
				PublicKey:          input,
				AuthorizedKeysPath: filepath.Join(t.TempDir(), "authorized_keys"),
				Out:                &bytes.Buffer{},
			})
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat %s returned error: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}
