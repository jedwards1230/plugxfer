package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jedwards1230/plugxfer/internal/app"
	"github.com/jedwards1230/plugxfer/internal/model"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := Run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "plugxfer check") {
		t.Fatalf("help output missing check usage: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestParseInterspersedFlags(t *testing.T) {
	options, help, err := parse(app.Convert, []string{"input", "--to", "codex", "--strict", "-o", "output", "--only=one,two"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if help || options.Input != "input" || options.Output != "output" || options.Target != model.Codex || !options.Strict || !options.Only["one"] || !options.Only["two"] {
		t.Fatalf("unexpected options: %#v help=%v", options, help)
	}
}

func TestParseRejectsInvalidSurface(t *testing.T) {
	if _, _, err := parse(app.Convert, []string{"input"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected missing-output error")
	}
	if _, _, err := parse(app.Check, []string{"input", "-o", "out"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected check-output error")
	}
	if _, _, err := parse(app.Check, []string{"input", "--to", "other"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected invalid-target error")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := Run([]string{"unknown"}, &stdout, &stderr); code != 3 {
		t.Fatalf("Run() code = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "unknown"`) {
		t.Fatalf("stderr missing unknown-command error: %q", stderr.String())
	}
}
