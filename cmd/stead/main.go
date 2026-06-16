package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/ed/stead/internal/clientconfig"
	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/plan"
	"github.com/ed/stead/internal/status"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "stead: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}

	switch args[0] {
	case "status":
		return status.Run(os.Stdout)
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

func runHost(args []string) error {
	if len(args) == 1 && args[0] == "status" {
		return status.RunHost(os.Stdout)
	}
	printUsage(os.Stderr)
	return fmt.Errorf("unknown host command %q", joinArgs(args))
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
	if !dryRun {
		return fmt.Errorf("client apply currently requires --dry-run")
	}

	cfg, path, err := config.LoadDefault()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return clientconfig.WriteDryRun(os.Stdout, cfg, path, clientconfig.DefaultSSHConfigPath(home), alias)
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
	fmt.Fprintln(out, "  stead status")
	fmt.Fprintln(out, "  stead host status")
	fmt.Fprintln(out, "  stead client status")
	fmt.Fprintln(out, "  stead client plan [--alias name]")
	fmt.Fprintln(out, "  stead client apply --dry-run [--alias name]")
	fmt.Fprintln(out, "  stead config path")
	fmt.Fprintln(out, "  stead config show")
	fmt.Fprintln(out, "  stead config init")
	fmt.Fprintln(out, "  stead config init --dry-run")
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
