package detect

import (
	"testing"

	"github.com/jedwards1230/plugxfer/internal/model"
	"github.com/jedwards1230/plugxfer/internal/rulebook"
)

func TestMarkdownIsFenceAware(t *testing.T) {
	book, err := rulebook.Load("")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("!`live`\n```text\n!`ignored`\n```\n@file.txt\n")
	findings := Markdown("command.md", data, model.Codex, book)
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2: %#v", len(findings), findings)
	}
	if findings[0].Line != 1 || findings[0].Class != model.Activation || findings[0].Severity != model.High {
		t.Fatalf("unexpected first finding: %#v", findings[0])
	}
	if findings[1].Line != 5 {
		t.Fatalf("unexpected import line: %#v", findings[1])
	}
}

func TestMarkdownKeepsFenceTypeAndLength(t *testing.T) {
	book, err := rulebook.Load("")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("````md\n~~~\n!`ignored`\n```\n!`still ignored`\n````\n!`live`\n")
	findings := Markdown("command.md", data, model.Codex, book)
	if len(findings) != 1 || findings[0].Line != 7 {
		t.Fatalf("findings = %#v", findings)
	}
}
