package clientwake

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/ui"
)

type Options struct {
	Alias      string
	ConfigPath string
	MACAddress string
	Broadcast  string
	Timeout    string
	Interval   string
	DryRun     bool
	Out        io.Writer
}

func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	cfg, path, err := loadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}
	alias := opts.Alias
	if alias == "" {
		alias = cfg.Defaults.Alias
	}
	if alias == "" {
		return fmt.Errorf("--alias is required")
	}
	host := cfg.Hosts[alias]
	if host == nil {
		return fmt.Errorf("alias %q not found in %s", alias, path)
	}
	if err := validate(opts); err != nil {
		return err
	}

	next := host.Wake
	if opts.MACAddress != "" {
		next.MACAddress = opts.MACAddress
	}
	if opts.Broadcast != "" {
		next.Broadcast = opts.Broadcast
	}
	if opts.Timeout != "" {
		next.Timeout = opts.Timeout
	}
	if opts.Interval != "" {
		next.Interval = opts.Interval
	}

	ui.PrintTitle(out, "Stead client wake-config")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Config", path)
	ui.PrintKV(out, "Alias", alias)
	if opts.DryRun {
		ui.PrintKV(out, "Mode", "dry-run (no files changed)")
	} else {
		ui.PrintKV(out, "Mode", "apply")
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Wake config")
	ui.PrintKV(out, "MAC address", displayMAC(out, next.MACAddress))
	ui.PrintKV(out, "Broadcast", displayWakeValue(out, next.Broadcast))
	ui.PrintKV(out, "Wake wait timeout", valueOrDefault(next.Timeout, "90s"))
	ui.PrintKV(out, "Wake poll interval", valueOrDefault(next.Interval, "2s"))
	fmt.Fprintln(out)

	ui.PrintSection(out, "Changes")
	if sameWake(host.Wake, next) {
		ui.PrintKV(out, "Action", "no changes needed")
		if opts.DryRun {
			fmt.Fprintln(out)
			fmt.Fprintln(out, "No files were modified.")
		}
		return nil
	}
	if opts.DryRun {
		ui.PrintKV(out, "Action", "would update wake config")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "No files were modified.")
		fmt.Fprintln(out)
		ui.PrintSection(out, "Next steps")
		ui.PrintStep(out, 1, "Re-run without --dry-run when the plan looks right")
		return nil
	}

	host.Wake = next
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	ui.PrintKV(out, "Action", "updated wake config")
	fmt.Fprintln(out)
	ui.PrintSection(out, "Next steps")
	ui.PrintStep(out, 1, "stead wake --alias "+alias+" --dry-run")
	ui.PrintStep(out, 2, "stead connect --alias "+alias+" --wake")
	return nil
}

func loadConfig(path string) (*config.Config, string, error) {
	if path == "" {
		return config.LoadDefault()
	}
	cfg, err := config.Load(path)
	return cfg, path, err
}

func validate(opts Options) error {
	if opts.MACAddress != "" {
		if _, err := net.ParseMAC(strings.TrimSpace(opts.MACAddress)); err != nil {
			return fmt.Errorf("invalid --mac-address: %w", err)
		}
	}
	if opts.Broadcast != "" && net.ParseIP(strings.TrimSpace(opts.Broadcast)) == nil {
		return fmt.Errorf("invalid --broadcast %q", opts.Broadcast)
	}
	if opts.Timeout != "" {
		if _, err := time.ParseDuration(opts.Timeout); err != nil {
			return fmt.Errorf("invalid --timeout: %w", err)
		}
	}
	if opts.Interval != "" {
		if _, err := time.ParseDuration(opts.Interval); err != nil {
			return fmt.Errorf("invalid --interval: %w", err)
		}
	}
	return nil
}

func sameWake(a, b config.Wake) bool {
	return a.MACAddress == b.MACAddress &&
		a.Broadcast == b.Broadcast &&
		a.Timeout == b.Timeout &&
		a.Interval == b.Interval
}

func displayMAC(out io.Writer, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ui.State(out, "missing")
	}
	if strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") {
		return ui.StateDetail(out, "missing", "placeholder")
	}
	return ui.State(out, "ok")
}

func displayWakeValue(out io.Writer, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ui.State(out, "missing")
	}
	if strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") {
		return ui.StateDetail(out, "missing", "placeholder")
	}
	return value
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
