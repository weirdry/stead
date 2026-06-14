package main

import (
	"fmt"
	"os"

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
	printUsage(os.Stderr)
	return fmt.Errorf("unknown client command %q", joinArgs(args))
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, "stead manages personal OpenSSH remote-dev setup")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  stead status")
	fmt.Fprintln(out, "  stead host status")
	fmt.Fprintln(out, "  stead client status")
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
