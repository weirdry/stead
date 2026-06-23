package hostinstall

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlanInstallAddsManagedBlock(t *testing.T) {
	change, err := PlanInstall([]byte("export EDITOR=vim\n"), "main", false)
	if err != nil {
		t.Fatalf("PlanInstall returned error: %v", err)
	}
	if change.State != "add" {
		t.Fatalf("state = %q, want add", change.State)
	}
	got := string(change.NewContent)
	if !strings.Contains(got, "export EDITOR=vim") || !strings.Contains(got, beginMarker) {
		t.Fatalf("new content missing original or managed block:\n%s", got)
	}
}

func TestPlanInstallReplacesManagedBlock(t *testing.T) {
	old := ManagedBlock("old")
	change, err := PlanInstall([]byte("before\n"+old+"after\n"), "main", false)
	if err != nil {
		t.Fatalf("PlanInstall returned error: %v", err)
	}
	if change.State != "replace" {
		t.Fatalf("state = %q, want replace", change.State)
	}
	got := string(change.NewContent)
	if strings.Contains(got, "-s 'old'") {
		t.Fatalf("old session still present:\n%s", got)
	}
	if !strings.Contains(got, "-s 'main'") || !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Fatalf("new content malformed:\n%s", got)
	}
}

func TestPlanInstallSkipsCustomAutoAttachWithoutForce(t *testing.T) {
	custom := `if [ -n "$SSH_CONNECTION" ]; then
    exec tmux new-session -A -s main
fi
`
	change, err := PlanInstall([]byte(custom), "main", false)
	if err != nil {
		t.Fatalf("PlanInstall returned error: %v", err)
	}
	if change.State != "custom" {
		t.Fatalf("state = %q, want custom", change.State)
	}
	if string(change.NewContent) != custom {
		t.Fatalf("custom content changed:\n%s", string(change.NewContent))
	}
}

func TestPlanInstallAddsManagedBlockWithForce(t *testing.T) {
	custom := `if [ -n "$SSH_CONNECTION" ]; then
    exec tmux new-session -A -s main
fi
`
	change, err := PlanInstall([]byte(custom), "main", true)
	if err != nil {
		t.Fatalf("PlanInstall returned error: %v", err)
	}
	if change.State != "add" {
		t.Fatalf("state = %q, want add", change.State)
	}
	if !strings.Contains(string(change.NewContent), beginMarker) {
		t.Fatalf("managed block missing:\n%s", string(change.NewContent))
	}
}

func TestPlanUninstallRemovesOnlyManagedBlock(t *testing.T) {
	original := "before\n" + ManagedBlock("main") + "after\n"
	change, err := PlanUninstall([]byte(original))
	if err != nil {
		t.Fatalf("PlanUninstall returned error: %v", err)
	}
	if change.State != "remove" {
		t.Fatalf("state = %q, want remove", change.State)
	}
	got := string(change.NewContent)
	if strings.Contains(got, beginMarker) {
		t.Fatalf("managed block still present:\n%s", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Fatalf("unrelated content missing:\n%s", got)
	}
}

func TestRunInstallDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := "export EDITOR=vim\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var buf bytes.Buffer
	if err := Run(Options{DryRun: true, ShellConfigPath: path, TmuxSession: "main", Out: &buf}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != original {
		t.Fatalf("dry-run modified file:\n%s", string(data))
	}
	if !strings.Contains(buf.String(), "would add managed tmux auto-attach block") {
		t.Fatalf("output missing add action:\n%s", buf.String())
	}
}

func TestRunInstallApplyWritesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := "export EDITOR=vim\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var buf bytes.Buffer
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	if err := Run(Options{Apply: true, ShellConfigPath: path, TmuxSession: "main", Out: &buf, Now: func() time.Time { return now }}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(data), original) || !strings.Contains(string(data), beginMarker) {
		t.Fatalf("managed block not written:\n%s", string(data))
	}
	backup := path + ".stead-backup-20260623T120000.000000000Z"
	backupData, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("ReadFile backup returned error: %v", err)
	}
	if string(backupData) != original {
		t.Fatalf("backup content = %q", string(backupData))
	}
}

func TestRunUninstallRequiresConfirm(t *testing.T) {
	err := Run(Options{Uninstall: true, Apply: true, ShellConfigPath: filepath.Join(t.TempDir(), ".zshrc"), Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected confirm error")
	}
}

func TestRunUninstallApplyRemovesManagedBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := "before\n" + ManagedBlock("main") + "after\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	var buf bytes.Buffer
	if err := Run(Options{Uninstall: true, Apply: true, Confirm: true, ShellConfigPath: path, Out: &buf}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	got := string(data)
	if strings.Contains(got, beginMarker) {
		t.Fatalf("managed block still present:\n%s", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Fatalf("unrelated content missing:\n%s", got)
	}
}
