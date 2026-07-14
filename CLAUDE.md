# plugxfer

@CONTRIBUTING.md

Bidirectional Claude Code and Codex plugin conversion with mandatory,
line-addressable compatibility reporting.

`docs/PRD.md` owns product scope, `docs/TESTING.md` owns the verification and
workflow matrix, and `docs/research/` preserves the format audit trail.

## Architecture

The implementation follows the product requirements in `docs/PRD.md`:

- `rules/` contains embedded declarative feature, detector, and value maps.
- `internal/model/` owns normalized files, dialects, findings, and reports.
- `internal/reader/` detects dialects and reads bounded, symlink-free trees.
- `internal/engine/` applies manifest, skill/command, agent, hook, MCP, content,
  and rule-selected transformations.
- `internal/app/` orchestrates plugin/marketplace runs and atomic output.
- `internal/marketplace/` parses, resolves, and reshapes registry entries.
- `internal/report/` renders mandatory Markdown reports.
- `internal/answers/` loads and merges deterministic value maps.
- `internal/fsx/` enforces output safety and staging.
- `testdata/` contains golden conversion and round-trip fixtures.

## Invariants

- Silence is a bug: every discovered component and compatibility decision must
  appear in the conversion report.
- `check` never writes files.
- Conversion is deterministic and non-interactive by default.
- Unknown or unmapped values are preserved in `plugxfer.map.yaml` rather than
  guessed.
- Content detectors are fence-aware and distinguish loss from activation.
- Parsing uses structured JSON, YAML, and TOML libraries rather than string
  replacement.
- MCP is structurally pass-through; only environment semantics are rewritten.
- Keep the CLI on the standard library `flag` package unless its surface grows
  enough to justify a framework.
- Never add network fetching to a conversion path; remote marketplace entries
  stay report-only unless a separately reviewed future flag explicitly changes
  the v1 contract.

## Package conventions

Keep `cmd/plugxfer/main.go` limited to process wiring. Business behavior belongs
under `internal/`, with table-driven tests alongside each package. Rulebook
changes require fixture coverage that proves both the output and report entry.
