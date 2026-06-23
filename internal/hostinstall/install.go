package hostinstall

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ed/stead/internal/ui"
)

const (
	beginMarker    = "# >>> stead managed block: tmux auto-attach"
	endMarker      = "# <<< stead managed block: tmux auto-attach"
	defaultSession = "main"
)

type Options struct {
	DryRun          bool
	Apply           bool
	Uninstall       bool
	Confirm         bool
	Force           bool
	ShellConfigPath string
	TmuxSession     string
	Out             io.Writer
	Now             func() time.Time
}

type Change struct {
	State      string
	Action     string
	NewContent []byte
	Backup     string
}

func Run(opts Options) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Uninstall {
		return uninstall(opts)
	}
	if opts.DryRun == opts.Apply {
		return fmt.Errorf("host install requires exactly one of --dry-run or --apply")
	}
	session := valueOrDefault(opts.TmuxSession, defaultSession)
	if strings.TrimSpace(session) == "" {
		return fmt.Errorf("--tmux-session cannot be empty")
	}
	path, err := shellConfigPath(opts.ShellConfigPath)
	if err != nil {
		return err
	}
	existing, existed, mode, err := readExisting(path)
	if err != nil {
		return err
	}
	change, err := PlanInstall(existing, session, opts.Force)
	if err != nil {
		return err
	}

	printInstallHeader(opts.Out, path, session, opts)
	printInstallPlan(opts.Out, change, opts.Force, opts.DryRun)
	if opts.DryRun || change.State == "unchanged" || change.State == "custom" {
		if opts.DryRun {
			fmt.Fprintln(opts.Out)
			fmt.Fprintln(opts.Out, "No files were modified.")
		}
		return nil
	}

	if existed {
		change.Backup = backupPath(path, now(opts))
		if err := os.WriteFile(change.Backup, existing, mode); err != nil {
			return err
		}
	}
	if err := writeAtomic(path, change.NewContent, mode); err != nil {
		return err
	}
	printApplied(opts.Out, change)
	return nil
}

func uninstall(opts Options) error {
	if opts.DryRun == opts.Apply {
		return fmt.Errorf("host uninstall requires exactly one of --dry-run or --apply")
	}
	if opts.Apply && !opts.Confirm {
		return fmt.Errorf("host uninstall --apply requires --confirm")
	}
	path, err := shellConfigPath(opts.ShellConfigPath)
	if err != nil {
		return err
	}
	existing, existed, mode, err := readExisting(path)
	if err != nil {
		return err
	}
	change, err := PlanUninstall(existing)
	if err != nil {
		return err
	}

	printUninstallHeader(opts.Out, path, opts)
	printUninstallPlan(opts.Out, change, opts.DryRun)
	if opts.DryRun || change.State == "absent" {
		if opts.DryRun {
			fmt.Fprintln(opts.Out)
			fmt.Fprintln(opts.Out, "No files were modified.")
		}
		return nil
	}
	if !existed {
		return fmt.Errorf("%s does not exist", path)
	}
	change.Backup = backupPath(path, now(opts))
	if err := os.WriteFile(change.Backup, existing, mode); err != nil {
		return err
	}
	if err := writeAtomic(path, change.NewContent, mode); err != nil {
		return err
	}
	printApplied(opts.Out, change)
	return nil
}

func PlanInstall(existing []byte, session string, force bool) (Change, error) {
	block := ManagedBlock(session)
	current := string(existing)
	begin, end, found, err := managedRange(current)
	if err != nil {
		return Change{}, err
	}
	if found {
		if current[begin:end] == block {
			return Change{State: "unchanged", Action: "no changes needed", NewContent: existing}, nil
		}
		return Change{
			State:      "replace",
			Action:     "would replace managed tmux auto-attach block",
			NewContent: []byte(current[:begin] + block + current[end:]),
		}, nil
	}
	if hasCustomAutoAttach(current) && !force {
		return Change{State: "custom", Action: "custom tmux auto-attach present; use --force to add managed block", NewContent: existing}, nil
	}
	return Change{
		State:      "add",
		Action:     "would add managed tmux auto-attach block",
		NewContent: []byte(appendBlock(current, block)),
	}, nil
}

func PlanUninstall(existing []byte) (Change, error) {
	current := string(existing)
	begin, end, found, err := managedRange(current)
	if err != nil {
		return Change{}, err
	}
	if !found {
		return Change{State: "absent", Action: "no changes needed", NewContent: existing}, nil
	}
	return Change{
		State:      "remove",
		Action:     "would remove managed tmux auto-attach block",
		NewContent: []byte(current[:begin] + current[end:]),
	}, nil
}

func ManagedBlock(session string) string {
	var b strings.Builder
	b.WriteString(beginMarker + "\n")
	b.WriteString(`if command -v tmux >/dev/null 2>&1 && [ -n "$PS1" ] && [ -n "$SSH_CONNECTION" ] && [ -z "$TMUX" ]; then` + "\n")
	fmt.Fprintf(&b, "    exec tmux new-session -A -s %s\n", shellQuote(session))
	b.WriteString("fi\n")
	b.WriteString(endMarker + "\n")
	return b.String()
}

