package clientconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ed/stead/internal/config"
)

func TestManagedBlock(t *testing.T) {
	block := ManagedBlock("devmac", testHost())

	for _, want := range []string{
		"# BEGIN stead devmac",
		"Host devmac",
		"HostName devmac.tailnet.example",
		"User ed",
		"Port 22",
		"IdentityFile ~/.ssh/stead_ed25519",
		"AddKeysToAgent yes",
		"IdentitiesOnly yes",
		"# END stead devmac",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("block missing %q:\n%s", want, block)
		}
	}
}

func TestPlanAddReplaceUnchanged(t *testing.T) {
	host := testHost()
	block := ManagedBlock("devmac", host)

	add, err := Plan([]byte("Host other\n    HostName other.example\n"), "/tmp/config", "devmac", host)
	if err != nil {
		t.Fatalf("Plan add returned error: %v", err)
	}
	if add.State != "add" {
		t.Fatalf("add state = %q", add.State)
	}

	unchanged, err := Plan([]byte(block), "/tmp/config", "devmac", host)
	if err != nil {
		t.Fatalf("Plan unchanged returned error: %v", err)
	}
	if unchanged.State != "unchanged" {
		t.Fatalf("unchanged state = %q", unchanged.State)
	}

	replacedBlock := strings.Replace(block, "User ed", "User other", 1)
	replace, err := Plan([]byte(replacedBlock), "/tmp/config", "devmac", host)
	if err != nil {
		t.Fatalf("Plan replace returned error: %v", err)
	}
	if replace.State != "replace" {
		t.Fatalf("replace state = %q", replace.State)
	}
}

func TestPlanMalformedManagedBlock(t *testing.T) {
	_, err := Plan([]byte("# BEGIN stead devmac\nHost devmac\n"), "/tmp/config", "devmac", testHost())
	if err == nil {
		t.Fatal("expected malformed marker error")
	}
	if !strings.Contains(err.Error(), "malformed managed block") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteDryRunDoesNotModifySSHConfig(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")
	original := "Host other\n    HostName other.example\n"
	if err := os.WriteFile(sshConfig, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg := &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": testHost(),
		},
	}

	var buf bytes.Buffer
	if err := WriteDryRun(&buf, cfg, "/tmp/stead-config.toml", sshConfig, ""); err != nil {
		t.Fatalf("WriteDryRun returned error: %v", err)
	}

	data, err := os.ReadFile(sshConfig)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != original {
		t.Fatalf("dry-run modified file:\n%s", string(data))
	}

	out := buf.String()
	for _, want := range []string{
		"Mode: dry-run",
		"Action: would add managed SSH config block",
		"# BEGIN stead devmac",
		"No files were modified.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestWriteDryRunReportsIncludeDirective(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")
	if err := os.WriteFile(sshConfig, []byte("Include ~/.ssh/config.d/*\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg := &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": testHost(),
		},
	}

	var buf bytes.Buffer
	if err := WriteDryRun(&buf, cfg, "/tmp/stead-config.toml", sshConfig, "devmac"); err != nil {
		t.Fatalf("WriteDryRun returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Note: Include directive present; included files are not expanded") {
		t.Fatalf("output missing Include note:\n%s", buf.String())
	}
}

func TestWriteApplyCreatesSSHConfig(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, ".ssh", "config")
	cfg := testConfig()

	var buf bytes.Buffer
	if err := WriteApply(&buf, cfg, "/tmp/stead-config.toml", sshConfig, "devmac"); err != nil {
		t.Fatalf("WriteApply returned error: %v", err)
	}

	data, err := os.ReadFile(sshConfig)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != ManagedBlock("devmac", testHost()) {
		t.Fatalf("unexpected config content:\n%s", string(data))
	}
	assertMode(t, filepath.Dir(sshConfig), 0o700)
	assertMode(t, sshConfig, 0o600)
	if backups := backupFiles(t, sshConfig); len(backups) != 0 {
		t.Fatalf("unexpected backups for new config: %#v", backups)
	}
	if !strings.Contains(buf.String(), "Action: added managed SSH config block") {
		t.Fatalf("output missing add action:\n%s", buf.String())
	}
}

