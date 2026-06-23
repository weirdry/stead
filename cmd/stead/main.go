package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ed/stead/internal/clientconfig"
	"github.com/ed/stead/internal/clientinit"
	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/connect"
	"github.com/ed/stead/internal/hostauth"
	"github.com/ed/stead/internal/hostharden"
	"github.com/ed/stead/internal/hostops"
	"github.com/ed/stead/internal/plan"
	"github.com/ed/stead/internal/setup"
	"github.com/ed/stead/internal/status"
	"github.com/ed/stead/internal/ui"
	"github.com/ed/stead/internal/verify"
	"github.com/ed/stead/internal/wake"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "stead: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	args = parseGlobalOptions(args)
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}

	switch args[0] {
	case "status":
		return status.Run(os.Stdout)
	case "setup":
		return runSetup(args[1:])
	case "verify":
		return runVerify(args[1:])
	case "connect":
		return runConnect(args[1:])
	case "wake":
		return runWake(args[1:])
	case "host":
		return runHost(args[1:])
	case "client":
		return runClient(args[1:])
	case "config":
		return runConfig(args[1:])
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func parseGlobalOptions(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--no-color" {
			ui.DisableColor()
			continue
		}
		out = append(out, arg)
	}
	return out
}

func runConnect(args []string) error {
	opts := connect.Options{Out: os.Stdout}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			opts.Alias = args[i+1]
			i++
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown connect option %q", args[i])
		}
	}
	return connect.Run(opts)
}

func runWake(args []string) error {
	opts := wake.Options{Out: os.Stdout}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			opts.Alias = args[i+1]
			i++
		case "--dry-run":
			opts.DryRun = true
		case "--timeout":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--timeout requires a value")
			}
			timeout, err := time.ParseDuration(args[i+1])
			if err != nil {
				return err
			}
			opts.Timeout = timeout
			i++
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown wake option %q", args[i])
		}
	}
	return wake.Run(opts)
}

func runSetup(args []string) error {
	alias := ""
	dryRun := false
	verifyLogin := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			alias = args[i+1]
			i++
		case "--dry-run":
			dryRun = true
		case "--verify":
			verifyLogin = true
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown setup option %q", args[i])
		}
	}
	if !dryRun {
		return fmt.Errorf("setup currently requires --dry-run")
	}
	return setup.WritePlan(setup.Options{Alias: alias, Verify: verifyLogin, Out: os.Stdout})
}

func runVerify(args []string) error {
	opts := verify.Options{Out: os.Stdout}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			opts.Alias = args[i+1]
			i++
		case "--timeout":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--timeout requires a value")
			}
			timeout, err := time.ParseDuration(args[i+1])
			if err != nil {
				return err
			}
			opts.Timeout = timeout
			i++
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown verify option %q", args[i])
		}
	}
	return verify.Run(opts)
}

func runHost(args []string) error {
	if len(args) >= 1 && args[0] == "status" {
		return runHostStatus(args[1:])
	}
	if len(args) >= 1 && args[0] == "authorize" {
		return runHostAuthorize(args[1:])
	}
	if len(args) >= 1 && args[0] == "unauthorize" {
		return runHostUnauthorize(args[1:])
	}
	if len(args) >= 1 && args[0] == "harden" {
		return runHostHarden(args[1:])
	}
	if len(args) >= 1 && args[0] == "validate" {
		return runHostValidate(args[1:])
	}
	if len(args) >= 1 && args[0] == "reload" {
		return runHostReload(args[1:])
	}
	printUsage(os.Stderr)
	return fmt.Errorf("unknown host command %q", joinArgs(args))
}

func runHostStatus(args []string) error {
	opts := status.HostOptions{}
	for _, arg := range args {
		switch arg {
		case "--effective":
			opts.EffectiveSSHD = true
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown host status option %q", arg)
		}
	}
	return status.RunHost(os.Stdout, opts)
}

