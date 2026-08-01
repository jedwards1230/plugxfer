# Testing plugxfer

The test strategy is layered so parser correctness, output stability, process
behavior, and real ecosystem compatibility fail independently.

## Local suite

```bash
make check
```

This runs module normalization, formatting checks, `go vet`, the race-enabled
suite, and a production build. The suite contains:

| Layer | Coverage |
|---|---|
| Unit | answers, frontmatter, detector fences, rules, reports, source resolution, exit codes, and filesystem safety |
| Strategy | manifests, skills/commands, agents, hooks, MCP, custom paths, inline components, and rule overrides |
| Golden | exact Claude-to-Codex and Codex-to-Claude output trees, including modes |
| Round trip | generated command provenance, imports, model/effort maps, bin calls, and activation safety |
| Marketplace | local fan-out, remote skips, aggregate reports, shared maps, and `--only` preservation |
| CLI e2e | builds and executes the real binary, validates stdout reports, artifacts, and exit codes |

Tests use only local fixtures and never execute plugin content.

## Reference plugins

Build the binary, then point the integration script at any directory containing
Claude plugins:

```bash
go build -o plugxfer ./cmd/plugxfer
scripts/test-reference-plugins.sh /path/to/claude-code/plugins
```

For every detected plugin the script runs `check`, Claude-to-Codex conversion,
and Codex-to-Claude conversion. Exit codes `0`, `1`, and `2` are valid analyzed
outcomes; `3` fails the integration. Every conversion must write its mandatory
report.

The `Reference plugins` workflow checks out `anthropics/claude-code` at an
immutable commit and runs this script over all 13 bundled plugins. Updating the
pin is an intentional compatibility change: run the script locally against the
new commit and review changed reports before updating the workflow SHA.

## GitHub workflows

| Workflow | Triggers | Purpose |
|---|---|---|
| `CI` | pull request, `main`, manual | module integrity, format, vet, race tests, coverage summary, build/smoke, govulncheck |
| `CodeQL` | pull request, `main`, weekly, manual | Go security analysis |
| `Reference plugins` | pull request, weekly, manual | pinned upstream check, conversion, and round trip |
| `Release` | signed `vX.Y.Z` tag | retest, five static archives, SHA-256 checksums, GitHub release |

All Actions are pinned to full commit SHAs. Dependabot groups Go module and
GitHub Actions updates into weekly pull requests.
