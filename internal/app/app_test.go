package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func TestCheckIsReadOnlyAndReportsNeedsMap(t *testing.T) {
	input := filepath.Join("..", "..", "testdata", "claude-basic")
	before, err := snapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(Options{Mode: Check, Input: input, Target: model.Codex})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 2 {
		t.Fatalf("exit = %d, want 2", result.ExitCode)
	}
	if !strings.Contains(string(result.Markdown), "needs-map") {
		t.Fatalf("report missing needs-map:\n%s", result.Markdown)
	}
	after, err := snapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("check modified input")
	}
}

func TestConvertWritesAtomicTree(t *testing.T) {
	input := filepath.Join("..", "..", "testdata", "claude-basic")
	output := filepath.Join(t.TempDir(), "converted")
	result, err := Run(Options{Mode: Convert, Input: input, Output: output, Target: model.Codex, MapPath: filepath.Join(input, "map.yaml")})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("exit = %d, want 1", result.ExitCode)
	}
	for _, path := range []string{"PLUGXFER-REPORT.md", ".codex-plugin/plugin.json", ".codex/agents/reviewer.toml", "skills/deploy/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(path))); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
	if _, err := Run(Options{Mode: Convert, Input: input, Output: output, Target: model.Codex}); err == nil {
		t.Fatal("expected existing-output error")
	}
}

func TestMarketplaceFanoutAndRemoteSkip(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".claude-plugin/marketplace.json", `{"name":"fixtures","plugins":[{"name":"local","source":"./plugins/local","description":"local","category":"other"},{"name":"remote","source":{"source":"url","url":"https://example.test/plugin.git"},"description":"remote","category":"other"}]}`)
	write(t, root, "plugins/local/.claude-plugin/plugin.json", `{"name":"local","description":"local"}`)
	write(t, root, "plugins/local/skills/test/SKILL.md", "---\nname: test\ndescription: test\n---\n\nTest.\n")
	output := filepath.Join(t.TempDir(), "converted")
	result, err := Run(Options{Mode: Convert, Input: root, Output: output, Target: model.Codex})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit = %d, want 0\n%s", result.ExitCode, result.Markdown)
	}
	for _, path := range []string{".agents/plugins/marketplace.json", ".claude-plugin/marketplace.json", "plugins/local/.codex-plugin/plugin.json", "plugins/local/PLUGXFER-REPORT.md", "PLUGXFER-REPORT.md"} {
		if _, err := os.Stat(filepath.Join(output, filepath.FromSlash(path))); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
	if !strings.Contains(string(result.Markdown), "remote plugin remote was not fetched") {
		t.Fatalf("aggregate report missing remote skip:\n%s", result.Markdown)
	}
}

func TestMarketplaceOnlySelection(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".claude-plugin/marketplace.json", `{"name":"fixtures","plugins":[{"name":"one","source":"./plugins/one","description":"one","category":"other"},{"name":"two","source":"./plugins/two","description":"two","category":"other"}]}`)
	for _, name := range []string{"one", "two"} {
		write(t, root, "plugins/"+name+"/.claude-plugin/plugin.json", `{"name":"`+name+`","description":"`+name+`"}`)
	}
	output := filepath.Join(t.TempDir(), "converted")
	_, err := Run(Options{Mode: Convert, Input: root, Output: output, Target: model.Codex, Only: map[string]bool{"one": true}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "plugins", "one", ".codex-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "plugins", "two", ".claude-plugin", "plugin.json")); err != nil {
		t.Fatalf("unselected plugin should be preserved: %v", err)
	}
	_, err = Run(Options{Mode: Check, Input: root, Target: model.Codex, Only: map[string]bool{"missing": true}})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing selection error = %v", err)
	}
}

func TestSemanticRoundTrip(t *testing.T) {
	input := filepath.Join("..", "..", "testdata", "claude-basic")
	root := t.TempDir()
	codex := filepath.Join(root, "codex")
	mapPath := filepath.Join(input, "map.yaml")
	if _, err := Run(Options{Mode: Convert, Input: input, Output: codex, Target: model.Codex, MapPath: mapPath}); err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(root, "claude")
	if _, err := Run(Options{Mode: Convert, Input: codex, Output: claude, Target: model.Claude, MapPath: mapPath}); err != nil {
		t.Fatal(err)
	}
	command, err := os.ReadFile(filepath.Join(claude, "commands", "deploy.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"model: sonnet", "@runbook.txt", "deploy-helper $ARGUMENTS", "!`unsafe example`"} {
		if !strings.Contains(string(command), want) {
			t.Errorf("round-trip command missing %q:\n%s", want, command)
		}
	}
	for _, unwanted := range []string{"## Command Template", "plugxfer: materialized", "${PLUGIN_ROOT}", `\!` + "`unsafe"} {
		if strings.Contains(string(command), unwanted) {
			t.Errorf("round-trip command retained %q:\n%s", unwanted, command)
		}
	}
	agent, err := os.ReadFile(filepath.Join(claude, "agents", "reviewer.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agent), "effort: max") || !strings.Contains(string(agent), "model: opus") {
		t.Errorf("round-trip agent lost mapped fields:\n%s", agent)
	}
}

func snapshot(root string) (string, error) {
	var out strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out.WriteString(rel)
		out.WriteString(info.Mode().String())
		if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out.Write(data)
		}
		return nil
	})
	return out.String(), err
}

func write(t *testing.T, root, path, data string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
