package rulebook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func TestEmbeddedAndPublicRulebooksMatchBehavior(t *testing.T) {
	embedded, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	public, err := Load(filepath.Join("..", "..", "rules"))
	if err != nil {
		t.Fatal(err)
	}
	for _, feature := range []string{"manifest", "skills", "mcp", "hooks", "commands", "agents", "instructions", "bin", "lsp", "output-styles", "app", "other"} {
		for _, dialect := range []model.Dialect{model.Claude, model.Codex} {
			if got, want := public.Strategy(feature, dialect), embedded.Strategy(feature, dialect); got != want {
				t.Errorf("%s/%s = %q, want %q", feature, dialect, got, want)
			}
		}
	}
	if len(public.Detectors) != len(embedded.Detectors) {
		t.Fatalf("public detectors = %d, embedded = %d", len(public.Detectors), len(embedded.Detectors))
	}
	for i := range embedded.Detectors {
		got, want := public.Detectors[i], embedded.Detectors[i]
		if got.ID != want.ID || got.Pattern != want.Pattern || got.ClaudeToCodex != want.ClaudeToCodex || got.CodexToClaude != want.CodexToClaude || got.Severity != want.Severity {
			t.Errorf("detector %d differs: %#v != %#v", i, got, want)
		}
	}
	for _, test := range []struct {
		name             string
		public, embedded DirectionMap
	}{
		{"effort", public.Effort, embedded.Effort}, {"permission", public.PermissionMode, embedded.PermissionMode}, {"hooks", public.HookEvents, embedded.HookEvents},
	} {
		for _, dialect := range []model.Dialect{model.Claude, model.Codex} {
			for _, value := range []string{"low", "max", "xhigh", "acceptEdits", "workspace-write", "SessionEnd", "PermissionRequest"} {
				got, gotOK := test.public.Lookup(dialect, value)
				want, wantOK := test.embedded.Lookup(dialect, value)
				if got != want || gotOK != wantOK {
					t.Errorf("%s/%s/%s = %q,%v want %q,%v", test.name, dialect, value, got, gotOK, want, wantOK)
				}
			}
		}
	}
}

func TestLoadRejectsUnknownStrategy(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"features.yaml", "detectors.yaml", "values.yaml"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "rules", name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "features.yaml" {
			data = []byte("version: 1\nfeatures:\n  - {id: manifest, claude_to_codex: unknown, codex_to_claude: copy}\n")
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected unknown-strategy error")
	}
}
