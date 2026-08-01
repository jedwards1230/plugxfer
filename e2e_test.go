package plugxfer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess build in short mode")
	}
	bin := filepath.Join(t.TempDir(), "plugxfer")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/plugxfer")
	build.Env = append(os.Environ(), "GOCACHE="+filepath.Join(t.TempDir(), "go-cache"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	input, err := filepath.Abs(filepath.Join("testdata", "claude-basic"))
	if err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "output")
	command := exec.Command(bin, "convert", input, "--to", "codex", "--map", filepath.Join(input, "map.yaml"), "-o", outputDir)
	output, err := command.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("convert exit = %v, want 1\n%s", err, output)
	}
	if !strings.Contains(string(output), "# plugxfer conversion report") {
		t.Fatalf("stdout missing report:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "PLUGXFER-REPORT.md")); err != nil {
		t.Fatalf("output report: %v", err)
	}
	check := exec.Command(bin, "check", input, "--to=codex")
	checkOutput, err := check.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
		t.Fatalf("check exit = %v, want 2\n%s", err, checkOutput)
	}
}
