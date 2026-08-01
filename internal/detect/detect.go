package detect

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/model"
	"github.com/jedwards1230/plugxfer/internal/rulebook"
)

func Markdown(path string, data []byte, from model.Dialect, book *rulebook.Book) []model.Finding {
	lines := bytes.Split(data, []byte("\n"))
	fence := ""
	var findings []model.Finding
	for i, raw := range lines {
		line := string(raw)
		trimmed := strings.TrimSpace(line)
		if marker := fenceMarker(trimmed); marker != "" {
			if fence == "" {
				fence = marker
			} else if marker[0] == fence[0] && len(marker) >= len(fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		for _, detector := range book.Detectors {
			if detector.ID == "codex-placeholder" && from == model.Claude {
				continue
			}
			if detector.ID == "inline-at-reference" && strings.HasPrefix(trimmed, "@") && !strings.ContainsAny(strings.TrimPrefix(trimmed, "@"), " \t") {
				continue
			}
			if !detector.Regexp.MatchString(line) {
				continue
			}
			class := detector.ClaudeToCodex
			if from == model.Codex {
				class = detector.CodexToClaude
			}
			finding := model.Finding{
				Component: "content", Status: model.Note, Path: path, Line: i + 1,
				Class: model.Class(class), Severity: model.Severity(detector.Severity),
				Message: message(detector.ID, from),
			}
			if detector.ID == "at-import" && from == model.Claude {
				finding.Fix = "materialize relative import when it is an exact line"
			}
			if detector.ID == "inline-shell" && from == model.Codex {
				finding.Fix = "escape activation in Claude command output"
			}
			findings = append(findings, finding)
		}
	}
	return findings
}

func fenceMarker(line string) string {
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return ""
	}
	i := 1
	for i < len(line) && line[i] == line[0] {
		i++
	}
	if i < 3 {
		return ""
	}
	return line[:i]
}

func message(id string, from model.Dialect) string {
	direction := fmt.Sprintf("%s-to-%s", from, from.Other())
	switch id {
	case "at-import":
		return "@ import changes semantics during " + direction
	case "inline-at-reference":
		return "possible inline @ reference requires semantic review"
	case "inline-shell":
		return "inline shell syntax changes semantics during " + direction
	case "codex-placeholder":
		return "Codex prompt placeholder has no equivalent substitution"
	case "claude-arguments":
		return "Claude command argument substitution has no Codex skill equivalent"
	case "handlebars":
		return "handlebars-style syntax is unsupported"
	case "plugin-root-in-markdown":
		return "plugin-root variables are inert in Markdown bodies"
	default:
		return "content syntax requires review"
	}
}
