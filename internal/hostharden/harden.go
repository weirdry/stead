package hostharden

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ed/stead/internal/ui"
)

const DefaultDropInPath = "/etc/ssh/sshd_config.d/stead.conf"

type Options struct {
	User            string
	DisablePassword bool
	DryRun          bool
	DropInPath      string
	Out             io.Writer
}

func Run(opts Options) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if !opts.DryRun {
		return fmt.Errorf("host harden currently requires --dry-run")
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
	printPlan(opts.Out, path, loginUser, userSource, opts.DisablePassword, config)
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
