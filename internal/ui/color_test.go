package ui

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestStateDoesNotColorNonTerminalWriter(t *testing.T) {
	var buf bytes.Buffer
	got := State(&buf, "ok")
	if got != "ok" {
		t.Fatalf("State = %q, want plain ok", got)
	}
}

func TestStateHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := State(os.Stdout, "failed")
	if strings.Contains(got, "\033[") {
		t.Fatalf("State should not contain ANSI escape sequence with NO_COLOR: %q", got)
	}
}
