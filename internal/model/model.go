package model

import "fmt"

type Dialect string

const (
	Claude Dialect = "claude"
	Codex  Dialect = "codex"
)

func (d Dialect) Valid() bool { return d == Claude || d == Codex }

func (d Dialect) Other() Dialect {
	if d == Claude {
		return Codex
	}
	return Claude
}

type Status string

const (
	Copied      Status = "copied"
	Transformed Status = "transformed"
	Dropped     Status = "dropped"
	NeedsMap    Status = "needs-map"
	Skipped     Status = "skipped"
	Note        Status = "note"
)

type Class string

const (
	Loss       Class = "loss"
	Activation Class = "activation"
	Possible   Class = "possible"
	Info       Class = "info"
)

type Severity string

const (
	Low    Severity = "low"
	Medium Severity = "medium"
	High   Severity = "high"
)

type Finding struct {
	Component string
	Status    Status
	Path      string
	Line      int
	Class     Class
	Severity  Severity
	Message   string
	Fix       string
}

func (f Finding) Location() string {
	if f.Path == "" {
		return "-"
	}
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d", f.Path, f.Line)
	}
	return f.Path
}

type Report struct {
	Source      Dialect
	Target      Dialect
	Input       string
	Output      string
	Marketplace bool
	Verified    *RulesVerified
	Findings    []Finding
	Children    []ChildReport
}

type ChildReport struct {
	Name       string
	Source     string
	ReportPath string
	ExitCode   int
	Counts     map[Status]int
	// Findings is populated only for check-mode marketplace runs, where
	// per-plugin reports are not written to disk and must instead be embedded
	// in the aggregate report.
	Findings []Finding
}

// SpecPin records the upstream CLI spec a rulebook was verified against. Claude
// Code is pinned by released version, Codex by source commit; both carry the
// verification date.
type SpecPin struct {
	Version string `yaml:"version,omitempty"`
	Commit  string `yaml:"commit,omitempty"`
	Date    string `yaml:"date,omitempty"`
}

// RulesVerified is the optional rulebook provenance block surfaced in report
// headers. It is absent for custom rulebooks that omit it.
type RulesVerified struct {
	ClaudeCode SpecPin `yaml:"claude_code"`
	Codex      SpecPin `yaml:"codex"`
}

// Line renders the provenance as a single human-readable string, e.g.
// "Claude Code 2.1.207, Codex @0877afbe8 (2026-07-13)".
func (v RulesVerified) Line() string {
	claude := "Claude Code " + v.ClaudeCode.Version
	codex := "Codex @" + v.Codex.Commit
	if v.ClaudeCode.Date != "" && v.ClaudeCode.Date == v.Codex.Date {
		return fmt.Sprintf("%s, %s (%s)", claude, codex, v.ClaudeCode.Date)
	}
	if v.ClaudeCode.Date != "" {
		claude += " (" + v.ClaudeCode.Date + ")"
	}
	if v.Codex.Date != "" {
		codex += " (" + v.Codex.Date + ")"
	}
	return fmt.Sprintf("%s, %s", claude, codex)
}

func (r *Report) Add(f Finding) { r.Findings = append(r.Findings, f) }

func (r Report) Counts() map[Status]int {
	counts := map[Status]int{}
	for _, finding := range r.Findings {
		counts[finding.Status]++
	}
	return counts
}

// ExitCode maps a report to the CLI exit code: 0 clean, 1 converted-with-losses,
// 2 needs-map unresolved (3 error is returned by the CLI layer, not here).
//
// By default only dropped components (1) and unresolved mappings (2) affect the
// code; compatibility warnings that are still reported — loss- and
// activation-class detector findings and lossy transforms — leave it at 0.
// --strict escalates those otherwise-advisory losses to a non-zero code so CI
// can gate on a perfectly clean conversion.
func (r Report) ExitCode(strict bool) int {
	code := 0
	bump := func(c int) {
		if c > code {
			code = c
		}
	}
	for _, f := range r.Findings {
		switch f.Status {
		case NeedsMap:
			bump(2)
		case Dropped:
			bump(1)
		}
		if strict && (f.Class == Loss || f.Class == Activation) {
			bump(1)
		}
	}
	for _, child := range r.Children {
		bump(child.ExitCode)
	}
	return code
}

type File struct {
	Path string
	Mode uint32
	Data []byte
}

type Plugin struct {
	Root    string
	Dialect Dialect
	Files   []File
}

func (p *Plugin) File(path string) (File, bool) {
	for _, file := range p.Files {
		if file.Path == path {
			return file, true
		}
	}
	return File{}, false
}
