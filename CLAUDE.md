# plugxfer

@CONTRIBUTING.md

Bidirectional Claude Code and Codex plugin conversion with mandatory,
line-addressable compatibility reporting.

## Architecture

The implementation follows the product requirements in `docs/PRD.md`:

- `rules/` contains embedded declarative feature, detector, and value maps.
- `internal/ir/` owns the normalized plugin representation.
- `internal/reader/` parses each source dialect into the IR.
- `internal/strategy/` contains the fixed transformation strategy set.
- `internal/writer/` emits each target dialect.
- `internal/report/` records every copied, transformed, dropped, and needs-map item.
- `testdata/` contains golden conversion and round-trip fixtures.

The current repository is an initialization scaffold; add these packages only
as their milestone requires them.

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

## Package conventions

Keep `cmd/plugxfer/main.go` limited to process wiring. Business behavior belongs
under `internal/`, with table-driven tests alongside each package. Rulebook
changes require fixture coverage that proves both the output and report entry.
