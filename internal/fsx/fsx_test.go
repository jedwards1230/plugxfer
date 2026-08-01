package fsx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func TestValidateOutput(t *testing.T) {
	input := t.TempDir()
	if _, err := ValidateOutput(input, filepath.Join(input, "child")); err == nil {
		t.Fatal("expected descendant rejection")
	}
	existing := t.TempDir()
	if _, err := ValidateOutput(input, existing); err == nil {
		t.Fatal("expected existing-output rejection")
	}
	output := filepath.Join(filepath.Dir(input), "new-output")
	if got, err := ValidateOutput(input, output); err != nil || got != output {
		t.Fatalf("ValidateOutput = %q, %v", got, err)
	}
}

func TestStageWriteAndCommit(t *testing.T) {
	output := filepath.Join(t.TempDir(), "output")
	stage, finish, err := Stage(output)
	if err != nil {
		t.Fatal(err)
	}
	files := []model.File{{Path: "nested/file.txt", Mode: 0o600, Data: []byte("data")}}
	if err := Write(stage, files); err != nil {
		t.Fatal(err)
	}
	if err := finish(true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(output, "nested", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "data" {
		t.Fatalf("data = %q", data)
	}
	if info, err := os.Stat(filepath.Join(output, "nested", "file.txt")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, err=%v", info.Mode(), err)
	}
}

func TestWriteRejectsTraversal(t *testing.T) {
	if err := Write(t.TempDir(), []model.File{{Path: "../escape", Data: []byte("x")}}); err == nil {
		t.Fatal("expected unsafe-path error")
	}
}
