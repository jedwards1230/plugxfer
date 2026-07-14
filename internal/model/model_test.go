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

func TestDialectOther(t *testing.T) {
	if Claude.Other() != Codex || Codex.Other() != Claude || Dialect("other").Valid() {
		t.Fatal("unexpected dialect behavior")
	}
}
