package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jedwards1230/plugxfer/internal/answers"
	"github.com/jedwards1230/plugxfer/internal/model"
	"github.com/jedwards1230/plugxfer/internal/reader"
	"github.com/jedwards1230/plugxfer/internal/rulebook"
)

func TestClaudeToCodexFixture(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "claude-basic")
	plugin, err := reader.Read(root, model.Claude)
	if err != nil {
		t.Fatal(err)
	}
	book, err := rulebook.Load("")
	if err != nil {
		t.Fatal(err)
	}
	mapFile, err := answers.Load(filepath.Join(root, "map.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	files, report, err := (&Converter{Rules: book, Answers: mapFile}).Plugin(plugin, root, "out")
	if err != nil {
		t.Fatal(err)
	}
	byPath := index(files)
	for _, path := range []string{".codex-plugin/plugin.json", ".codex/agents/reviewer.toml", "skills/deploy/SKILL.md", "skills/review/SKILL.md", "skills/review/agents/openai.yaml", "hooks/hooks.json", ".mcp.json", "AGENTS.md", "bin/deploy-helper"} {
		if _, ok := byPath[path]; !ok {
			t.Errorf("missing output %s", path)
		}
	}
	if strings.Contains(string(byPath["hooks/hooks.json"].Data), `"type": "prompt"`) || strings.Contains(string(byPath["hooks/hooks.json"].Data), `"async"`) {
		t.Errorf("hooks were not filtered:\n%s", byPath["hooks/hooks.json"].Data)
	}
	if !strings.Contains(string(byPath[".mcp.json"].Data), `"env_vars"`) || !strings.Contains(string(byPath[".mcp.json"].Data), `"bearer_token_env_var"`) || !strings.Contains(string(byPath[".mcp.json"].Data), `"env_http_headers"`) {
		t.Errorf("MCP env was not transformed:\n%s", byPath[".mcp.json"].Data)
	}
	if !strings.Contains(string(byPath["skills/deploy/SKILL.md"].Data), "materialized @runbook.txt") || !strings.Contains(string(byPath["skills/deploy/SKILL.md"].Data), "${PLUGIN_ROOT}/bin/deploy-helper") {
		t.Errorf("command conversion incomplete:\n%s", byPath["skills/deploy/SKILL.md"].Data)
	}
	if report.ExitCode(false) != 1 {
		t.Fatalf("exit code = %d, want 1", report.ExitCode(false))
	}
	compareGolden(t, files, filepath.Join("..", "..", "testdata", "golden", "claude-to-codex"), map[string]bool{"map.yaml": true})
}

func TestCodexToClaudeFixture(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "codex-basic")
	plugin, err := reader.Read(root, model.Codex)
	if err != nil {
		t.Fatal(err)
	}
	book, err := rulebook.Load("")
	if err != nil {
		t.Fatal(err)
	}
	mapFile, err := answers.Load(filepath.Join("..", "..", "testdata", "claude-basic", "map.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	files, report, err := (&Converter{Rules: book, Answers: mapFile}).Plugin(plugin, root, "out")
	if err != nil {
		t.Fatal(err)
	}
	byPath := index(files)
	for _, path := range []string{".claude-plugin/plugin.json", "agents/reviewer.md", "skills/inspect/SKILL.md", "hooks/hooks.json", ".mcp.json", "CLAUDE.md"} {
		if _, ok := byPath[path]; !ok {
			t.Errorf("missing output %s", path)
		}
	}
	if _, ok := byPath[".app.json"]; ok {
		t.Error("Codex app should be dropped")
	}
	if !strings.Contains(string(byPath["skills/inspect/SKILL.md"].Data), "disable-model-invocation: true") {
		t.Errorf("openai policy not mapped:\n%s", byPath["skills/inspect/SKILL.md"].Data)
	}
	if !strings.Contains(string(byPath["hooks/hooks.json"].Data), "${CLAUDE_PLUGIN_ROOT}") || strings.Contains(string(byPath["hooks/hooks.json"].Data), "PermissionRequest") {
		t.Errorf("hooks not reversed:\n%s", byPath["hooks/hooks.json"].Data)
	}
	if report.ExitCode(false) != 1 {
		t.Fatalf("exit code = %d, want 1", report.ExitCode(false))
	}
	compareGolden(t, files, filepath.Join("..", "..", "testdata", "golden", "codex-to-claude"), nil)
}

func TestCustomPathsAndInlineManifestComponents(t *testing.T) {
	root := t.TempDir()
	writeEngineFixture(t, root, ".claude-plugin/plugin.json", `{"name":"custom","commands":["./prompts/"],"skills":["./knowledge/"],"hooks":"./config/hooks.json","mcpServers":{"inline":{"command":"server","env":{"TOKEN":"${TOKEN}"}}}}`)
	writeEngineFixture(t, root, "prompts/run.md", "---\ndescription: Run it\n---\n\nRun.\n")
	writeEngineFixture(t, root, "knowledge/inspect/SKILL.md", "---\nname: inspect\ndescription: Inspect\n---\n\nInspect.\n")
	writeEngineFixture(t, root, "config/hooks.json", `{"hooks":{"SessionEnd":[{"hooks":[{"type":"command","command":"echo done"}]}]}}`)
	plugin, err := reader.Read(root, model.Claude)
	if err != nil {
		t.Fatal(err)
	}
	book, err := rulebook.Load("")
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := (&Converter{Rules: book, Answers: answers.Empty()}).Plugin(plugin, root, "out")
	if err != nil {
		t.Fatal(err)
	}
	byPath := index(files)
	for _, path := range []string{".codex-plugin/plugin.json", "skills/run/SKILL.md", "knowledge/inspect/SKILL.md"} {
		if _, ok := byPath[path]; !ok {
			t.Errorf("missing custom-path output %s", path)
		}
	}
	if hooks, ok := byPath["config/hooks.json"]; !ok || strings.Contains(string(hooks.Data), "SessionEnd") {
		t.Errorf("custom hooks were not filtered: %s", hooks.Data)
	}
	manifest := string(byPath[".codex-plugin/plugin.json"].Data)
	if !strings.Contains(manifest, `"env_vars"`) {
		t.Errorf("inline MCP was not transformed:\n%s", manifest)
	}
}

func TestRuleOverrideChangesStrategy(t *testing.T) {
	rulesDir := t.TempDir()
	public := filepath.Join("..", "..", "rules")
	for _, name := range []string{"features.yaml", "detectors.yaml", "values.yaml"} {
		data, err := os.ReadFile(filepath.Join(public, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "features.yaml" {
			data = []byte(strings.ReplaceAll(string(data), "claude_to_codex: drop-warn", "claude_to_codex: copy"))
		}
		if err := os.WriteFile(filepath.Join(rulesDir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	book, err := rulebook.Load(rulesDir)
	if err != nil {
		t.Fatal(err)
	}
	plugin, err := reader.Read(filepath.Join("..", "..", "testdata", "claude-basic"), model.Claude)
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := (&Converter{Rules: book, Answers: answers.Empty()}).Plugin(plugin, "in", "out")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := index(files)[".lsp.json"]; !ok {
		t.Fatal("rule override did not select copy strategy for lsp")
	}
}

func index(files []model.File) map[string]model.File {
	out := map[string]model.File{}
	for _, file := range files {
		out[file.Path] = file
	}
	return out
}

func compareGolden(t *testing.T, files []model.File, root string, ignored map[string]bool) {
	t.Helper()
	actual := map[string]model.File{}
	for _, file := range files {
		if !ignored[file.Path] {
			actual[file.Path] = file
		}
	}
	var expectedPaths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		expectedPaths = append(expectedPaths, rel)
		expected, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got, ok := actual[rel]
		if !ok {
			t.Errorf("golden output missing %s", rel)
			return nil
		}
		if !bytes.Equal(got.Data, expected) {
			t.Errorf("%s differs from golden\n--- got ---\n%s\n--- want ---\n%s", rel, got.Data, expected)
		}
		if os.FileMode(got.Mode).Perm() != info.Mode().Perm() {
			t.Errorf("%s mode = %o, want %o", rel, got.Mode, info.Mode().Perm())
		}
		delete(actual, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(expectedPaths)
	var extras []string
	for path := range actual {
		extras = append(extras, path)
	}
	sort.Strings(extras)
	if len(extras) > 0 {
		t.Errorf("unexpected golden outputs: %v", extras)
	}
}

func writeEngineFixture(t *testing.T, root, path, data string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
