package cli

import (
	"fmt"
	"io"
)

const usage = `plugxfer converts plugins between Claude Code and Codex.

Usage:
  plugxfer check <dir> [--to claude|codex] [--map <file>] [--rules <dir>] [--strict]
  plugxfer convert <dir> -o <outdir> [--to claude|codex] [--map <file>] [--rules <dir>] [--strict]

Commands:
  check    Analyze a plugin and print a conversion report without writing files
  convert  Convert a plugin and write a mandatory conversion report
`

// Run executes the command and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "check", "convert":
		fmt.Fprintf(stderr, "plugxfer %s is not implemented yet\n", args[0])
		return 3
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		fmt.Fprint(stderr, usage)
		return 3
	}
}
