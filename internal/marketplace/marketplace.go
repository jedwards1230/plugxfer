package marketplace

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/model"
)

type Entry struct {
	Name   string
	Source any
	Raw    map[string]any
}

type Registry struct {
	Dialect model.Dialect
	Path    string
	Raw     map[string]any
	Entries []Entry
}

func Parse(file model.File, dialect model.Dialect) (*Registry, error) {
	var raw map[string]any
	if err := json.Unmarshal(file.Data, &raw); err != nil {
		return nil, fmt.Errorf("%s: parse marketplace: %w", file.Path, err)
	}
	plugins, ok := raw["plugins"].([]any)
	if !ok {
		return nil, fmt.Errorf("%s: plugins array is required", file.Path)
	}
	r := &Registry{Dialect: dialect, Path: file.Path, Raw: raw}
	seen := map[string]bool{}
	for _, value := range plugins {
		plugin, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s: plugin entry must be an object", file.Path)
		}
		name, _ := plugin["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("%s: plugin entry name is required", file.Path)
		}
		if seen[name] {
			return nil, fmt.Errorf("%s: duplicate plugin %q", file.Path, name)
		}
		seen[name] = true
		source, ok := plugin["source"]
		if !ok {
			return nil, fmt.Errorf("%s: plugin %q source is required", file.Path, name)
		}
		r.Entries = append(r.Entries, Entry{Name: name, Source: source, Raw: plugin})
	}
	sort.Slice(r.Entries, func(i, j int) bool { return r.Entries[i].Name < r.Entries[j].Name })
	return r, nil
}

func (e Entry) LocalPath() (string, bool, error) {
	var path string
	switch source := e.Source.(type) {
	case string:
		if strings.Contains(source, "://") || strings.HasPrefix(source, "git@") {
			return "", false, nil
		}
		path = source
	case map[string]any:
		typeName, _ := source["source"].(string)
		if typeName != "local" {
			return "", false, nil
		}
		path, _ = source["path"].(string)
		if path == "" {
			path, _ = source["url"].(string)
		}
	default:
		return "", false, fmt.Errorf("plugin %q has unsupported source shape", e.Name)
	}
	if path == "" {
		return "", false, fmt.Errorf("plugin %q has an empty local source", e.Name)
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("plugin %q local source escapes marketplace root", e.Name)
	}
	return filepath.ToSlash(clean), true, nil
}

func (r *Registry) Render(target model.Dialect) (model.File, error) {
	out := map[string]any{}
	for _, key := range []string{"name", "description", "version"} {
		if value, ok := r.Raw[key]; ok {
			out[key] = value
		}
	}
	if target == model.Claude {
		if value, ok := r.Raw["owner"]; ok {
			out["owner"] = value
		}
		if value, ok := r.Raw["renames"]; ok {
			out["renames"] = value
		}
	}
	plugins := make([]any, 0, len(r.Entries))
	for _, entry := range r.Entries {
		converted := map[string]any{"name": entry.Name, "source": entry.Source}
		if target == model.Codex {
			for _, key := range []string{"displayName", "description", "author", "category", "version", "homepage", "license", "keywords", "tags"} {
				if value, ok := entry.Raw[key]; ok {
					converted[key] = value
				}
			}
			if policy, ok := entry.Raw["policy"]; ok {
				converted["policy"] = policy
			} else {
				converted["policy"] = map[string]any{"installation": "AVAILABLE", "authentication": "ON_USE", "products": []string{"CODEX"}}
			}
		} else {
			for _, key := range []string{"displayName", "description", "author", "category", "version", "homepage", "license", "keywords", "tags", "strict", "skills", "commands", "agents", "mcpServers", "lspServers"} {
				if value, ok := entry.Raw[key]; ok {
					converted[key] = value
				}
			}
			if _, ok := converted["description"]; !ok {
				converted["description"] = entry.Name
			}
			if _, ok := converted["category"]; !ok {
				converted["category"] = "other"
			}
		}
		plugins = append(plugins, converted)
	}
	out["plugins"] = plugins
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return model.File{}, err
	}
	path := ".agents/plugins/marketplace.json"
	if target == model.Claude {
		path = ".claude-plugin/marketplace.json"
	}
	return model.File{Path: path, Mode: 0o644, Data: append(data, '\n')}, nil
}
