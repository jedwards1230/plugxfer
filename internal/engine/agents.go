package engine

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jedwards1230/plugxfer/internal/frontmatter"
	"github.com/jedwards1230/plugxfer/internal/model"
)

func (c *Converter) agentToTOML(file model.File) (model.File, []model.Finding, error) {
	doc, err := frontmatter.Parse(file.Data, true)
	if err != nil {
		return model.File{}, nil, fmt.Errorf("%s: %w", file.Path, err)
	}
	name := frontmatter.String(doc.Fields, "name")
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(file.Path), ".md")
	}
	description := frontmatter.String(doc.Fields, "description")
	if description == "" {
		return model.File{}, nil, fmt.Errorf("%s: agent description is required", file.Path)
	}
	target := map[string]any{"name": name, "description": description, "developer_instructions": strings.TrimSpace(doc.Body)}
	var findings []model.Finding
	if value := frontmatter.String(doc.Fields, "effort"); value != "" {
		if mapped, ok := c.Rules.Effort.Lookup(model.Claude, value); ok {
			target["model_reasoning_effort"] = mapped
		} else {
			findings = append(findings, droppedField(file.Path, "effort", value))
		}
	}
	if value := frontmatter.String(doc.Fields, "permissionMode"); value != "" {
		if mapped, ok := c.Rules.PermissionMode.Lookup(model.Claude, value); ok {
			target["sandbox_mode"] = mapped
		} else {
			findings = append(findings, droppedField(file.Path, "permissionMode", value))
		}
	}
	if value := frontmatter.String(doc.Fields, "model"); value != "" && value != "inherit" {
		if mapped, ok := c.Answers.Model("claude-to-codex", value); ok {
			target["model"] = mapped
		} else {
			c.Answers.StubModel("claude-to-codex", value)
			findings = append(findings, needsModel(file.Path, value))
		}
	}
	if value, ok := doc.Fields["mcpServers"]; ok {
		target["mcp_servers"] = normalizeStringList(value)
	}
	for _, key := range []string{"tools", "disallowedTools", "maxTurns", "skills", "hooks", "memory", "background", "isolation", "color"} {
		if value, ok := doc.Fields[key]; ok {
			findings = append(findings, droppedField(file.Path, key, fmt.Sprint(value)))
		}
	}
	var out bytes.Buffer
	if err := toml.NewEncoder(&out).Encode(target); err != nil {
		return model.File{}, nil, err
	}
	findings = append(findings, transformed("agents", file.Path, "converted Markdown agent to project .codex/agents TOML"))
	return model.File{Path: ".codex/agents/" + name + ".toml", Mode: 0o644, Data: out.Bytes()}, findings, nil
}

func (c *Converter) agentToMarkdown(file model.File) (model.File, []model.Finding, error) {
	var source map[string]any
	if _, err := toml.Decode(string(file.Data), &source); err != nil {
		return model.File{}, nil, fmt.Errorf("%s: parse agent TOML: %w", file.Path, err)
	}
	name, _ := source["name"].(string)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(file.Path), ".toml")
	}
	description, _ := source["description"].(string)
	if description == "" {
		return model.File{}, nil, fmt.Errorf("%s: agent description is required", file.Path)
	}
	body, _ := source["developer_instructions"].(string)
	if strings.TrimSpace(body) == "" {
		return model.File{}, nil, fmt.Errorf("%s: developer_instructions is required", file.Path)
	}
	fields := map[string]any{"name": name, "description": description}
	var findings []model.Finding
	if value, ok := source["model_reasoning_effort"].(string); ok {
		if mapped, ok := c.Rules.Effort.Lookup(model.Codex, value); ok {
			fields["effort"] = mapped
		} else {
			findings = append(findings, droppedField(file.Path, "model_reasoning_effort", value))
		}
	}
	if value, ok := source["sandbox_mode"].(string); ok {
		if mapped, ok := c.Rules.PermissionMode.Lookup(model.Codex, value); ok {
			fields["permissionMode"] = mapped
		} else {
			findings = append(findings, droppedField(file.Path, "sandbox_mode", value))
		}
	}
	if value, ok := source["model"].(string); ok {
		if mapped, ok := c.Answers.Model("codex-to-claude", value); ok {
			fields["model"] = mapped
		} else {
			c.Answers.StubModel("codex-to-claude", value)
			findings = append(findings, needsModel(file.Path, value))
		}
	}
	if value, ok := source["mcp_servers"]; ok {
		fields["mcpServers"] = value
	}
	known := map[string]bool{"name": true, "description": true, "developer_instructions": true, "model": true, "model_reasoning_effort": true, "sandbox_mode": true, "mcp_servers": true}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := source[key]
		if !known[key] {
			findings = append(findings, droppedField(file.Path, key, fmt.Sprint(value)))
		}
	}
	data, err := frontmatter.Render(frontmatter.Document{Fields: fields, Body: body})
	if err != nil {
		return model.File{}, nil, err
	}
	findings = append(findings, transformed("agents", file.Path, "converted project Codex agent TOML to Markdown agent"))
	return model.File{Path: "agents/" + name + ".md", Mode: 0o644, Data: data}, findings, nil
}

func droppedField(path, key, value string) model.Finding {
	return model.Finding{Component: "agents", Status: model.Dropped, Path: path, Class: model.Loss, Severity: model.Medium, Message: fmt.Sprintf("agent field %q value %q has no safe mapping", key, value)}
}

func normalizeStringList(value any) any {
	if text, ok := value.(string); ok {
		parts := strings.Split(text, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out
	}
	return value
}