func runHostHarden(args []string) error {
	opts := hostharden.Options{Out: os.Stdout}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--user":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--user requires a value")
			}
			opts.User = args[i+1]
			i++
		case "--disable-password":
			opts.DisablePassword = true
		case "--dry-run":
			opts.DryRun = true
		case "--apply":
			opts.Apply = true
		case "--unapply":
			opts.Unapply = true
		case "--confirm":
			opts.Confirm = true
		case "--confirm-key-login":
			opts.ConfirmKeyLogin = true
		case "--force":
			opts.Force = true
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown host harden option %q", args[i])
		}
	}
	return hostharden.Run(opts)
}

func runHostValidate(args []string) error {
	if len(args) != 0 {
		printUsage(os.Stderr)
		return fmt.Errorf("unknown host validate option %q", joinArgs(args))
	}
	return hostops.Validate(hostops.ValidateOptions{Out: os.Stdout})
}

func runHostReload(args []string) error {
	opts := hostops.ReloadOptions{Out: os.Stdout}
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			opts.DryRun = true
		case "--apply":
			opts.Apply = true
		case "--confirm":
			opts.Confirm = true
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown host reload option %q", arg)
		}
	}
	return hostops.Reload(opts)
}

func runHostAuthorize(args []string) error {
	opts := hostauth.Options{Out: os.Stdout}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			opts.Alias = args[i+1]
			i++
		case "--public-key":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--public-key requires a value")
			}
			opts.PublicKey = args[i+1]
			i++
		case "--dry-run":
			opts.DryRun = true
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown host authorize option %q", args[i])
		}
	}
	return hostauth.Run(opts)
}

func runHostUnauthorize(args []string) error {
	opts := hostauth.Options{Out: os.Stdout}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			opts.Alias = args[i+1]
			i++
		case "--public-key":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--public-key requires a value")
			}
			opts.PublicKey = args[i+1]
			i++
		case "--dry-run":
			opts.DryRun = true
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown host unauthorize option %q", args[i])
		}
	}
	return hostauth.RunUnauthorize(opts)
}

func runClient(args []string) error {
	if len(args) == 1 && args[0] == "status" {
		return status.RunClient(os.Stdout)
	}
	if len(args) >= 1 && args[0] == "plan" {
		return runClientPlan(args[1:])
	}
	if len(args) >= 1 && args[0] == "apply" {
		return runClientApply(args[1:])
	}
	if len(args) >= 1 && args[0] == "unapply" {
		return runClientUnapply(args[1:])
	}
	if len(args) >= 1 && args[0] == "init" {
		return runClientInit(args[1:])
	}
	printUsage(os.Stderr)
	return fmt.Errorf("unknown client command %q", joinArgs(args))
}

func runClientPlan(args []string) error {
	alias := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			alias = args[i+1]
			i++
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown client plan option %q", args[i])
		}
	}

	cfg, path, err := config.LoadDefault()
	if err != nil {
		return err
	}
	return plan.WriteClient(os.Stdout, cfg, path, alias)
}

func runClientApply(args []string) error {
	alias := ""
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			alias = args[i+1]
			i++
		case "--dry-run":
			dryRun = true
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown client apply option %q", args[i])
		}
	}
	cfg, path, err := config.LoadDefault()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sshConfigPath := clientconfig.DefaultSSHConfigPath(home)
	if dryRun {
		return clientconfig.WriteDryRun(os.Stdout, cfg, path, sshConfigPath, alias)
	}
	return clientconfig.WriteApply(os.Stdout, cfg, path, sshConfigPath, alias)
}

func runClientUnapply(args []string) error {
	alias := ""
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			alias = args[i+1]
			i++
		case "--dry-run":
			dryRun = true
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown client unapply option %q", args[i])
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return clientconfig.WriteUnapply(os.Stdout, clientconfig.DefaultSSHConfigPath(home), alias, dryRun)
}

