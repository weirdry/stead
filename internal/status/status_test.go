package status

import "testing"

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
