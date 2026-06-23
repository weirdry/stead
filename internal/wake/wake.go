package wake

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/ed/stead/internal/config"
	"github.com/ed/stead/internal/ui"
)

type Options struct {
	Alias      string
	ConfigPath string
	DryRun     bool
	Timeout    time.Duration
	Out        io.Writer
	Dial       DialFunc
}

type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	if !opts.DryRun {
		return fmt.Errorf("wake currently requires --dry-run")
	}

	cfg, cfgPath, err := loadConfig(opts.ConfigPath)
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
		return fmt.Errorf("alias %q not found in %s", alias, cfgPath)
	}
	if strings.TrimSpace(host.Hostname) == "" || isPlaceholder(host.Hostname) {
		return fmt.Errorf("alias %q has no usable hostname", alias)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}
	dial := opts.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}

	address := net.JoinHostPort(host.Hostname, strconv.Itoa(defaultPort(host.Port)))
	reachable, reason := checkReachable(address, timeout, dial)

	ui.PrintTitle(out, "Stead wake")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Alias", alias)
	ui.PrintKV(out, "Mode", "dry-run (no packet sent)")
	ui.PrintKV(out, "Target", address)
	fmt.Fprintln(out)

	ui.PrintSection(out, "Reachability")
	if reachable {
		ui.PrintKV(out, "SSH port", ui.StateDetail(out, "ok", "reachable"))
	} else {
		ui.PrintKV(out, "SSH port", ui.StateDetail(out, "unreachable", reason))
	}
	fmt.Fprintln(out)

	ui.PrintSection(out, "Wake config")
	ui.PrintKV(out, "MAC address", wakeValue(out, host.Wake.MACAddress))
	ui.PrintKV(out, "Broadcast", wakeValue(out, host.Wake.Broadcast))
	ui.PrintKV(out, "Timeout", valueOrDefault(host.Wake.Timeout, "90s"))
	ui.PrintKV(out, "Interval", valueOrDefault(host.Wake.Interval, "2s"))
	fmt.Fprintln(out)

	ui.PrintSection(out, "Next")
	if reachable {
		ui.PrintStep(out, 1, "stead connect --alias "+alias)
	} else if wakeReady(host.Wake) {
		ui.PrintStep(out, 1, "future wake apply will send Wake-on-LAN, wait for SSH, then connect")
	} else {
		ui.PrintStep(out, 1, "configure hosts."+alias+".wake before sending Wake-on-LAN")
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "No Wake-on-LAN packet was sent.")
	return nil
}

func checkReachable(address string, timeout time.Duration, dial DialFunc) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := dial(ctx, "tcp", address)
	if err != nil {
		if ctx.Err() != nil {
			return false, "timed out after " + timeout.String()
		}
		return false, err.Error()
	}
	_ = conn.Close()
	return true, ""
}

func wakeValue(out io.Writer, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ui.State(out, "missing")
	}
	if isPlaceholder(value) {
		return ui.StateDetail(out, "missing", "placeholder")
	}
	return ui.State(out, "ok")
}

func wakeReady(w config.Wake) bool {
	return !isPlaceholder(w.MACAddress) && !isPlaceholder(w.Broadcast) &&
		strings.TrimSpace(w.MACAddress) != "" && strings.TrimSpace(w.Broadcast) != ""
}

func loadConfig(path string) (*config.Config, string, error) {
	if path == "" {
		return config.LoadDefault()
	}
	cfg, err := config.Load(path)
	return cfg, path, err
}

func defaultPort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func isPlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")
}
