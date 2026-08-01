package report

import (
	"strings"
	"testing"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func TestMarkdownIncludesLocationsAndEnvironmentNotes(t *testing.T) {
	r := model.Report{Source: model.Claude, Target: model.Codex, Input: "in", Output: "out", Findings: []model.Finding{{Component: "hooks", Status: model.Dropped, Path: "hooks/hooks.json", Line: 4, Class: model.Loss, Severity: model.High, Message: "prompt hook dropped"}}}
	data := string(Markdown(r))
	for _, want := range []string{"hooks/hooks.json:4", "prompt hook dropped", "trust-gated", "never executed plugin scripts"} {
		if !strings.Contains(data, want) {
			t.Errorf("report missing %q:\n%s", want, data)
		}
	}
}
