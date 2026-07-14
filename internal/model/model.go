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
	Findings    []Finding
	Children    []ChildReport
}

type ChildReport struct {
	Name       string
	Source     string
	ReportPath string
	ExitCode   int
	Counts     map[Status]int
}

func (r *Report) Add(f Finding) { r.Findings = append(r.Findings, f) }

func (r Report) Counts() map[Status]int {
	counts := map[Status]int{}
	for _, finding := range r.Findings {
		counts[finding.Status]++
	}
	return counts
}

func (r Report) ExitCode(strict bool) int {
	code := 0
	for _, f := range r.Findings {
		switch f.Status {
		case NeedsMap:
			if code < 2 {
				code = 2
			}
		case Dropped:
			if code < 1 {
				code = 1
			}
		}
	}
	for _, child := range r.Children {
		if child.ExitCode > code {
			code = child.ExitCode
		}
	}
	if strict && code == 1 {
		return 1
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
