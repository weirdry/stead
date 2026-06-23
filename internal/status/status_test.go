package status

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ed/stead/internal/config"
)

func TestFormatConfigLineNormalizesWhitespace(t *testing.T) {
	got := formatConfigLine([]string{"AuthorizedKeysFile", ".ssh/authorized_keys"})
	want := "AuthorizedKeysFile .ssh/authorized_keys"
	if got != want {
		t.Fatalf("formatConfigLine = %q, want %q", got, want)
	}
}

func TestFormatConfigLineRedactsForceCommand(t *testing.T) {
	got := formatConfigLine([]string{"ForceCommand", "/secret/path", "--token", "value"})
	want := "ForceCommand [redacted]"
	if got != want {
		t.Fatalf("formatConfigLine = %q, want %q", got, want)
	}
}

func TestParseEffectiveSSHDConfigKeepsTrackedKeys(t *testing.T) {
	got := parseEffectiveSSHDConfig(`
port 22
passwordauthentication yes
pubkeyauthentication yes
untracked value
`)
	want := []effectiveValue{
		{Key: "port", Value: "22"},
		{Key: "passwordauthentication", Value: "yes"},
		{Key: "pubkeyauthentication", Value: "yes"},
	}
	if len(got) != len(want) {
		t.Fatalf("parseEffectiveSSHDConfig length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseEffectiveSSHDConfig[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestParseEffectiveSSHDConfigRedactsSensitiveValues(t *testing.T) {
	got := parseEffectiveSSHDConfig(`
forcecommand /secret/path --token value
chrootdirectory /private/path
`)
	want := []effectiveValue{
		{Key: "forcecommand", Value: "[redacted]"},
		{Key: "chrootdirectory", Value: "[redacted]"},
	}
	if len(got) != len(want) {
		t.Fatalf("parseEffectiveSSHDConfig length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseEffectiveSSHDConfig[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestSummarizeAlgorithms(t *testing.T) {
	got := summarizeAlgorithms("a,b,c,d,e,f,g,h")
	want := "8 algorithm(s): a, b, c, d, e, f, ..."
	if got != want {
		t.Fatalf("summarizeAlgorithms = %q, want %q", got, want)
	}
}

func TestEffectiveSSHDErrorExplainsMacOSHostKeyAccess(t *testing.T) {
	got := effectiveSSHDError(errors.New("exit status 1"), []byte("sshd: no hostkeys available -- exiting."))
	want := "sshd -T needs root-readable host keys on this macOS; no sudo attempted"
	if got != want {
		t.Fatalf("effectiveSSHDError = %q, want %q", got, want)
	}
}

func TestPrintSteadHostModeSuppressesConfiguredHostReadiness(t *testing.T) {
	var buf bytes.Buffer
	printStead(&buf, snapshot{
		ConfigPath:  "/tmp/config.toml",
		Config:      check{State: "ok", Detail: "/tmp/config.toml"},
		ConfigAlias: "devmac",
		ConfigHosts: []config.HostStatus{{
			Alias:    "devmac",
			State:    "incomplete",
			Findings: []string{"hostname placeholder"},
		}},
	}, steadDetailHost)

	out := buf.String()
	if strings.Contains(out, "Configured hosts") || strings.Contains(out, "hostname placeholder") {
		t.Fatalf("host mode should not show client host readiness:\n%s", out)
	}
	if !strings.Contains(out, "Host config:") || !strings.Contains(out, "local host checks use system SSH files") {
		t.Fatalf("host mode missing host config note:\n%s", out)
	}
}

func TestPrintSteadFullModeShowsConfiguredHostReadiness(t *testing.T) {
	var buf bytes.Buffer
	printStead(&buf, snapshot{
		ConfigPath:  "/tmp/config.toml",
		Config:      check{State: "ok", Detail: "/tmp/config.toml"},
		ConfigAlias: "devmac",
		ConfigHosts: []config.HostStatus{{
			Alias:    "devmac",
			State:    "incomplete",
			Findings: []string{"hostname placeholder"},
		}},
	}, steadDetailFull)

	out := buf.String()
	if !strings.Contains(out, "Configured hosts") || !strings.Contains(out, "hostname placeholder") {
		t.Fatalf("full mode should show configured host readiness:\n%s", out)
	}
}
