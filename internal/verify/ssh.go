package verify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

type Options struct {
	Alias   string
	Timeout time.Duration
	Out     io.Writer
	Runner  Runner
}

type Runner func(ctx context.Context, alias string) error

func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	if opts.Alias == "" {
		return fmt.Errorf("--alias is required")
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	runner := opts.Runner
	if runner == nil {
		runner = SSHRunner
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Fprintln(out, "Stead verify")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Alias: %s\n", opts.Alias)
	fmt.Fprintf(out, "Check: ssh BatchMode login\n")
	fmt.Fprintln(out)

	err := runner(ctx, opts.Alias)
	if err == nil {
		fmt.Fprintln(out, "Result: ok")
		fmt.Fprintf(out, "Next: ssh %s\n", opts.Alias)
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		fmt.Fprintln(out, "Result: failed")
		fmt.Fprintf(out, "Reason: timed out after %s\n", timeout)
		return nil
	}
	fmt.Fprintln(out, "Result: failed")
	fmt.Fprintf(out, "Reason: %v\n", err)
	return nil
}

func SSHRunner(ctx context.Context, alias string) error {
	cmd := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", alias, "true")
	return cmd.Run()
}
