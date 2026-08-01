package reader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jedwards1230/plugxfer/internal/model"
)

func TestDetect(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "claude-basic")
	dialect, market, err := Detect(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if dialect != model.Claude || market {
		t.Fatalf("got %s market=%v", dialect, market)
	}
}

func TestReadRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("target", filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Read(root, model.Claude); err == nil {
		t.Fatal("expected symlink error")
	}
}
