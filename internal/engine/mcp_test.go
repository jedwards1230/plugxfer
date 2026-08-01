package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jedwards1230/plugxfer/internal/answers"
	"github.com/jedwards1230/plugxfer/internal/model"
)

// TestMCPToClaudeMergesBearerAndHTTPHeaders covers a Codex HTTP server that
// carries BOTH bearer_token_env_var and http_headers. The bearer materializes
// an Authorization header and http_headers must merge into the same map rather
// than replace it (which previously dropped Authorization silently).
func TestMCPToClaudeMergesBearerAndHTTPHeaders(t *testing.T) {
	tests := []struct {
		name       string
		server     map[string]any
		wantHeader map[string]string
		absent     []string
	}{
		{
			name: "bearer and disjoint http_headers coexist",
			server: map[string]any{
				"type":                 "http",
				"url":                  "https://example.test/mcp",
				"bearer_token_env_var": "REMOTE_TOKEN",
				"http_headers":         map[string]any{"X-Api-Version": "2"},
			},
			wantHeader: map[string]string{
				"Authorization": "Bearer ${REMOTE_TOKEN}",
				"X-Api-Version": "2",
			},
			absent: []string{"bearer_token_env_var", "http_headers"},
		},
		{
			name: "bearer wins over colliding http_headers Authorization",
			server: map[string]any{
				"type":                 "http",
				"url":                  "https://example.test/mcp",
				"bearer_token_env_var": "REMOTE_TOKEN",
				"http_headers":         map[string]any{"Authorization": "Bearer static", "X-Api-Key": "k"},
			},
			wantHeader: map[string]string{
				"Authorization": "Bearer ${REMOTE_TOKEN}",
				"X-Api-Key":     "k",
			},
			absent: []string{"bearer_token_env_var", "http_headers"},
		},
		{
			name: "http_headers only, no bearer",
			server: map[string]any{
				"type":         "http",
				"url":          "https://example.test/mcp",
				"http_headers": map[string]any{"X-Api-Key": "k"},
			},
			wantHeader: map[string]string{"X-Api-Key": "k"},
			absent:     []string{"http_headers"},
		},
	}
	c := &Converter{Answers: answers.Empty()}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			findings := c.mcpToClaude("in/.mcp.json", "remote", test.server)
			headers, ok := test.server["headers"].(map[string]any)
			if !ok {
				t.Fatalf("headers missing after conversion: %#v", test.server)
			}
			for key, want := range test.wantHeader {
				if got, _ := headers[key].(string); got != want {
					t.Errorf("headers[%q] = %q, want %q", key, got, want)
				}
			}
			if len(headers) != len(test.wantHeader) {
				t.Errorf("headers = %#v, want exactly %d keys", headers, len(test.wantHeader))
			}
			for _, key := range test.absent {
				if _, ok := test.server[key]; ok {
					t.Errorf("server still carries %q: %#v", key, test.server)
				}
			}
			if len(findings) == 0 {
				t.Error("expected at least one transformed finding (silence is a bug)")
			}
		})
	}
}

// TestMCPBothFieldsThroughPipeline exercises the full JSON round-trip so the
// merged Authorization header survives marshaling.
func TestMCPBothFieldsThroughPipeline(t *testing.T) {
	input := `{
  "mcpServers": {
    "remote": {
      "type": "http",
      "url": "https://example.test/mcp",
      "bearer_token_env_var": "REMOTE_TOKEN",
      "http_headers": {"X-Api-Key": "k"}
    }
  }
}`
	c := &Converter{Answers: answers.Empty()}
	out, findings, err := c.mcp(model.File{Path: ".mcp.json", Data: []byte(input)}, model.Codex)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(out.Data, &root); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.Data)
	}
	server := root["mcpServers"].(map[string]any)["remote"].(map[string]any)
	headers, ok := server["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers missing:\n%s", out.Data)
	}
	if headers["Authorization"] != "Bearer ${REMOTE_TOKEN}" {
		t.Errorf("Authorization dropped or wrong: %#v", headers)
	}
	if headers["X-Api-Key"] != "k" {
		t.Errorf("http_headers entry lost: %#v", headers)
	}
	if _, ok := server["http_headers"]; ok {
		t.Errorf("http_headers not removed:\n%s", out.Data)
	}
	if !strings.Contains(string(out.Data), "Authorization") {
		t.Errorf("serialized output missing Authorization:\n%s", out.Data)
	}
	if len(findings) == 0 {
		t.Error("expected findings from MCP conversion")
	}
}
