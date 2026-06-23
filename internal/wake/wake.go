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
	Send       Sender
}

type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)
type Sender func(network, address string, payload []byte) error

func Run(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
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

	totalTimeout := opts.Timeout
	if totalTimeout == 0 {
		if opts.DryRun {
			totalTimeout = 3 * time.Second
		} else {
			var err error
			totalTimeout, err = parseDurationOrDefault(host.Wake.Timeout, 90*time.Second)
			if err != nil {
				return err
			}
		}
	}
	dial := opts.Dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	send := opts.Send
	if send == nil {
		send = sendUDP
	}

	address := net.JoinHostPort(host.Hostname, strconv.Itoa(defaultPort(host.Port)))
	checkTimeout := totalTimeout
	if !opts.DryRun {
		checkTimeout = minDuration(3*time.Second, totalTimeout)
	}
	reachable, reason := checkReachable(address, checkTimeout, dial)

	ui.PrintTitle(out, "Stead wake")
	fmt.Fprintln(out)
	ui.PrintKV(out, "Alias", alias)
	if opts.DryRun {
		ui.PrintKV(out, "Mode", "dry-run (no packet sent)")
	} else {
		ui.PrintKV(out, "Mode", "apply")
	}
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

	if !opts.DryRun {
		if reachable {
			ui.PrintSection(out, "Changes")
			ui.PrintKV(out, "Wake-on-LAN", "skipped; SSH already reachable")
			fmt.Fprintln(out)
			ui.PrintSection(out, "Next")
			ui.PrintStep(out, 1, "stead connect --alias "+alias)
			return nil
		}
		if !wakeReady(host.Wake) {
			return fmt.Errorf("wake config is incomplete for alias %q", alias)
		}
		if err := sendWake(host.Wake, send); err != nil {
			return err
		}
		interval, err := parseDurationOrDefault(host.Wake.Interval, 2*time.Second)
		if err != nil {
			return err
		}
		ui.PrintSection(out, "Changes")
		ui.PrintKV(out, "Wake-on-LAN", ui.StateDetail(out, "ok", "packet sent"))
		waitReachable(out, alias, address, totalTimeout, interval, dial)
		return nil
	}

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

func sendWake(w config.Wake, send Sender) error {
	mac, err := net.ParseMAC(strings.TrimSpace(w.MACAddress))
	if err != nil {
		return fmt.Errorf("invalid wake MAC address: %w", err)
	}
	payload := magicPacket(mac)
	address := net.JoinHostPort(strings.TrimSpace(w.Broadcast), "9")
	if err := send("udp", address, payload); err != nil {
		return fmt.Errorf("send Wake-on-LAN packet: %w", err)
	}
	return nil
}

func magicPacket(mac net.HardwareAddr) []byte {
	packet := make([]byte, 6+16*len(mac))
	for i := 0; i < 6; i++ {
		packet[i] = 0xff
	}
	offset := 6
	for i := 0; i < 16; i++ {
		copy(packet[offset:offset+len(mac)], mac)
		offset += len(mac)
	}
	return packet
}

func sendUDP(network, address string, payload []byte) error {
	conn, err := net.Dial(network, address)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write(payload)
	return err
}

func waitReachable(out io.Writer, alias, address string, timeout, interval time.Duration, dial DialFunc) {
	deadline := time.Now().Add(timeout)
	for {
		if reachable, _ := checkReachable(address, interval, dial); reachable {
			ui.PrintKV(out, "SSH port", ui.StateDetail(out, "ok", "reachable"))
			fmt.Fprintln(out)
			ui.PrintSection(out, "Next")
			ui.PrintStep(out, 1, "stead connect --alias "+alias)
			return
		}
		if time.Now().Add(interval).After(deadline) {
			ui.PrintKV(out, "SSH port", ui.StateDetail(out, "failed", "timed out after "+timeout.String()))
			return
		}
		time.Sleep(interval)
	}
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

func parseDurationOrDefault(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func isPlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")
}
