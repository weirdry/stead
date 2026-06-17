package hostauth

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Alias              string
	PublicKey          string
	DryRun             bool
	AuthorizedKeysPath string
	Out                io.Writer
}

type Plan struct {
	State   string
	Line    string
	Content []byte
}

func Run(opts Options) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	path := opts.AuthorizedKeysPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".ssh", "authorized_keys")
	}

	line, err := normalizePublicKey(opts.PublicKey, opts.Alias)
	if err != nil {
		return err
	}

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	plan := PlanContent(existing, line)

	fmt.Fprintln(opts.Out, "Stead host authorize")
	fmt.Fprintln(opts.Out)
	fmt.Fprintf(opts.Out, "AuthorizedKeys: %s\n", path)
	if opts.Alias != "" {
		fmt.Fprintf(opts.Out, "Alias: %s\n", opts.Alias)
	}
	if opts.DryRun {
		fmt.Fprintln(opts.Out, "Mode: dry-run")
	} else {
		fmt.Fprintln(opts.Out, "Mode: apply")
	}
	fmt.Fprintln(opts.Out)

	if opts.DryRun {
		switch plan.State {
		case "present":
			fmt.Fprintln(opts.Out, "Action: no changes needed")
		case "add":
			fmt.Fprintln(opts.Out, "Action: would add public key")
		}
		fmt.Fprintln(opts.Out, "No files were modified.")
		return nil
	}

	if plan.State == "present" {
		fmt.Fprintln(opts.Out, "Action: no changes needed")
		fmt.Fprintln(opts.Out, "No files were modified.")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(plan.Line + "\n"); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}

	fmt.Fprintln(opts.Out, "Action: added public key")
	return nil
}

func RunUnauthorize(opts Options) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	path := opts.AuthorizedKeysPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".ssh", "authorized_keys")
	}

	line, err := normalizePublicKey(opts.PublicKey, opts.Alias)
	if err != nil {
		return err
	}

	existing, existed, mode, err := readExisting(path)
	if err != nil {
		return err
	}
	plan := PlanRemoval(existing, line)

	fmt.Fprintln(opts.Out, "Stead host unauthorize")
	fmt.Fprintln(opts.Out)
	fmt.Fprintf(opts.Out, "AuthorizedKeys: %s\n", path)
	if opts.Alias != "" {
		fmt.Fprintf(opts.Out, "Alias: %s\n", opts.Alias)
	}
	if opts.DryRun {
		fmt.Fprintln(opts.Out, "Mode: dry-run")
	} else {
		fmt.Fprintln(opts.Out, "Mode: apply")
	}
	fmt.Fprintln(opts.Out)

	if opts.DryRun {
		switch plan.State {
		case "remove":
			fmt.Fprintln(opts.Out, "Action: would remove public key")
		case "absent":
			fmt.Fprintln(opts.Out, "Action: no changes needed")
		}
		fmt.Fprintln(opts.Out, "No files were modified.")
		return nil
	}

	if plan.State == "absent" {
		fmt.Fprintln(opts.Out, "Action: no changes needed")
		fmt.Fprintln(opts.Out, "No files were modified.")
		return nil
	}
	if !existed {
		return fmt.Errorf("authorized_keys does not exist")
	}
	if err := os.WriteFile(path, plan.Content, mode); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}

	fmt.Fprintln(opts.Out, "Action: removed public key")
	return nil
}

func PlanContent(existing []byte, line string) Plan {
	target := keyIdentity(line)
	for _, existingLine := range strings.Split(string(existing), "\n") {
		if keyIdentity(existingLine) == target {
			return Plan{State: "present", Line: line}
		}
	}
	return Plan{State: "add", Line: line}
}

func PlanRemoval(existing []byte, line string) Plan {
	target := keyIdentity(line)
	lines := strings.Split(string(existing), "\n")
	removed := false
	out := make([]string, 0, len(lines))
	for _, existingLine := range lines {
		if keyIdentity(existingLine) == target {
			removed = true
			continue
		}
		out = append(out, existingLine)
	}
	if !removed {
		return Plan{State: "absent", Line: line, Content: existing}
	}
	return Plan{State: "remove", Line: line, Content: []byte(strings.Join(out, "\n"))}
}

func readExisting(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, 0o600, nil
		}
		return nil, false, 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, 0, err
	}
	return data, true, info.Mode().Perm(), nil
}

func keyIdentity(line string) string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 2 || !isPublicKeyType(fields[0]) {
		return ""
	}
	return fields[0] + " " + fields[1]
}

func normalizePublicKey(value, alias string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("--public-key is required")
	}
	if strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		return "", fmt.Errorf("public key must be a single line")
	}
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return "", fmt.Errorf("invalid public key")
	}
	if !isPublicKeyType(fields[0]) {
		return "", fmt.Errorf("unsupported or private-looking key type %q", fields[0])
	}
	if alias != "" && len(fields) == 2 {
		fields = append(fields, "stead "+alias)
	}
	return strings.Join(fields, " "), nil
}

func isPublicKeyType(value string) bool {
	switch value {
	case "ssh-ed25519", "ssh-rsa", "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521", "sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com":
		return true
	default:
		return false
	}
}
