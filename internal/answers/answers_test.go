package answers

import (
	"strings"
	"testing"
)

func TestMarshalStubs(t *testing.T) {
	answer := Empty()
	answer.StubModel("claude-to-codex", "sonnet")
	answer.StubReplacement(".mcp.json#x")
	if !answer.HasStubs() {
		t.Fatal("expected stubs")
	}
	data, err := answer.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"version: 1", "claude-to-codex:", "sonnet: \"\"", ".mcp.json#x: \"\""} {
		if !strings.Contains(string(data), want) {
			t.Errorf("marshal missing %q:\n%s", want, data)
		}
	}
}
