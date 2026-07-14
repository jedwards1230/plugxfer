package model

import "testing"

func TestReportExitCode(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		children []ChildReport
		want     int
	}{
		{"clean", nil, nil, 0},
		{"drop", []Finding{{Status: Dropped}}, nil, 1},
		{"map", []Finding{{Status: Dropped}, {Status: NeedsMap}}, nil, 2},
		{"child", nil, []ChildReport{{ExitCode: 2}}, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := Report{Findings: test.findings, Children: test.children}
			if got := r.ExitCode(false); got != test.want {
				t.Fatalf("ExitCode = %d, want %d", got, test.want)
			}
		})
	}
}

func TestReportExitCodeStrict(t *testing.T) {
	tests := []struct {
		name                  string
		findings              []Finding
		children              []ChildReport
		wantLoose, wantStrict int
	}{
		{"clean stays zero", nil, nil, 0, 0},
		{
			name:       "activation warning is advisory by default, fails under strict",
			findings:   []Finding{{Status: Note, Class: Activation, Severity: High}},
			wantLoose:  0,
			wantStrict: 1,
		},
		{
			name:       "lossy transform is advisory by default, fails under strict",
			findings:   []Finding{{Status: Transformed, Class: Loss}},
			wantLoose:  0,
			wantStrict: 1,
		},
		{
			name:       "drop is non-zero in both modes",
			findings:   []Finding{{Status: Dropped, Class: Loss}},
			wantLoose:  1,
			wantStrict: 1,
		},
		{
			name:       "needs-map outranks everything in both modes",
			findings:   []Finding{{Status: NeedsMap, Class: Loss}, {Status: Note, Class: Activation}},
			wantLoose:  2,
			wantStrict: 2,
		},
		{
			name:       "strict escalation propagates from children",
			children:   []ChildReport{{ExitCode: 1}},
			wantLoose:  1,
			wantStrict: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := Report{Findings: test.findings, Children: test.children}
			if got := r.ExitCode(false); got != test.wantLoose {
				t.Errorf("ExitCode(false) = %d, want %d", got, test.wantLoose)
			}
			if got := r.ExitCode(true); got != test.wantStrict {
				t.Errorf("ExitCode(true) = %d, want %d", got, test.wantStrict)
			}
		})
	}
}

func TestRulesVerifiedLine(t *testing.T) {
	same := RulesVerified{
		ClaudeCode: SpecPin{Version: "2.1.207", Date: "2026-07-13"},
		Codex:      SpecPin{Commit: "0877afbe8", Date: "2026-07-13"},
	}
	if got, want := same.Line(), "Claude Code 2.1.207, Codex @0877afbe8 (2026-07-13)"; got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
	diff := RulesVerified{
		ClaudeCode: SpecPin{Version: "2.1.207", Date: "2026-07-13"},
		Codex:      SpecPin{Commit: "abc1234", Date: "2026-07-10"},
	}
	if got, want := diff.Line(), "Claude Code 2.1.207 (2026-07-13), Codex @abc1234 (2026-07-10)"; got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
}

func TestDialectOther(t *testing.T) {
	if Claude.Other() != Codex || Codex.Other() != Claude || Dialect("other").Valid() {
		t.Fatal("unexpected dialect behavior")
	}
}
