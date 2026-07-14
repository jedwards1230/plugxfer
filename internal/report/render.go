package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func Markdown(r model.Report) []byte {
	var out strings.Builder
	out.WriteString("# plugxfer conversion report\n\n")
	fmt.Fprintf(&out, "- Source: `%s`\n- Target: `%s`\n- Input: `%s`\n", r.Source, r.Target, escape(r.Input))
	if r.Output != "" {
		fmt.Fprintf(&out, "- Output: `%s`\n", escape(r.Output))
	}
	fmt.Fprintf(&out, "- Mode: `%s`\n\n", mode(r.Marketplace))

	if len(r.Children) > 0 {
		out.WriteString("## Plugins\n\n| Plugin | Source | Result | Report |\n|---|---|---:|---|\n")
		children := append([]model.ChildReport(nil), r.Children...)
		sort.Slice(children, func(i, j int) bool { return children[i].Name < children[j].Name })
		for _, child := range children {
			link := "-"
			if child.ReportPath != "" {
				link = fmt.Sprintf("[%s](%s)", child.ReportPath, child.ReportPath)
			}
			fmt.Fprintf(&out, "| %s | `%s` | %d | %s |\n", cell(child.Name), escape(child.Source), child.ExitCode, link)
		}
		out.WriteByte('\n')
	}

	counts := r.Counts()
	for _, child := range r.Children {
		for status, count := range child.Counts {
			counts[status] += count
		}
	}
	out.WriteString("## Summary\n\n| Status | Count |\n|---|---:|\n")
	for _, status := range []model.Status{model.Copied, model.Transformed, model.Dropped, model.NeedsMap, model.Skipped, model.Note} {
		fmt.Fprintf(&out, "| %s | %d |\n", status, counts[status])
	}
	out.WriteByte('\n')

	out.WriteString("## Details\n\n")
	if len(r.Findings) == 0 {
		if len(r.Children) > 0 {
			out.WriteString("No marketplace-level findings. See per-plugin reports for component details.\n")
		} else {
			out.WriteString("No components were discovered.\n")
		}
	} else {
		out.WriteString("| Status | Component | Location | Class | Severity | Detail | Applied fix |\n|---|---|---|---|---|---|---|\n")
		for _, f := range r.Findings {
			fmt.Fprintf(&out, "| %s | %s | `%s` | %s | %s | %s | %s |\n", f.Status, cell(f.Component), escape(f.Location()), empty(string(f.Class)), empty(string(f.Severity)), cell(f.Message), cell(empty(f.Fix)))
		}
	}
	out.WriteString("\n## Environment notes\n\n")
	if r.Target == model.Codex {
		out.WriteString("- Codex plugin hooks are trust-gated and must be enabled and approved before they run.\n- Converted agents are emitted under project `.codex/agents/`; Codex plugins cannot bundle agent roles.\n")
	}
	out.WriteString("- plugxfer never executed plugin scripts, hooks, or MCP commands during this run.\n")
	return []byte(out.String())
}

func mode(marketplace bool) string {
	if marketplace {
		return "marketplace"
	}
	return "plugin"
}
func empty(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
func escape(value string) string { return strings.ReplaceAll(value, "`", "'") }
func cell(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "|", "\\|"), "\n", " ")
}
