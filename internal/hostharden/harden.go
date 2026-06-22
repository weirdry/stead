package hostharden

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/ed/stead/internal/ui"
)

const DefaultDropInPath = "/etc/ssh/sshd_config.d/stead.conf"

type Options struct {
	User            string
	DisablePassword bool
	DryRun          bool
	Apply           bool
	ConfirmKeyLogin bool
	Force           bool
	DropInPath      string
	Out             io.Writer
	Validate        Validator
	Now             func() time.Time
}

type Validator func(path string) error

func Run(opts Options) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.DryRun == opts.Apply {
		return fmt.Errorf("host harden requires exactly one of --dry-run or --apply")
	}
	if opts.DisablePassword && opts.Apply && !opts.ConfirmKeyLogin && !opts.Force {
		return fmt.Errorf("--disable-password with --apply requires --confirm-key-login or --force")
	}

	loginUser, userSource, err := loginUser(opts.User)
	if err != nil {
		return err
	}
	if err := validateUser(loginUser); err != nil {
		return err
	}

	path := opts.DropInPath
	if path == "" {
		path = DefaultDropInPath
	}

	config := Config(loginUser, opts.DisablePassword)
	if opts.DryRun {
		printPlan(opts.Out, path, loginUser, userSource, opts.DisablePassword, config)
		return nil
	}
	return apply(opts, path, loginUser, userSource, config)
}

type applyResult struct {
	Action string
	Backup string
}

func apply(opts Options, path, loginUser, userSource, config string) error {
	validator := opts.Validate
	if validator == nil {
		validator = validateWithSSHD
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	if err := validateCandidate(filepath.Dir(path), []byte(config), validator); err != nil {
		return err
	}

	result, err := writeDropIn(path, []byte(config), now)
	if err != nil {
		return err
	}

	printApply(opts.Out, path, loginUser, userSource, opts.DisablePassword, result)
	return nil
}

func Config(loginUser string, disablePassword bool) string {
	var b strings.Builder
	b.WriteString("# stead managed OpenSSH host hardening\n")
	b.WriteString("PubkeyAuthentication yes\n")
	if disablePassword {
		b.WriteString("PasswordAuthentication no\n")
		b.WriteString("KbdInteractiveAuthentication no\n")
	}
	b.WriteString("PermitRootLogin no\n")
	fmt.Fprintf(&b, "AllowUsers %s\n", loginUser)
	return b.String()
}

func printPlan(out io.Writer, path, loginUser, userSource string, disablePassword bool, config string) {
	ui.PrintTitle(out, "Stead host harden")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Mode", "dry-run (no files changed)")
	ui.PrintKV(out, "Target", path)
	ui.PrintKV(out, "Login user", loginUser+" ("+userSource+")")
	if disablePassword {
		ui.PrintKV(out, "Password auth", ui.StateDetail(out, "ok", "would disable password-style SSH login"))
	} else {
		ui.PrintKV(out, "Password auth", ui.StateDetail(out, "warn", "unchanged; pass --disable-password to include it"))
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Preflight")
	ui.PrintKV(out, "sshd", checkLookPath(out, "sshd"))
	ui.PrintKV(out, "Drop-in directory", checkPath(out, filepath.Dir(path)))
	ui.PrintKV(out, "Existing stead.conf", checkPath(out, path))
	ui.PrintKV(out, "authorized_keys", checkAuthorizedKeys(out, loginUser))
	fmt.Fprintln(out)

	ui.PrintSection(out, "Proposed drop-in")
	for _, line := range strings.Split(strings.TrimRight(config, "\n"), "\n") {
		ui.PrintListItem(out, line)
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Safety")
	ui.PrintStep(out, 1, "Authorize and verify key login before applying host hardening")
	ui.PrintStep(out, 2, "Validate sshd configuration before reload")
	ui.PrintStep(out, 3, "Keep an existing local session open during any future apply")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "No files were modified.")
}

func printApply(out io.Writer, path, loginUser, userSource string, disablePassword bool, result applyResult) {
	ui.PrintTitle(out, "Stead host harden")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Mode", "apply")
	ui.PrintKV(out, "Target", path)
	ui.PrintKV(out, "Login user", loginUser+" ("+userSource+")")
	if disablePassword {
		ui.PrintKV(out, "Password auth", ui.StateDetail(out, "ok", "disabled in managed drop-in"))
	} else {
		ui.PrintKV(out, "Password auth", ui.StateDetail(out, "warn", "unchanged"))
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Changes")
	ui.PrintKV(out, "Validation", ui.StateDetail(out, "ok", "candidate accepted by sshd -t"))
	ui.PrintKV(out, "Action", result.Action)
	if result.Backup != "" {
		ui.PrintKV(out, "Backup", result.Backup)
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Next")
	ui.PrintStep(out, 1, "Keep the current local session open")
	ui.PrintStep(out, 2, "Test a new SSH login from the client")
	ui.PrintStep(out, 3, "If needed, restore the backup or remove "+path)
	fmt.Fprintln(out)
	ui.PrintKV(out, "Reload", "not performed by stead")
}

func validateCandidate(dir string, content []byte, validator Validator) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".stead-sshd-*.conf")
	if err != nil {
		return err
	}
	path := file.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o644); err != nil {
		return err
	}
	if err := validator(path); err != nil {
		return fmt.Errorf("candidate sshd validation failed: %w", err)
	}
	return nil
}

func writeDropIn(path string, content []byte, now func() time.Time) (applyResult, error) {
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, content) {
			return applyResult{Action: "no changes needed"}, nil
		}
		backup := backupPath(path, now())
		if err := os.WriteFile(backup, existing, 0o644); err != nil {
			return applyResult{}, err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return applyResult{}, err
		}
		return applyResult{Action: "replaced managed drop-in", Backup: backup}, nil
	} else if !os.IsNotExist(err) {
		return applyResult{}, err
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return applyResult{}, err
	}
	return applyResult{Action: "created managed drop-in"}, nil
}

func backupPath(path string, t time.Time) string {
	return path + ".stead-backup-" + t.UTC().Format("20060102T150405.000000000Z")
}

func validateWithSSHD(path string) error {
	sshd, err := exec.LookPath("sshd")
	if err != nil {
		return err
	}
	cmd := exec.Command(sshd, "-t", "-f", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func loginUser(explicit string) (string, string, error) {
	if explicit != "" {
		return explicit, "explicit", nil
	}
	u, err := user.Current()
	if err != nil {
		return "", "", err
	}
	if u == nil || u.Username == "" {
		return "", "", fmt.Errorf("unable to detect current user; pass --user")
	}
	return u.Username, "current user", nil
}

func validateUser(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("--user cannot be empty")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("--user must be a single local account name")
	}
	return nil
}

func checkLookPath(out io.Writer, name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ui.State(out, "missing")
	}
	return ui.StateDetail(out, "ok", path)
}

func checkPath(out io.Writer, path string) string {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ui.State(out, "missing")
		}
		return ui.StateDetail(out, "unknown", err.Error())
	}
	if info.IsDir() {
		return ui.StateDetail(out, "ok", "directory")
	}
	return ui.StateDetail(out, "ok", fmt.Sprintf("%d bytes", info.Size()))
}

func checkAuthorizedKeys(out io.Writer, loginUser string) string {
	u, err := user.Lookup(loginUser)
	if err != nil || u == nil || u.HomeDir == "" {
		return ui.StateDetail(out, "unknown", "unable to resolve user home")
	}
	return checkPath(out, filepath.Join(u.HomeDir, ".ssh", "authorized_keys"))
}
