package version

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrint(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, Date
	t.Cleanup(func() {
		Version, Commit, Date = oldVersion, oldCommit, oldDate
	})
	Version = "v0.1.0"
	Commit = "abc123"
	Date = "2026-06-25"

	var buf bytes.Buffer
	Print(&buf)

	for _, want := range []string{
		"Stead version",
		"Version:",
		"v0.1.0",
		"Commit:",
		"abc123",
		"Build date:",
		"2026-06-25",
		"OS/Arch:",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestInfoUsesExplicitValues(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, Date
	t.Cleanup(func() {
		Version, Commit, Date = oldVersion, oldCommit, oldDate
	})
	Version = "v0.1.0"
	Commit = "abc123"
	Date = "2026-06-25"

	got := Info()
	if got.Version != "v0.1.0" || got.Commit != "abc123" || got.Date != "2026-06-25" {
		t.Fatalf("Info = %#v", got)
	}
}
