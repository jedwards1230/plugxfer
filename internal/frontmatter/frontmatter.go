package frontmatter

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Document struct {
	Fields map[string]any
	Body   string
}

func Parse(data []byte, required bool) (Document, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		if required {
			return Document{}, fmt.Errorf("missing YAML frontmatter")
		}
		return Document{Fields: map[string]any{}, Body: text}, nil
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return Document{}, fmt.Errorf("unterminated YAML frontmatter")
	}
	end += 4
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(text[4:end]), &fields); err != nil {
		var fallbackErr error
		fields, fallbackErr = parseLenient(text[4:end])
		if fallbackErr != nil {
			return Document{}, fmt.Errorf("parse YAML frontmatter: %w (lenient fallback: %v)", err, fallbackErr)
		}
	}
	if fields == nil {
		fields = map[string]any{}
	}
	return Document{Fields: fields, Body: text[end+5:]}, nil
}

func parseLenient(block string) (map[string]any, error) {
	lines := strings.Split(block, "\n")
	fields := map[string]any{}
	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}
		if len(line) != len(strings.TrimLeft(line, " \t")) {
			return nil, fmt.Errorf("unexpected indentation on line %d", i+1)
		}
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			return nil, fmt.Errorf("expected key:value on line %d", i+1)
		}
		key := strings.TrimSpace(line[:colon])
		if key == "" || strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("invalid key on line %d", i+1)
		}
		raw := strings.TrimSpace(line[colon+1:])
		if key == "description" && raw != "" {
			j := i + 1
			for j < len(lines) {
				if candidate, ok := fieldLine(lines[j]); ok && !descriptionContinuation[candidate] {
					break
				}
				j++
			}
			if j > i+1 {
				raw = strings.TrimRight(raw+"\n"+strings.Join(lines[i+1:j], "\n"), "\n")
			}
			fields[key] = raw
			i = j
			continue
		}
		j := i + 1
		for j < len(lines) {
			next := lines[j]
			if strings.TrimSpace(next) == "" {
				j++
				continue
			}
			if len(next) == len(strings.TrimLeft(next, " \t")) {
				break
			}
			j++
		}
		if raw == "" && j > i+1 {
			var parsed map[string]any
			if err := yaml.Unmarshal([]byte(strings.Join(lines[i:j], "\n")), &parsed); err != nil {
				return nil, fmt.Errorf("field %q: %w", key, err)
			}
			fields[key] = parsed[key]
		} else {
			var value any
			if err := yaml.Unmarshal([]byte(raw), &value); err != nil {
				value = raw
			}
			if _, isMap := value.(map[string]any); isMap {
				value = raw
			}
			fields[key] = value
		}
		i = j
	}
	return fields, nil
}

var descriptionContinuation = map[string]bool{
	"Context":    true,
	"user":       true,
	"assistant":  true,
	"commentary": true,
}

func fieldLine(line string) (string, bool) {
	if len(line) != len(strings.TrimLeft(line, " \t")) {
		return "", false
	}
	colon := strings.IndexByte(line, ':')
	if colon <= 0 {
		return "", false
	}
	key := strings.TrimSpace(line[:colon])
	if key == "" || strings.ContainsAny(key, " \t<>") {
		return "", false
	}
	return key, true
}

func Render(doc Document) ([]byte, error) {
	keys := make([]string, 0, len(doc.Fields))
	for key := range doc.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make([]string, 0, len(keys))
	for _, preferred := range []string{"name", "description"} {
		if _, ok := doc.Fields[preferred]; ok {
			ordered = append(ordered, preferred)
		}
	}
	for _, key := range keys {
		if key != "name" && key != "description" {
			ordered = append(ordered, key)
		}
	}
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, key := range ordered {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
		var valueNode yaml.Node
		if err := valueNode.Encode(doc.Fields[key]); err != nil {
			return nil, err
		}
		value := &valueNode
		if valueNode.Kind == yaml.DocumentNode && len(valueNode.Content) == 1 {
			value = valueNode.Content[0]
		}
		mapping.Content = append(mapping.Content, keyNode, value)
	}
	var out bytes.Buffer
	out.WriteString("---\n")
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(mapping); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	out.WriteString("---\n\n")
	out.WriteString(strings.TrimLeft(doc.Body, "\n"))
	if out.Len() == 0 || out.Bytes()[out.Len()-1] != '\n' {
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

func String(fields map[string]any, key string) string {
	v, _ := fields[key].(string)
	return v
}

func Bool(fields map[string]any, key string) bool {
	v, _ := fields[key].(bool)
	return v
}
