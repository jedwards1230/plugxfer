package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/answers"
	"github.com/jedwards1230/plugxfer/internal/detect"
	"github.com/jedwards1230/plugxfer/internal/model"
	"github.com/jedwards1230/plugxfer/internal/rulebook"
)

type Converter struct {
	Rules   *rulebook.Book
	Answers *answers.File
}

func (c *Converter) Plugin(plugin *model.Plugin, input, output string) ([]model.File, model.Report, error) {
	r := model.Report{Source: plugin.Dialect, Target: plugin.Dialect.Other(), Input: input, Output: output}
	files := make(map[string]model.File)
	put := func(file model.File) error {
		file.Path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(file.Path)))
		if _, exists := files[file.Path]; exists {
			return fmt.Errorf("multiple components produce %s", file.Path)
		}
		files[file.Path] = file
		return nil
	}

	policies := skillPolicies(plugin)
	bins := newBinRewriter(bundledBins(plugin))
	overrides := componentOverrides(plugin)
	for _, file := range plugin.Files {
		path := file.Path
		if path == "PLUGXFER-REPORT.md" || path == "plugxfer.map.yaml" {
			continue
		}
		component := componentFor(path, overrides)
		strategy := c.Rules.Strategy(component, plugin.Dialect)
		if strategy == "drop-warn" {
			r.Add(model.Finding{Component: component, Status: model.Dropped, Path: path, Class: model.Loss, Severity: model.High, Message: fmt.Sprintf("%s rule selected drop-warn", component)})
			continue
		}
		if strategy == "copy" {
			if err := put(file); err != nil {
				return nil, r, err
			}
			r.Add(model.Finding{Component: component, Status: model.Copied, Path: path, Message: "copied by rulebook strategy"})
			continue
		}
		if isManifest(path) {
			out, findings, err := c.manifest(file, plugin.Dialect)
			if err != nil {
				return nil, r, err
			}
			for _, finding := range findings {
				r.Add(finding)
			}
			if err := put(out); err != nil {
				return nil, r, err
			}
			continue
		}
		if component == "mcp" {
			out, findings, err := c.mcp(file, plugin.Dialect)
			if err != nil {
				return nil, r, err
			}
			for _, finding := range findings {
				r.Add(finding)
			}
			if err := put(out); err != nil {
				return nil, r, err
			}
			continue
		}
		if component == "hooks" {
			out, findings, err := c.hooks(file, plugin.Dialect)
			if err != nil {
				return nil, r, err
			}
			for _, finding := range findings {
				r.Add(finding)
			}
			if len(out.Data) > 0 {
				if err := put(out); err != nil {
					return nil, r, err
				}
			}
			continue
		}
		if plugin.Dialect == model.Claude && component == "commands" && strings.HasSuffix(path, ".md") {
			out, extras, findings, err := c.commandToSkill(plugin, file, bins)
			if err != nil {
				return nil, r, err
			}
			for _, finding := range findings {
				r.Add(finding)
			}
			for _, extra := range extras {
				if err := put(extra); err != nil {
					return nil, r, err
				}
			}
			if err := put(out); err != nil {
				return nil, r, err
			}
			continue
		}
		if plugin.Dialect == model.Claude && component == "agents" && strings.HasSuffix(path, ".md") {
			out, findings, err := c.agentToTOML(file)
			if err != nil {
				return nil, r, err
			}
			for _, finding := range findings {
				r.Add(finding)
			}
			if err := put(out); err != nil {
				return nil, r, err
			}
			continue
		}
		if plugin.Dialect == model.Codex && component == "agents" && strings.HasSuffix(path, ".toml") {
			out, findings, err := c.agentToMarkdown(file)
			if err != nil {
				return nil, r, err
			}
			for _, finding := range findings {
				r.Add(finding)
			}
			if err := put(out); err != nil {
				return nil, r, err
			}
			continue
		}
		if strings.HasSuffix(path, "/agents/openai.yaml") && strings.HasPrefix(path, "skills/") {
			continue
		}
		if component == "skills" && strings.HasSuffix(path, "/SKILL.md") {
			out, extras, findings, command, err := c.skill(file, plugin.Dialect, policies[path])
			if err != nil {
				return nil, r, err
			}
			for _, finding := range findings {
				r.Add(finding)
			}
			for _, extra := range extras {
				if err := put(extra); err != nil {
					return nil, r, err
				}
			}
			if command != nil {
				if err := put(*command); err != nil {
					return nil, r, err
				}
			} else if err := put(out); err != nil {
				return nil, r, err
			}
			continue
		}
		changed := false
		if path == "CLAUDE.md" && plugin.Dialect == model.Claude {
			file.Path = "AGENTS.md"
			r.Add(transformed("instructions", path, "renamed CLAUDE.md to AGENTS.md"))
			changed = true
		}
		if path == "AGENTS.md" && plugin.Dialect == model.Codex {
			file.Path = "CLAUDE.md"
			r.Add(transformed("instructions", path, "renamed AGENTS.md to CLAUDE.md"))
			changed = true
		}
		if component == "commands" && plugin.Dialect == model.Codex && strings.HasSuffix(path, ".md") {
			file.Data = []byte(escapeClaudeActivation(string(file.Data)))
			r.Add(transformed("commands", path, "escaped Codex-inert syntax that would activate in Claude"))
			changed = true
		}
		if strings.HasSuffix(path, ".md") {
			for _, finding := range detect.Markdown(path, file.Data, plugin.Dialect, c.Rules) {
				r.Add(finding)
			}
		}
		if strategy == "copy-warn" {
			r.Add(model.Finding{Component: "bin", Status: model.Transformed, Path: path, Class: model.Loss, Severity: model.Medium, Message: "copied executable, but Codex does not add bin/ to PATH", Fix: "rewrite recognized command call sites to ${PLUGIN_ROOT}/bin/..."})
			changed = true
		}
		if !bins.empty() && plugin.Dialect == model.Claude && strings.HasSuffix(path, ".md") {
			if rewritten := bins.rewrite(file.Data); !bytes.Equal(rewritten, file.Data) {
				file.Data = rewritten
				r.Add(model.Finding{Component: "bin", Status: model.Transformed, Path: path, Class: model.Loss, Severity: model.Medium, Message: "rewrote bundled bin call sites to ${PLUGIN_ROOT}/bin", Fix: "verify the referenced commands resolve under the plugin root"})
				changed = true
			}
		}
		if err := put(file); err != nil {
			return nil, r, err
		}
		if !changed {
			r.Add(model.Finding{Component: component, Status: model.Copied, Path: path, Message: "copied without structural changes"})
		}
	}

	result := make([]model.File, 0, len(files))
	for _, file := range files {
		result = append(result, file)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, r, nil
}

