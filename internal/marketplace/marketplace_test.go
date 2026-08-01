package marketplace

import (
	"strings"
	"testing"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func TestParseLocalAndRemoteSources(t *testing.T) {
	file := model.File{Path: ".claude-plugin/marketplace.json", Data: []byte(`{"name":"test","plugins":[{"name":"local","source":"./plugins/local"},{"name":"object","source":{"source":"local","path":"plugins/object"}},{"name":"remote","source":{"source":"url","url":"https://example.test/repo"}}]}`)}
	registry, err := Parse(file, model.Claude)
	if err != nil {
		t.Fatal(err)
	}
	if path, local, err := registry.Entries[0].LocalPath(); err != nil || !local || path != "plugins/local" {
		t.Fatalf("string local = %q, %v, %v", path, local, err)
	}
	if path, local, err := registry.Entries[1].LocalPath(); err != nil || !local || path != "plugins/object" {
		t.Fatalf("object local = %q, %v, %v", path, local, err)
	}
	if _, local, err := registry.Entries[2].LocalPath(); err != nil || local {
		t.Fatalf("remote local=%v err=%v", local, err)
	}
	out, err := registry.Render(model.Codex)
	if err != nil {
		t.Fatal(err)
	}
	if out.Path != ".agents/plugins/marketplace.json" || !strings.Contains(string(out.Data), `"installation": "AVAILABLE"`) {
		t.Fatalf("unexpected Codex registry:\n%s", out.Data)
	}
}

func TestLocalPathRejectsTraversal(t *testing.T) {
	entry := Entry{Name: "bad", Source: "../outside"}
	if _, _, err := entry.LocalPath(); err == nil {
		t.Fatal("expected traversal error")
	}
}
