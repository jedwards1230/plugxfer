package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/model"
)

var exactEnv = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
var bearerEnv = regexp.MustCompile(`^Bearer \$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
var anyEnv = regexp.MustCompile(`\$\{[^}]+\}`)

func (c *Converter) mcp(file model.File, from model.Dialect) (model.File, []model.Finding, error) {
	var root map[string]any
	if err := json.Unmarshal(file.Data, &root); err != nil {
		return model.File{}, nil, fmt.Errorf("%s: parse MCP config: %w", file.Path, err)
	}
	servers, ok := root["mcpServers"].(map[string]any)
	if !ok {
		return model.File{}, nil, fmt.Errorf("%s: mcpServers object is required", file.Path)
	}
	var findings []model.Finding
	serverNames := make([]string, 0, len(servers))
	for name := range servers {
		serverNames = append(serverNames, name)
	}
	sort.Strings(serverNames)
	for _, name := range serverNames {
		server, ok := servers[name].(map[string]any)
		if !ok {
			return model.File{}, nil, fmt.Errorf("%s: MCP server %s must be an object", file.Path, name)
		}
		if from == model.Claude {
			findings = append(findings, c.mcpToCodex(file.Path, name, server)...)
		} else {
			findings = append(findings, c.mcpToClaude(file.Path, name, server)...)
		}
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return model.File{}, nil, err
	}
	file.Data = append(data, '\n')
	findings = append(findings, transformed("mcp", file.Path, "rewrote MCP environment semantics"))
	return file, findings, nil
}

func (c *Converter) mcpToCodex(path, name string, server map[string]any) []model.Finding {
	var findings []model.Finding
	var envVars []string
	if current, ok := server["env_vars"].([]any); ok {
		for _, value := range current {
			if text, ok := value.(string); ok {
				envVars = append(envVars, text)
			}
		}
	}
	if env, ok := server["env"].(map[string]any); ok {
		for key, raw := range env {
			value, ok := raw.(string)
			if !ok {
				continue
			}
			if match := exactEnv.FindStringSubmatch(value); match != nil {
				envVars = append(envVars, match[1])
				delete(env, key)
				findings = append(findings, transformed("mcp", path, fmt.Sprintf("mapped %s env passthrough to env_vars", key)))
				continue
			}
			findings = append(findings, c.unresolvedMCP(path, fmt.Sprintf("mcpServers.%s.env.%s", name, key), &env, key, value)...)
		}
		if len(env) == 0 {
			delete(server, "env")
		}
	}
	if len(envVars) > 0 {
		sort.Strings(envVars)
		server["env_vars"] = unique(envVars)
	}
	if headers, ok := server["headers"].(map[string]any); ok {
		envHeaders := map[string]any{}
		if current, ok := server["env_http_headers"].(map[string]any); ok {
			for key, value := range current {
				envHeaders[key] = value
			}
		}
		for key, raw := range headers {
			value, ok := raw.(string)
			if !ok {
				continue
			}
			if strings.EqualFold(key, "Authorization") {
				if match := bearerEnv.FindStringSubmatch(value); match != nil {
					server["bearer_token_env_var"] = match[1]
					delete(headers, key)
					findings = append(findings, transformed("mcp", path, "mapped bearer header to bearer_token_env_var"))
					continue
				}
			}
			if match := exactEnv.FindStringSubmatch(value); match != nil {
				envHeaders[key] = match[1]
				delete(headers, key)
				findings = append(findings, transformed("mcp", path, fmt.Sprintf("mapped %s header interpolation to env_http_headers", key)))
				continue
			}
			findings = append(findings, c.unresolvedMCP(path, fmt.Sprintf("mcpServers.%s.headers.%s", name, key), &headers, key, value)...)
		}
		if len(envHeaders) > 0 {
			server["env_http_headers"] = envHeaders
		}
		if len(headers) > 0 {
			server["http_headers"] = headers
		}
		delete(server, "headers")
	}
	return findings
}

func (c *Converter) mcpToClaude(path, name string, server map[string]any) []model.Finding {
	var findings []model.Finding
	env, _ := server["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	switch values := server["env_vars"].(type) {
	case []any:
		for _, raw := range values {
			if value, ok := raw.(string); ok {
				env[value] = "${" + value + "}"
			}
		}
	case []string:
		for _, value := range values {
			env[value] = "${" + value + "}"
		}
	}
	if len(env) > 0 {
		server["env"] = env
	}
	if _, ok := server["env_vars"]; ok {
		delete(server, "env_vars")
		findings = append(findings, transformed("mcp", path, "mapped env_vars to Claude env interpolation"))
	}
	if token, ok := server["bearer_token_env_var"].(string); ok {
		headers, _ := server["headers"].(map[string]any)
		if headers == nil {
			headers = map[string]any{}
		}
		headers["Authorization"] = "Bearer ${" + token + "}"
		server["headers"] = headers
		delete(server, "bearer_token_env_var")
		findings = append(findings, transformed("mcp", path, "mapped bearer_token_env_var to Authorization header"))
	}
	if httpHeaders, ok := server["http_headers"].(map[string]any); ok {
		headers, _ := server["headers"].(map[string]any)
		if headers == nil {
			headers = map[string]any{}
		}
		collision := false
		for key, value := range httpHeaders {
			// A bearer_token_env_var already materialized Authorization above;
			// keep it rather than let a static http_headers entry silently
			// clobber it (previously the whole headers map was overwritten).
			if _, exists := headers[key]; exists {
				collision = true
				continue
			}
			headers[key] = value
		}
		if len(headers) > 0 {
			server["headers"] = headers
		}
		delete(server, "http_headers")
		msg := "mapped http_headers to Claude headers"
		if collision {
			msg = "mapped http_headers to Claude headers; kept existing header on collision"
		}
		findings = append(findings, transformed("mcp", path, msg))
	}
	if envHeaders, ok := server["env_http_headers"].(map[string]any); ok {
		headers, _ := server["headers"].(map[string]any)
		if headers == nil {
			headers = map[string]any{}
		}
		for key, raw := range envHeaders {
			if value, ok := raw.(string); ok {
				headers[key] = "${" + value + "}"
			}
		}
		server["headers"] = headers
		delete(server, "env_http_headers")
		findings = append(findings, transformed("mcp", path, "mapped env_http_headers to Claude header interpolation"))
	}
	_ = name
	return findings
}

func (c *Converter) unresolvedMCP(path, jsonPath string, target *map[string]any, key, value string) []model.Finding {
	if !anyEnv.MatchString(value) {
		return nil
	}
	locator := path + "#" + jsonPath
	if replacement, ok := c.Answers.Replacements[locator]; ok && replacement != "" {
		(*target)[key] = replacement
		return []model.Finding{transformed("mcp", path, "applied mapped replacement for "+jsonPath)}
	}
	c.Answers.StubReplacement(locator)
	return []model.Finding{{Component: "mcp", Status: model.NeedsMap, Path: path, Class: model.Loss, Severity: model.High, Message: "Codex cannot interpolate " + value + " at " + jsonPath, Fix: "set the replacement in plugxfer.map.yaml"}}
}

func unique(values []string) []string {
	out := values[:0]
	var previous string
	for i, value := range values {
		if i == 0 || value != previous {
			out = append(out, value)
			previous = value
		}
	}
	return out
}
