package ui

import (
	"bytes"
	"testing"
)

func TestRenderPlainKeyValueLayout(t *testing.T) {
	var buf bytes.Buffer
	PrintTitle(&buf, "Stead status")
	PrintSection(&buf, "System")
	PrintKV(&buf, "OS", "darwin")
	PrintSubKV(&buf, "devmac", "ok")
	PrintStep(&buf, 1, "ssh devmac")

	got := buf.String()
	for _, want := range []string{
		"Stead status\n============",
		"System\n------",
		"  OS:                            darwin",
		"    devmac:                      ok",
		"  1. ssh devmac",
	} {
		if !bytes.Contains([]byte(got), []byte(want)) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}