func TestWriteApplyAppendsAndBacksUpExistingSSHConfig(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")
	original := "Host other\n    HostName other.example\n"
	if err := os.WriteFile(sshConfig, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteApply(&buf, testConfig(), "/tmp/stead-config.toml", sshConfig, "devmac"); err != nil {
		t.Fatalf("WriteApply returned error: %v", err)
	}

	data, err := os.ReadFile(sshConfig)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.HasPrefix(string(data), original+"\n") {
		t.Fatalf("existing content not preserved:\n%s", string(data))
	}
	if !strings.Contains(string(data), ManagedBlock("devmac", testHost())) {
		t.Fatalf("managed block missing:\n%s", string(data))
	}
	assertMode(t, sshConfig, 0o644)

	backups := backupFiles(t, sshConfig)
	if len(backups) != 1 {
		t.Fatalf("backup count = %d, want 1: %#v", len(backups), backups)
	}
	backup, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatalf("ReadFile backup returned error: %v", err)
	}
	if string(backup) != original {
		t.Fatalf("backup content = %q", string(backup))
	}
	if !strings.Contains(buf.String(), "Backup: "+backups[0]) {
		t.Fatalf("output missing backup path:\n%s", buf.String())
	}
}

func TestWriteApplyReplacesManagedBlock(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")
	old := strings.Replace(ManagedBlock("devmac", testHost()), "User ed", "User old", 1)
	original := "Host other\n    HostName other.example\n\n" + old + "Host after\n    HostName after.example\n"
	if err := os.WriteFile(sshConfig, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteApply(&buf, testConfig(), "/tmp/stead-config.toml", sshConfig, "devmac"); err != nil {
		t.Fatalf("WriteApply returned error: %v", err)
	}

	data, err := os.ReadFile(sshConfig)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "User old") {
		t.Fatalf("old managed block was not replaced:\n%s", got)
	}
	if !strings.Contains(got, ManagedBlock("devmac", testHost())) {
		t.Fatalf("new managed block missing:\n%s", got)
	}
	if !strings.Contains(got, "Host other") || !strings.Contains(got, "Host after") {
		t.Fatalf("unrelated content not preserved:\n%s", got)
	}
	if !strings.Contains(buf.String(), "Action: replaced existing managed SSH config block") {
		t.Fatalf("output missing replace action:\n%s", buf.String())
	}
}

func TestWriteApplyUnchangedDoesNotWriteBackup(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")
	block := ManagedBlock("devmac", testHost())
	if err := os.WriteFile(sshConfig, []byte(block), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteApply(&buf, testConfig(), "/tmp/stead-config.toml", sshConfig, "devmac"); err != nil {
		t.Fatalf("WriteApply returned error: %v", err)
	}
	if backups := backupFiles(t, sshConfig); len(backups) != 0 {
		t.Fatalf("unexpected backups: %#v", backups)
	}
	if !strings.Contains(buf.String(), "Action: no changes needed") {
		t.Fatalf("output missing unchanged action:\n%s", buf.String())
	}
}

func TestWriteApplyMalformedBlockDoesNotModifySSHConfig(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")
	original := "# BEGIN stead devmac\nHost devmac\n"
	if err := os.WriteFile(sshConfig, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	err := WriteApply(&bytes.Buffer{}, testConfig(), "/tmp/stead-config.toml", sshConfig, "devmac")
	if err == nil {
		t.Fatal("expected malformed block error")
	}
	data, readErr := os.ReadFile(sshConfig)
	if readErr != nil {
		t.Fatalf("ReadFile returned error: %v", readErr)
	}
	if string(data) != original {
		t.Fatalf("malformed apply modified config:\n%s", string(data))
	}
	if backups := backupFiles(t, sshConfig); len(backups) != 0 {
		t.Fatalf("unexpected backups: %#v", backups)
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Defaults: config.Defaults{Alias: "devmac"},
		Hosts: map[string]*config.Host{
			"devmac": testHost(),
		},
	}
}

func testHost() *config.Host {
	return &config.Host{
		Hostname:     "devmac.tailnet.example",
		User:         "ed",
		Port:         22,
		IdentityFile: "~/.ssh/stead_ed25519",
	}
}

func backupFiles(t *testing.T, sshConfig string) []string {
	t.Helper()
	matches, err := filepath.Glob(sshConfig + ".stead-backup-*")
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	return matches
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
