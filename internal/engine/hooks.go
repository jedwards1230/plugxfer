package engine

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func (c *Converter) hooks(file model.File, from model.Dialect) (model.File, []model.Finding, error) {
	var root map[string]any
	if err := json.Unmarshal(file.Data, &root); err != nil {
		return model.File{}, nil, fmt.Errorf("%s: parse hooks: %w", file.Path, err)
	}
	events, ok := root["hooks"].(map[string]any)
	if !ok {
		return model.File{}, nil, fmt.Errorf("%s: hooks object is required", file.Path)
	}
	converted := map[string]any{}
	var findings []model.Finding
	eventNames := make([]string, 0, len(events))
	for event := range events {
		eventNames = append(eventNames, event)
	}
	sort.Strings(eventNames)
	for _, event := range eventNames {
		rawGroups := events[event]
		if mapped, present := c.Rules.HookEvents.Lookup(from, event); present && mapped == "" {
			findings = append(findings, model.Finding{Component: "hooks", Status: model.Dropped, Path: file.Path, Class: model.Loss, Severity: model.High, Message: fmt.Sprintf("hook event %s has no %s equivalent", event, from.Other())})
			continue
		}
		groups, ok := rawGroups.([]any)
		if !ok {
			return model.File{}, nil, fmt.Errorf("%s: event %s must be an array", file.Path, event)
		}
		var outGroups []any
		for _, rawGroup := range groups {
			group, ok := rawGroup.(map[string]any)
			if !ok {
				return model.File{}, nil, fmt.Errorf("%s: hook group must be an object", file.Path)
			}
			outGroup := cloneMap(group)
			handlers, ok := group["hooks"].([]any)
			if !ok {
				return model.File{}, nil, fmt.Errorf("%s: hook group handlers must be an array", file.Path)
			}
			var outHandlers []any
			for _, rawHandler := range handlers {
				handler, ok := rawHandler.(map[string]any)
				if !ok {
					return model.File{}, nil, fmt.Errorf("%s: hook handler must be an object", file.Path)
				}
				typeName, _ := handler["type"].(string)
				if from == model.Claude && typeName != "command" {
					findings = append(findings, model.Finding{Component: "hooks", Status: model.Dropped, Path: file.Path, Class: model.Loss, Severity: model.High, Message: fmt.Sprintf("Codex does not execute %q hooks", typeName)})
					continue
				}
				outHandler := cloneMap(handler)
				if from == model.Claude {
					if async, _ := outHandler["async"].(bool); async {
						delete(outHandler, "async")
						findings = append(findings, transformed("hooks", file.Path, "stripped async:true so Codex executes the command hook"))
					}
				} else if command, ok := outHandler["command"].(string); ok {
					outHandler["command"] = strings.ReplaceAll(command, "${PLUGIN_ROOT}", "${CLAUDE_PLUGIN_ROOT}")
				}
				outHandlers = append(outHandlers, outHandler)
			}
			if len(outHandlers) == 0 {
				continue
			}
			outGroup["hooks"] = outHandlers
			if from == model.Claude && (event == "UserPromptSubmit" || event == "Stop") {
				if _, exists := outGroup["matcher"]; exists {
					delete(outGroup, "matcher")
					findings = append(findings, transformed("hooks", file.Path, "removed matcher ignored by Codex for "+event))
				}
			}
			outGroups = append(outGroups, outGroup)
		}
		if len(outGroups) > 0 {
			converted[event] = outGroups
		}
	}
	root["hooks"] = converted
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return model.File{}, nil, err
	}
	file.Data = append(data, '\n')
	findings = append(findings, transformed("hooks", file.Path, "filtered hooks for target runtime"))
	return file, findings, nil
}

func cloneMap(source map[string]any) map[string]any {
	out := make(map[string]any, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}
