package frontmatter

import (
	"strings"
	"testing"
)

func TestParseAndRender(t *testing.T) {
	doc, err := Parse([]byte("---\ndescription: Test\nname: sample\n---\n\nBody\n"), true)
	if err != nil {
		t.Fatal(err)
	}
	if String(doc.Fields, "name") != "sample" || doc.Body != "\nBody\n" {
		t.Fatalf("unexpected document: %#v", doc)
	}
	data, err := Render(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "---\nname: sample\ndescription: Test\n---\n") {
		t.Fatalf("unexpected rendering:\n%s", data)
	}
}

func TestParseRequiresFrontmatter(t *testing.T) {
	if _, err := Parse([]byte("Body\n"), true); err == nil {
		t.Fatal("expected missing-frontmatter error")
	}
}

func TestParseLenientPlainScalarWithColon(t *testing.T) {
	doc, err := Parse([]byte("---\nname: agent\ndescription: Use when needed. Example: do the thing\ntools: [Read, Grep]\n---\n\nBody\n"), true)
	if err != nil {
		t.Fatal(err)
	}
	if got := String(doc.Fields, "description"); got != "Use when needed. Example: do the thing" {
		t.Fatalf("description = %q", got)
	}
	tools, ok := doc.Fields["tools"].([]any)
	if !ok || len(tools) != 2 {
		t.Fatalf("tools = %#v", doc.Fields["tools"])
	}
}

func TestParseLenientMultilineDescription(t *testing.T) {
	data := "---\nname: agent\ndescription: Examples:\n\n<example>\nContext: create an agent\nuser: do it\n</example>\n\nmodel: sonnet\ntools: [Read]\n---\n\nBody\n"
	doc, err := Parse([]byte(data), true)
	if err != nil {
		t.Fatal(err)
	}
	description := String(doc.Fields, "description")
	if !strings.Contains(description, "Context: create an agent") || !strings.Contains(description, "user: do it") {
		t.Fatalf("description = %q", description)
	}
	if String(doc.Fields, "model") != "sonnet" {
		t.Fatalf("model = %#v", doc.Fields["model"])
	}
}
