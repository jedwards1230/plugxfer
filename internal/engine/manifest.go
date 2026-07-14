package engine

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func (c *Converter) manifest(file model.File, from model.Dialect) (model.File, []model.Finding, error) {
	var source map[string]any
	if err := json.Unmarshal(file.Data, &source); err != nil {
		return model.File{}, nil, fmt.Errorf("%s: parse manifest: %w", file.Path, err)
	}
	target := map[string]any{}
	copyKeys(source, target, "name", "version", "description", "keywords", "skills", "mcpServers", "hooks")
	var findings []model.Finding
	if inline, ok := source["mcpServers"].(map[string]any); ok {
		wrapped, err := json.Marshal(map[string]any{"mcpServers": inline})
		if err != nil {
			return model.File{}, nil, err
		}
		converted, inlineFindings, err := c.mcp(model.File{Path: file.Path + "#mcpServers", Data: wrapped}, from)
		if err != nil {
			return model.File{}, nil, err
		}
		var convertedRoot map[string]any
		if err := json.Unmarshal(converted.Data, &convertedRoot); err != nil {
			return model.File{}, nil, err
		}
		target["mcpServers"] = convertedRoot["mcpServers"]
		findings = append(findings, inlineFindings...)
	}
	if inline, ok := source["hooks"].(map[string]any); ok {
		wrapped := inline
		if _, exists := inline["hooks"]; !exists {
			wrapped = map[string]any{"hooks": inline}
		}
		data, err := json.Marshal(wrapped)
		if err != nil {
			return model.File{}, nil, err
		}
		converted, inlineFindings, err := c.hooks(model.File{Path: file.Path + "#hooks", Data: data}, from)
		if err != nil {
			return model.File{}, nil, err
		}
		var convertedRoot map[string]any
		if err := json.Unmarshal(converted.Data, &convertedRoot); err != nil {
			return model.File{}, nil, err
		}
		target["hooks"] = convertedRoot["hooks"]
		findings = append(findings, inlineFindings...)
	}
	if from == model.Claude {
		iface := map[string]any{}
		if name, ok := source["name"]; ok {
			iface["displayName"] = name
		}
		if description, ok := source["description"]; ok {
			iface["shortDescription"] = description
		}
		if author, ok := source["author"].(map[string]any); ok {
			if name, ok := author["name"]; ok {
				iface["developerName"] = name
			}
		}
		if homepage, ok := source["homepage"]; ok {
			iface["websiteUrl"] = homepage
		}
		if len(iface) > 0 {
			target["interface"] = iface
		}
		unsupported := []string{"repository", "license", "commands", "agents", "lspServers", "outputStyles", "userConfig", "allowedSettings", "disallowedSettings", "dependencies", "channels"}
		for _, key := range unsupported {
			if _, ok := source[key]; ok {
				findings = append(findings, model.Finding{Component: "manifest", Status: model.Dropped, Path: file.Path, Class: model.Loss, Severity: model.Medium, Message: fmt.Sprintf("manifest field %q is not supported by Codex", key)})
			}
		}
		file.Path = ".codex-plugin/plugin.json"
	} else {
		copyKeys(source, target, "author", "homepage", "repository", "license")
		if iface, ok := source["interface"].(map[string]any); ok {
			if developer, ok := iface["developerName"].(string); ok {
				target["author"] = map[string]any{"name": developer}
			}
			if website, ok := iface["websiteUrl"]; ok {
				target["homepage"] = website
			}
			keys := make([]string, 0, len(iface))
			for key := range iface {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				derived := (key == "displayName" && iface[key] == source["name"]) || (key == "shortDescription" && iface[key] == source["description"])
				if key != "developerName" && key != "websiteUrl" && !derived {
					findings = append(findings, model.Finding{Component: "manifest", Status: model.Dropped, Path: file.Path, Class: model.Loss, Severity: model.Low, Message: fmt.Sprintf("Codex interface field %q has no plugin.json equivalent", key)})
				}
			}
		}
		file.Path = ".claude-plugin/plugin.json"
	}
	data, err := json.MarshalIndent(target, "", "  ")
	if err != nil {
		return model.File{}, nil, err
	}
	file.Data = append(data, '\n')
	findings = append(findings, transformed("manifest", file.Path, "reshaped plugin manifest"))
	return file, findings, nil
}

func copyKeys(source, target map[string]any, keys ...string) {
	for _, key := range keys {
		if value, ok := source[key]; ok {
			target[key] = value
		}
	}
}
