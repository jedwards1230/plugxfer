package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/detect"
	"github.com/jedwards1230/plugxfer/internal/frontmatter"
	"github.com/jedwards1230/plugxfer/internal/model"
)

var exactImport = regexp.MustCompile(`^\s*@([^\s@]+)\s*$`)

var (
	inlineShellActivation = regexp.MustCompile("!`")
	leadingAtActivation   = regexp.MustCompile(`(?m)^(\s*)@`)
	restoreBinCall        = regexp.MustCompile(`(?m)^(\s*)\$\{(?:CLAUDE_)?PLUGIN_ROOT\}/bin/([^\s]+)(\s|$)`)
)

// binRewriter holds the per-bin call-site patterns compiled once per conversion
// so a plugin with many Markdown files does not recompile them per file.
type binRewriter struct {
	names    []string
	patterns []*regexp.Regexp
}

func newBinRewriter(names []string) binRewriter {
	patterns := make([]*regexp.Regexp, len(names))
	for i, name := range names {
		patterns[i] = regexp.MustCompile(`(?m)^(\s*)(` + regexp.QuoteMeta(name) + `)(\s|$)`)
	}
	return binRewriter{names: names, patterns: patterns}
}

func (b binRewriter) empty() bool { return len(b.names) == 0 }

// rewrite rewrites bare bundled-bin call sites to ${PLUGIN_ROOT}/bin/<name>.
func (b binRewriter) rewrite(data []byte) []byte {
	text := string(data)
	for i, name := range b.names {
		text = b.patterns[i].ReplaceAllString(text, `${1}$${PLUGIN_ROOT}/bin/`+name+`${3}`)
	}
	return []byte(text)
}

func (c *Converter) commandToSkill(plugin *model.Plugin, file model.File, bins binRewriter) (model.File, []model.File, []model.Finding, error) {
	doc, err := frontmatter.Parse(file.Data, false)
	if err != nil {
		return model.File{}, nil, nil, fmt.Errorf("%s: %w", file.Path, err)
	}
	name := strings.TrimSuffix(filepath.Base(file.Path), filepath.Ext(file.Path))
	description := frontmatter.String(doc.Fields, "description")
	if description == "" {
		description = firstContentLine(doc.Body, "Converted Claude command")
	}
	body, materialized := materializeImports(plugin.Root, doc.Body)
	body = string(bins.rewrite([]byte(body)))
	fields := map[string]any{"name": name, "description": description, "plugxfer-origin": "command"}
	for _, key := range []string{"allowed-tools", "argument-hint", "disable-model-invocation"} {
		if value, ok := doc.Fields[key]; ok {
			fields[key] = value
		}
	}
	if modelValue := frontmatter.String(doc.Fields, "model"); modelValue != "" {
		fields["plugxfer-source-model"] = modelValue
	}
	outData, err := frontmatter.Render(frontmatter.Document{Fields: fields, Body: "## Command Template\n\n" + body})
	if err != nil {
		return model.File{}, nil, nil, err
	}
	out := model.File{Path: "skills/" + name + "/SKILL.md", Mode: file.Mode, Data: outData}
	findings := detect.Markdown(file.Path, file.Data, model.Claude, c.Rules)
	var extras []model.File
	findings = append(findings, transformed("commands", file.Path, "folded command into a Codex skill"))
	for _, path := range materialized {
		findings = append(findings, model.Finding{Component: "content", Status: model.Transformed, Path: file.Path, Class: model.Loss, Severity: model.Medium, Message: "materialized @ import " + path, Fix: "embedded referenced file in command template"})
	}
	for _, key := range []string{"allowed-tools", "argument-hint", "model"} {
		if value, ok := doc.Fields[key]; ok {
			findings = append(findings, model.Finding{Component: "commands", Status: model.Dropped, Path: file.Path, Class: model.Loss, Severity: model.Medium, Message: fmt.Sprintf("Codex skills do not honor command field %q value %q", key, value)})
		}
	}
	if frontmatter.Bool(doc.Fields, "disable-model-invocation") {
		extras = append(extras, model.File{Path: "skills/" + name + "/agents/openai.yaml", Mode: 0o644, Data: []byte("policy:\n  allow_implicit_invocation: false\n")})
		findings = append(findings, transformed("commands", file.Path, "mapped disable-model-invocation to agents/openai.yaml"))
	}
	return out, extras, findings, err
}

