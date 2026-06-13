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
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, "stead manages personal OpenSSH remote-dev setup")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  stead status")
	fmt.Fprintln(out, "  stead help")
}
