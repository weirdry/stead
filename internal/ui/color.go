package ui

import (
	"io"
	"os"
	"strings"
)

const (
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

var colorDisabled bool

func DisableColor() {
	colorDisabled = true
}

func ColorEnabled(out io.Writer) bool {
	if colorDisabled {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func State(out io.Writer, state string) string {
	if !ColorEnabled(out) {
		return state
	}
	color := stateColor(state)
	if color == "" {
		return state
	}
	return color + state + reset
}

func stateColor(state string) string {
	switch strings.ToLower(state) {
	case "ok", "enabled", "present", "unchanged":
		return green
	case "warn", "warning", "missing", "unknown", "disabled", "incomplete", "absent":
		return yellow
	case "risk", "failed", "failure", "error":
		return red
	default:
		return ""
	}
}