func (c *Converter) skill(file model.File, from model.Dialect, policy bool) (model.File, []model.File, []model.Finding, *model.File, error) {
	doc, err := frontmatter.Parse(file.Data, from == model.Codex)
	if err != nil && from == model.Claude {
		doc = frontmatter.Document{Fields: map[string]any{}, Body: string(file.Data)}
	} else if err != nil {
		return model.File{}, nil, nil, nil, fmt.Errorf("%s: %w", file.Path, err)
	}
	name := filepath.Base(filepath.Dir(file.Path))
	if frontmatter.String(doc.Fields, "name") == "" {
		doc.Fields["name"] = name
	}
	if frontmatter.String(doc.Fields, "description") == "" {
		doc.Fields["description"] = firstContentLine(doc.Body, "Converted skill")
	}
	findings := detect.Markdown(file.Path, file.Data, from, c.Rules)
	var extras []model.File
	if from == model.Claude {
		ignored := []string{"allowed-tools", "user-invocable", "argument-hint", "model", "context", "agent", "hooks"}
		for _, key := range ignored {
			if _, ok := doc.Fields[key]; ok {
				findings = append(findings, model.Finding{Component: "skills", Status: model.Dropped, Path: file.Path, Class: model.Loss, Severity: model.Medium, Message: fmt.Sprintf("Codex ignores skill frontmatter field %q", key)})
			}
		}
		data, renderErr := frontmatter.Render(doc)
		if renderErr != nil {
			return model.File{}, nil, nil, nil, renderErr
		}
		file.Data = data
		if frontmatter.Bool(doc.Fields, "disable-model-invocation") {
			sidecar := model.File{Path: filepath.ToSlash(filepath.Join(filepath.Dir(file.Path), "agents", "openai.yaml")), Mode: 0o644, Data: []byte("policy:\n  allow_implicit_invocation: false\n")}
			extras = append(extras, sidecar)
			findings = append(findings, transformed("skills", file.Path, "mapped disable-model-invocation to agents/openai.yaml"))
		}
		findings = append(findings, model.Finding{Component: "skills", Status: model.Copied, Path: file.Path, Message: "copied skill with normalized required frontmatter"})
		return file, extras, findings, nil, nil
	}
	if origin := frontmatter.String(doc.Fields, "plugxfer-origin"); origin == "command" {
		delete(doc.Fields, "name")
		delete(doc.Fields, "plugxfer-origin")
		if value := frontmatter.String(doc.Fields, "plugxfer-source-model"); value != "" {
			doc.Fields["model"] = value
			delete(doc.Fields, "plugxfer-source-model")
		}
		body := strings.TrimLeft(doc.Body, "\n")
		body = strings.TrimPrefix(body, "## Command Template\n\n")
		body = dematerializeImports(body)
		body = restoreBinCalls(body)
		data, renderErr := frontmatter.Render(frontmatter.Document{Fields: doc.Fields, Body: body})
		if renderErr != nil {
			return model.File{}, nil, nil, nil, renderErr
		}
		command := &model.File{Path: "commands/" + name + ".md", Mode: file.Mode, Data: data}
		findings = append(findings, transformed("commands", file.Path, "restored plugxfer-generated command skill"))
		return model.File{}, nil, findings, command, nil
	}
	if policy {
		doc.Fields["disable-model-invocation"] = true
		findings = append(findings, transformed("skills", file.Path, "mapped openai.yaml invocation policy to frontmatter"))
	}
	data, renderErr := frontmatter.Render(doc)
	if renderErr != nil {
		return model.File{}, nil, nil, nil, renderErr
	}
	file.Data = data
	findings = append(findings, model.Finding{Component: "skills", Status: model.Copied, Path: file.Path, Message: "copied skill"})
	return file, nil, findings, nil, nil
}

func skillPolicies(plugin *model.Plugin) map[string]bool {
	policies := map[string]bool{}
	for _, file := range plugin.Files {
		if strings.HasSuffix(file.Path, "/agents/openai.yaml") && strings.Contains(string(file.Data), "allow_implicit_invocation: false") {
			policies[strings.TrimSuffix(file.Path, "agents/openai.yaml")+"SKILL.md"] = true
		}
	}
	return policies
}

func materializeImports(root, body string) (string, []string) {
	lines := strings.Split(body, "\n")
	var imported []string
	for i, line := range lines {
		match := exactImport.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		rel := filepath.Clean(filepath.FromSlash(match[1]))
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || len(data) > 1<<20 {
			continue
		}
		fence := "```"
		if strings.Contains(string(data), "```") {
			fence = "````"
		}
		lines[i] = fmt.Sprintf("<!-- plugxfer: materialized @%s -->\n\n### Imported `%s`\n\n%s\n%s\n%s", match[1], match[1], fence, strings.TrimSuffix(string(data), "\n"), fence)
		imported = append(imported, match[1])
	}
	return strings.Join(lines, "\n"), imported
}

func dematerializeImports(body string) string {
	lines := strings.Split(body, "\n")
	for i := 0; i < len(lines); i++ {
		const prefix = "<!-- plugxfer: materialized @"
		if !strings.HasPrefix(lines[i], prefix) || !strings.HasSuffix(lines[i], " -->") {
			continue
		}
		path := strings.TrimSuffix(strings.TrimPrefix(lines[i], prefix), " -->")
		open := -1
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "```") {
				open = j
				break
			}
		}
		if open < 0 {
			continue
		}
		close := -1
		for j := open + 1; j < len(lines); j++ {
			if lines[j] == lines[open] {
				close = j
				break
			}
		}
		if close < 0 {
			continue
		}
		lines = append(lines[:i], append([]string{"@" + path}, lines[close+1:]...)...)
	}
	return strings.Join(lines, "\n")
}

func restoreBinCalls(body string) string {
	return restoreBinCall.ReplaceAllString(body, `${1}${2}${3}`)
}

func escapeClaudeActivation(body string) string {
	body = inlineShellActivation.ReplaceAllString(body, "\\!`")
	body = leadingAtActivation.ReplaceAllString(body, `${1}\@`)
	return body
}

func firstContentLine(body, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		if text := strings.TrimSpace(strings.TrimLeft(line, "#")); text != "" {
			return text
		}
	}
	return fallback
}

func needsModel(path, value string) model.Finding {
	return model.Finding{Component: "models", Status: model.NeedsMap, Path: path, Class: model.Loss, Severity: model.High, Message: fmt.Sprintf("model %q has no deterministic target mapping", value), Fix: "set the value in plugxfer.map.yaml"}
}