func printInstallHeader(out io.Writer, path, session string, opts Options) {
	ui.PrintTitle(out, "Stead host install")
	fmt.Fprintln(out)
	if opts.DryRun {
		ui.PrintKV(out, "Mode", "dry-run (no files changed)")
	} else {
		ui.PrintKV(out, "Mode", "apply")
	}
	ui.PrintKV(out, "Target", path)
	ui.PrintKV(out, "tmux session", session)
	ui.PrintKV(out, "Scope", "shell startup only; SSH auth is unchanged")
	if opts.Force {
		ui.PrintKV(out, "Force", "enabled")
	}
	fmt.Fprintln(out)
}

func printInstallPlan(out io.Writer, change Change, force bool, dryRun bool) {
	ui.PrintSection(out, "Changes")
	switch change.State {
	case "replace":
		ui.PrintKV(out, "Action", actionPhrase(dryRun, "replace managed tmux auto-attach block"))
	case "add":
		ui.PrintKV(out, "Action", actionPhrase(dryRun, "add managed tmux auto-attach block"))
	case "custom":
		ui.PrintKV(out, "Action", change.Action)
	case "unchanged":
		ui.PrintKV(out, "Action", "no changes needed")
	default:
		ui.PrintKV(out, "Action", change.Action)
	}
	if change.State == "custom" && !force {
		fmt.Fprintln(out)
		ui.PrintSection(out, "Next")
		ui.PrintStep(out, 1, "Leave the existing custom tmux auto-attach in place")
		ui.PrintStep(out, 2, "Use --force only if you want stead to add its own managed block")
	}
}

func printUninstallHeader(out io.Writer, path string, opts Options) {
	ui.PrintTitle(out, "Stead host uninstall")
	fmt.Fprintln(out)
	if opts.DryRun {
		ui.PrintKV(out, "Mode", "dry-run (no files changed)")
	} else {
		ui.PrintKV(out, "Mode", "apply")
	}
	ui.PrintKV(out, "Target", path)
	fmt.Fprintln(out)
}

func printUninstallPlan(out io.Writer, change Change, dryRun bool) {
	ui.PrintSection(out, "Changes")
	switch change.State {
	case "remove":
		ui.PrintKV(out, "Action", actionPhrase(dryRun, "remove managed tmux auto-attach block"))
	case "absent":
		ui.PrintKV(out, "Action", "no changes needed")
	default:
		ui.PrintKV(out, "Action", change.Action)
	}
}

func printApplied(out io.Writer, change Change) {
	fmt.Fprintln(out)
	ui.PrintSection(out, "Applied")
	switch change.State {
	case "add":
		ui.PrintKV(out, "Action", "added managed tmux auto-attach block")
	case "replace":
		ui.PrintKV(out, "Action", "replaced managed tmux auto-attach block")
	case "remove":
		ui.PrintKV(out, "Action", "removed managed tmux auto-attach block")
	default:
		ui.PrintKV(out, "Action", change.Action)
	}
	if change.Backup != "" {
		ui.PrintKV(out, "Backup", change.Backup)
	}
}

func shellConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".zshrc"), nil
}

func readExisting(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, 0o644, nil
		}
		return nil, false, 0, err
	}
	if info.IsDir() {
		return nil, false, 0, fmt.Errorf("%s is a directory", path)
	}
	data, err := os.ReadFile(path)
	return data, true, info.Mode().Perm(), err
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".stead-zshrc-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	remove = false
	return nil
}

func managedRange(current string) (int, int, bool, error) {
	begin := strings.Index(current, beginMarker)
	end := strings.Index(current, endMarker)
	if begin == -1 && end == -1 {
		return 0, 0, false, nil
	}
	if begin == -1 || end == -1 || end < begin {
		return 0, 0, false, fmt.Errorf("malformed managed tmux auto-attach block")
	}
	end += len(endMarker)
	if end < len(current) && current[end] == '\r' {
		end++
	}
	if end < len(current) && current[end] == '\n' {
		end++
	}
	return begin, end, true, nil
}

func appendBlock(current, block string) string {
	if current == "" {
		return block
	}
	var b strings.Builder
	b.WriteString(current)
	if !strings.HasSuffix(current, "\n") {
		b.WriteString("\n")
	}
	if !strings.HasSuffix(b.String(), "\n\n") {
		b.WriteString("\n")
	}
	b.WriteString(block)
	return b.String()
}

func hasCustomAutoAttach(current string) bool {
	return strings.Contains(current, "SSH_CONNECTION") && strings.Contains(current, "tmux new-session")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func valueOrDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func now(opts Options) time.Time {
	if opts.Now != nil {
		return opts.Now()
	}
	return time.Now().UTC()
}

func backupPath(path string, t time.Time) string {
	return path + ".stead-backup-" + t.UTC().Format("20060102T150405.000000000Z")
}

func actionPhrase(dryRun bool, text string) string {
	if dryRun {
		return "would " + text
	}
	return "will " + text
}
