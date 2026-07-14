package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jedwards1230/plugxfer/internal/app"
	"github.com/jedwards1230/plugxfer/internal/model"
)

const usage = `plugxfer converts plugins between Claude Code and Codex.

Usage:
  plugxfer check <dir> [--to claude|codex] [--map <file>] [--rules <dir>] [--strict] [--only <names>]
  plugxfer convert <dir> -o <outdir> [--to claude|codex] [--map <file>] [--rules <dir>] [--strict] [--only <names>]

Commands:
  check    Analyze a plugin or marketplace and print a report without writing files
  convert  Convert a plugin or marketplace and write mandatory reports
`

var version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Fprintf(stdout, "plugxfer %s\n", version)
		return 0
	}
	mode := app.Mode(args[0])
	if mode != app.Check && mode != app.Convert {
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 3
	}
	options, help, err := parse(mode, args[1:], stderr)
	if help {
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "plugxfer: %v\n", err)
		return 3
	}
	result, err := app.Run(options)
	if err != nil {
		fmt.Fprintf(stderr, "plugxfer: %v\n", err)
		return 3
	}
	if _, err := stdout.Write(result.Markdown); err != nil {
		fmt.Fprintf(stderr, "plugxfer: write report: %v\n", err)
		return 3
	}
	return result.ExitCode
}

func parse(mode app.Mode, args []string, stderr io.Writer) (app.Options, bool, error) {
	fs := flag.NewFlagSet(string(mode), flag.ContinueOnError)
	fs.SetOutput(stderr)
	var to, output, mapPath, rulesPath, only string
	var strict bool
	fs.StringVar(&to, "to", "", "target dialect: claude or codex")
	fs.StringVar(&output, "o", "", "output directory")
	fs.StringVar(&mapPath, "map", "", "answers file")
	fs.StringVar(&rulesPath, "rules", "", "rulebook override directory")
	fs.StringVar(&only, "only", "", "comma-separated marketplace plugin names")
	fs.BoolVar(&strict, "strict", false, "return non-zero when drops or unresolved mappings remain")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: plugxfer %s <dir>%s [options]\n", mode, outputUsage(mode))
		fs.PrintDefaults()
	}
	flagArgs, positionals, err := splitArgs(args)
	if err != nil {
		return app.Options{}, false, err
	}
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return app.Options{}, true, nil
		}
		return app.Options{}, false, err
	}
	if len(positionals) != 1 {
		return app.Options{}, false, fmt.Errorf("%s requires exactly one input directory", mode)
	}
	if mode == app.Convert && output == "" {
		return app.Options{}, false, errors.New("convert requires -o <outdir>")
	}
	if mode == app.Check && output != "" {
		return app.Options{}, false, errors.New("check does not accept -o")
	}
	target := model.Dialect(to)
	if to != "" && !target.Valid() {
		return app.Options{}, false, fmt.Errorf("invalid --to value %q", to)
	}
	if mapPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return app.Options{}, false, err
		}
		mapPath = app.DefaultMapPath(cwd)
	}
	selected := map[string]bool{}
	for _, name := range strings.Split(only, ",") {
		if name = strings.TrimSpace(name); name != "" {
			selected[name] = true
		}
	}
	return app.Options{Mode: mode, Input: positionals[0], Output: output, Target: target, MapPath: mapPath, RulesPath: rulesPath, Strict: strict, Only: selected}, false, nil
}

func splitArgs(args []string) ([]string, []string, error) {
	valueFlags := map[string]bool{"-o": true, "--to": true, "--map": true, "--rules": true, "--only": true}
	var flags, positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		flags = append(flags, arg)
		name := arg
		if index := strings.IndexByte(arg, '='); index >= 0 {
			name = arg[:index]
		}
		if valueFlags[name] && !strings.Contains(arg, "=") {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("flag %s requires a value", arg)
			}
			i++
			flags = append(flags, args[i])
		}
	}
	return flags, positionals, nil
}

func outputUsage(mode app.Mode) string {
	if mode == app.Convert {
		return " -o <outdir>"
	}
	return ""
}
