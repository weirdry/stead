package verify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/ed/stead/internal/ui"
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

	ui.PrintTitle(out, "Stead verify")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Alias", opts.Alias)
	ui.PrintKV(out, "Check", "ssh BatchMode login")
	fmt.Fprintln(out)

	err := runner(ctx, opts.Alias)
	if err == nil {
		ui.PrintSection(out, "Result")
		ui.PrintKV(out, "Login", ui.State(out, "ok"))
		fmt.Fprintln(out)
		ui.PrintSection(out, "Next steps")
		ui.PrintStep(out, 1, "ssh "+opts.Alias)
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		ui.PrintSection(out, "Result")
		ui.PrintKV(out, "Login", ui.State(out, "failed"))
		ui.PrintKV(out, "Reason", "timed out after "+timeout.String())
		return nil
	}
	ui.PrintSection(out, "Result")
	ui.PrintKV(out, "Login", ui.State(out, "failed"))
	ui.PrintKV(out, "Reason", err.Error())
	return nil
}

func SSHRunner(ctx context.Context, alias string) error {
	cmd := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", alias, "true")
	return cmd.Run()
}