func runClientInit(args []string) error {
	opts := clientinit.Options{
		In:  os.Stdin,
		Out: os.Stdout,
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--alias":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--alias requires a value")
			}
			opts.Alias = args[i+1]
			i++
		case "--hostname":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--hostname requires a value")
			}
			opts.Hostname = args[i+1]
			i++
		case "--user":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--user requires a value")
			}
			opts.User = args[i+1]
			i++
		case "--identity-file":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--identity-file requires a value")
			}
			opts.IdentityFile = args[i+1]
			i++
		case "--discover":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("--discover requires a value")
			}
			opts.Discover = args[i+1]
			i++
		case "--dry-run":
			opts.DryRun = true
		case "--yes":
			opts.Yes = true
		default:
			printUsage(os.Stderr)
			return fmt.Errorf("unknown client init option %q", args[i])
		}
	}
	return clientinit.Run(opts)
}

func runConfig(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return fmt.Errorf("unknown config command %q", joinArgs(args))
	}

	switch args[0] {
	case "path":
		fmt.Fprintln(os.Stdout, config.DefaultPath())
		return nil
	case "show":
		cfg, path, err := config.LoadDefault()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(os.Stdout, "Stead config\n\nPath: %s\nStatus: missing\n", path)
				return nil
			}
			return err
		}
		config.WriteSummary(os.Stdout, cfg, path)
		return nil
	case "init":
		return runConfigInit(args[1:])
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown config command %q", joinArgs(args))
	}
}

func runConfigInit(args []string) error {
	if len(args) == 1 && args[0] == "--dry-run" {
		path := config.DefaultPath()
		fmt.Fprintf(os.Stdout, "# dry run: would write %s\n", path)
		config.WriteStarter(os.Stdout)
		return nil
	}
	if len(args) == 0 {
		path, err := config.InitDefault()
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("config already exists at %s", path)
			}
			return err
		}
		fmt.Fprintf(os.Stdout, "wrote %s\n", path)
		return nil
	}
	printUsage(os.Stderr)
	return fmt.Errorf("unknown config init option %q", joinArgs(args))
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, "stead manages personal OpenSSH remote-dev setup")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  stead [--no-color] <command> [options]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Core:")
	fmt.Fprintln(out, "  stead status")
	fmt.Fprintln(out, "  stead setup --alias name --dry-run [--verify]")
	fmt.Fprintln(out, "  stead verify --alias name [--timeout 10s]")
	fmt.Fprintln(out, "  stead connect [--alias name]")
	fmt.Fprintln(out, "  stead wake [--dry-run] [--alias name] [--timeout 90s]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Host:")
	fmt.Fprintln(out, "  stead host status [--effective]")
	fmt.Fprintln(out, "  stead host authorize --public-key key [--alias name] [--dry-run]")
	fmt.Fprintln(out, "  stead host unauthorize --public-key key [--alias name] [--dry-run]")
	fmt.Fprintln(out, "  stead host harden (--dry-run|--apply) [--user name] [--disable-password] [--confirm-key-login|--force]")
	fmt.Fprintln(out, "  stead host harden --unapply (--dry-run|--apply) [--confirm]")
	fmt.Fprintln(out, "  stead host validate")
	fmt.Fprintln(out, "  stead host reload (--dry-run|--apply) [--confirm]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Client:")
	fmt.Fprintln(out, "  stead client status")
	fmt.Fprintln(out, "  stead client init [--alias name] [--hostname host] [--discover tailscale] [--user user] [--identity-file path] [--dry-run] [--yes]")
	fmt.Fprintln(out, "  stead client plan [--alias name]")
	fmt.Fprintln(out, "  stead client apply [--dry-run] [--alias name]")
	fmt.Fprintln(out, "  stead client unapply --alias name [--dry-run]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Config:")
	fmt.Fprintln(out, "  stead config path")
	fmt.Fprintln(out, "  stead config show")
	fmt.Fprintln(out, "  stead config init")
	fmt.Fprintln(out, "  stead config init --dry-run")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Help:")
	fmt.Fprintln(out, "  stead help")
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}

	out := args[0]
	for _, arg := range args[1:] {
		out += " " + arg
	}
	return out
}