func isManifest(path string) bool {
	return path == ".claude-plugin/plugin.json" || path == ".codex-plugin/plugin.json"
}
func isHooks(path string) bool { return path == "hooks/hooks.json" }
func transformed(component, path, message string) model.Finding {
	return model.Finding{Component: component, Status: model.Transformed, Path: path, Message: message}
}

func feature(path string) string {
	switch {
	case isManifest(path):
		return "manifest"
	case path == ".mcp.json":
		return "mcp"
	case isHooks(path):
		return "hooks"
	case strings.HasPrefix(path, "commands/"):
		return "commands"
	case strings.HasPrefix(path, "agents/") || strings.HasPrefix(path, ".codex/agents/"):
		return "agents"
	case path == "CLAUDE.md" || path == "AGENTS.md":
		return "instructions"
	case path == ".lsp.json":
		return "lsp"
	case strings.HasPrefix(path, "output-styles/"):
		return "output-styles"
	case strings.HasPrefix(path, "skills/"):
		return "skills"
	case strings.HasPrefix(path, "bin/"):
		return "bin"
	case path == ".app.json":
		return "app"
	default:
		return "other"
	}
}

func bundledBins(plugin *model.Plugin) []string {
	var names []string
	for _, file := range plugin.Files {
		if strings.HasPrefix(file.Path, "bin/") && !strings.Contains(strings.TrimPrefix(file.Path, "bin/"), "/") {
			names = append(names, filepath.Base(file.Path))
		}
	}
	sort.Strings(names)
	return names
}

type componentPath struct {
	path, component string
	directory       bool
}

func componentOverrides(plugin *model.Plugin) []componentPath {
	manifestPath := ".claude-plugin/plugin.json"
	if plugin.Dialect == model.Codex {
		manifestPath = ".codex-plugin/plugin.json"
	}
	file, ok := plugin.File(manifestPath)
	if !ok {
		return nil
	}
	var manifest map[string]any
	if json.Unmarshal(file.Data, &manifest) != nil {
		return nil
	}
	fields := map[string]string{"commands": "commands", "agents": "agents", "skills": "skills", "hooks": "hooks", "mcpServers": "mcp", "lspServers": "lsp", "outputStyles": "output-styles", "apps": "app"}
	var out []componentPath
	for field, component := range fields {
		value, ok := manifest[field]
		if !ok {
			continue
		}
		var paths []string
		switch typed := value.(type) {
		case string:
			paths = []string{typed}
		case []any:
			for _, item := range typed {
				if path, ok := item.(string); ok {
					paths = append(paths, path)
				}
			}
		default:
			continue // inline objects are handled by the manifest converter.
		}
		for _, raw := range paths {
			path := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(raw, "./")))
			directory := strings.HasSuffix(raw, "/") || filepath.Ext(path) == ""
			out = append(out, componentPath{path: path, component: component, directory: directory})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].path) != len(out[j].path) {
			return len(out[i].path) > len(out[j].path)
		}
		return out[i].path < out[j].path
	})
	return out
}

func componentFor(path string, overrides []componentPath) string {
	for _, override := range overrides {
		if path == override.path || (override.directory && strings.HasPrefix(path, strings.TrimSuffix(override.path, "/")+"/")) {
			return override.component
		}
	}
	return feature(path)
}
