package hostops

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/ed/stead/internal/hostharden"
	"github.com/ed/stead/internal/ui"
)

type ValidateOptions struct {
	Out        io.Writer
	Runner     Runner
	DropInPath string
}

type ReloadOptions struct {
	DryRun  bool
	Apply   bool
	Confirm bool
	Out     io.Writer
	Runner  Runner
}

type Runner func(name string, args ...string) ([]byte, error)

func Validate(opts ValidateOptions) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	path := opts.DropInPath
	if path == "" {
		path = hostharden.DefaultDropInPath
	}
	runner := opts.Runner
	if runner == nil {
		runner = commandRunner
	}

	sshdPath, sshdState := lookPath("sshd")
	sshdConfig := fileCheck("/etc/ssh/sshd_config")
	steadConfig := fileCheck(path)
	validation := validateSSHD(runner)

	ui.PrintTitle(out, "Stead host validate")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Mode", "read-only")
	ui.PrintKV(out, "Target", path)
	fmt.Fprintln(out)

	ui.PrintSection(out, "Files")
	ui.PrintKV(out, "sshd", sshdState)
	ui.PrintKV(out, "sshd_config", sshdConfig)
	ui.PrintKV(out, "stead.conf", steadConfig)
	fmt.Fprintln(out)

	ui.PrintSection(out, "Validation")
	if sshdPath == "" {
		ui.PrintKV(out, "sshd -t", ui.StateDetail(out, "missing", "sshd not found"))
	} else {
		ui.PrintKV(out, "sshd -t", validation)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "No files were modified.")
	return nil
}

func Reload(opts ReloadOptions) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	if opts.DryRun == opts.Apply {
		return fmt.Errorf("host reload requires exactly one of --dry-run or --apply")
	}
	if opts.Apply && !opts.Confirm {
		return fmt.Errorf("host reload --apply requires --confirm")
	}
	runner := opts.Runner
	if runner == nil {
		runner = commandRunner
	}
	if opts.Apply {
		return applyReload(out, runner)
	}

	ui.PrintTitle(out, "Stead host reload plan")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Mode", "dry-run (no services changed)")
	ui.PrintKV(out, "Target service", "system/com.openssh.sshd")
	ui.PrintKV(out, "Reload", "not performed by stead")
	fmt.Fprintln(out)

	ui.PrintSection(out, "Preflight")
	ui.PrintKV(out, "Remote Login", "check with stead host status")
	ui.PrintKV(out, "Config validation", "run stead host validate first")
	ui.PrintKV(out, "Active SSH session", "keep one local host session open")
	fmt.Fprintln(out)

	ui.PrintSection(out, "Manual commands")
	ui.PrintStep(out, 1, "sudo /usr/sbin/sshd -t")
	ui.PrintStep(out, 2, "sudo launchctl kickstart -k system/com.openssh.sshd")
	ui.PrintStep(out, 3, "ssh <alias> from the client")
	fmt.Fprintln(out)

	ui.PrintSection(out, "Rollback")
	ui.PrintStep(out, 1, "sudo rm /etc/ssh/sshd_config.d/stead.conf")
	ui.PrintStep(out, 2, "sudo launchctl kickstart -k system/com.openssh.sshd")
	ui.PrintStep(out, 3, "restore any .stead-backup-* file if one exists")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "No services were reloaded.")
	return nil
}

func applyReload(out io.Writer, runner Runner) error {
	if _, err := runner("/usr/sbin/sshd", "-t"); err != nil {
		return fmt.Errorf("sshd validation failed: %w", err)
	}
	if _, err := runner("launchctl", "kickstart", "-k", "system/com.openssh.sshd"); err != nil {
		return fmt.Errorf("launchctl reload failed: %w", err)
	}

	ui.PrintTitle(out, "Stead host reload")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Mode", "apply")
	ui.PrintKV(out, "Target service", "system/com.openssh.sshd")
	fmt.Fprintln(out)

	ui.PrintSection(out, "Changes")
	ui.PrintKV(out, "sshd -t", ui.State(out, "ok"))
	ui.PrintKV(out, "launchctl kickstart", ui.State(out, "ok"))
	fmt.Fprintln(out)

	ui.PrintSection(out, "Next")
	ui.PrintStep(out, 1, "Test a new SSH login from the client")
	ui.PrintStep(out, 2, "Keep the current local session open until the test passes")
	ui.PrintStep(out, 3, "If needed, remove /etc/ssh/sshd_config.d/stead.conf and reload again")
	return nil
}

func validateSSHD(runner Runner) string {
	out, err := runner("sshd", "-t")
	if err == nil {
		return "ok"
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	state := "failed"
	if strings.Contains(strings.ToLower(msg), "no hostkeys available") {
		state = "unknown"
		msg = "sshd -t needs root-readable host keys on this macOS; no sudo attempted"
	}
	return state + " (" + msg + ")"
}

func lookPath(name string) (string, string) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", "missing"
	}
	return path, "ok (" + path + ")"
}

func fileCheck(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "missing"
		}
		return "unknown (" + err.Error() + ")"
	}
	if info.IsDir() {
		return "ok (directory)"
	}
	return fmt.Sprintf("ok (%d bytes)", info.Size())
}

func commandRunner(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}
