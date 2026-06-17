package verify

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunReportsOK(t *testing.T) {
	var buf bytes.Buffer
	err := Run(Options{
		Alias: "devmac",
		Out:   &buf,
		Runner: func(ctx context.Context, alias string) error {
			if alias != "devmac" {
				t.Fatalf("alias = %q", alias)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Result: ok") {
		t.Fatalf("output missing ok result:\n%s", buf.String())
	}
}

func TestRunReportsFailure(t *testing.T) {
	var buf bytes.Buffer
	errVerify := errors.New("ssh failed")
	err := Run(Options{
		Alias: "devmac",
		Out:   &buf,
		Runner: func(ctx context.Context, alias string) error {
			return errVerify
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Result: failed") || !strings.Contains(buf.String(), "ssh failed") {
		t.Fatalf("output missing failure:\n%s", buf.String())
	}
}

func TestRunReportsTimeout(t *testing.T) {
	var buf bytes.Buffer
	err := Run(Options{
		Alias:   "devmac",
		Timeout: time.Nanosecond,
		Out:     &buf,
		Runner: func(ctx context.Context, alias string) error {
			<-ctx.Done()
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "timed out") {
		t.Fatalf("output missing timeout:\n%s", buf.String())
	}
}

func TestRunRequiresAlias(t *testing.T) {
	err := Run(Options{Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected alias error")
	}
}
