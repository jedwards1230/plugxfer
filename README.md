# plugxfer

`plugxfer` is a deterministic, bidirectional converter for Claude Code and
OpenAI Codex plugins.

The two ecosystems share much of their packaging, but differ at the behavioral
edges: hook types and events, MCP environment interpolation, commands, agents,
bundled executables, and harness-specific metadata. `plugxfer` converts what it
can and emits a mandatory report for everything it copies, transforms, drops,
or cannot map without an explicit answer.

## Install

Until the first tagged release, build from source:

```bash
go install github.com/jedwards1230/plugxfer/cmd/plugxfer@main
```

Tagged releases publish static archives for Linux and macOS on amd64/arm64 and
Windows on amd64.

## Usage

```text
plugxfer check   <dir>
plugxfer convert <dir> -o <outdir>
  [--to claude|codex]
  [--map <file>]
  [--rules <dir>]
  [--strict]
  [--only <name>[,<name>...]]
```

`check` analyzes without writing. `convert` writes a new converted tree. The
target is auto-detected from `.claude-plugin/` or `.codex-plugin/`; use `--to`
when both markers exist or a plugin only contains shared components.

```bash
# Inspect a Claude plugin without changing it
plugxfer check ./my-plugin --to codex

# Convert it to a new directory
plugxfer convert ./my-plugin --to codex -o ./build/my-plugin-codex

# Convert selected local entries in a marketplace
plugxfer convert ./marketplace --to codex -o ./build/marketplace-codex \
  --only tools,review
```

### Exit codes

| Code | Meaning |
|---:|---|
| `0` | Clean conversion; no dropped or unresolved features |
| `1` | Conversion completed with explicitly reported losses |
| `2` | Conversion completed but requires answer-file mappings |
| `3` | Invalid input, unsafe path, parse failure, or other error |

Exit codes `1` and `2` still produce a converted tree. `3` does not: conversion
uses a sibling staging directory and only renames it into place after every
artifact has been written successfully.

## Artifacts

Every converted plugin contains:

- `PLUGXFER-REPORT.md`: component summary, file-and-line findings, applied
  fixes, unresolved mappings, and target-runtime notes.
- `plugxfer.map.yaml`: emitted only when a model or MCP value needs an explicit
  mapping. Reuse it with `--map` on the next run.
- the converted plugin tree, including project `.codex/agents/` files when
  converting Claude agents because Codex plugins cannot bundle agent roles.

Marketplace conversion writes one aggregate root report, one report under each
converted local plugin, and one shared map file. It preserves relative local
source layouts and emits the target registry. Claude-to-Codex conversion also
keeps the Claude registry because Codex reads it natively. Remote URL,
git-subdir, and npm entries are reported but never fetched. Marketplace mode
copies registry-declared local plugin trees only; unrelated files elsewhere in
a monorepo are deliberately excluded from the generated marketplace.

### Answer file

```yaml
version: 1
models:
  claude-to-codex:
    opus: gpt-5.4
  codex-to-claude:
    gpt-5.4: opus
replacements:
  ".mcp.json#mcpServers.example.env.URL": "https://example.com/mcp"
```

An existing `./plugxfer.map.yaml` is loaded automatically. `check` never writes
or updates it; unresolved stubs appear in the report. `convert` writes a merged
map into the output tree when unresolved stubs remain.

## Conversion behavior

- Skills remain `SKILL.md`; required frontmatter is normalized. Claude-only
  keys are reported, and invocation policy maps to `agents/openai.yaml`.
- Claude commands become provenance-marked Codex skills with a
  `## Command Template`. Exact-line `@file` imports are materialized. A reverse
  conversion recognizes provenance and restores the command form.
- Agents convert between Markdown frontmatter and project
  `.codex/agents/*.toml`. Effort and sandbox values use declarative maps;
  unresolvable model IDs use the answer file.
- Command hooks are filtered by supported event. Prompt/agent hooks and
  target-only events are dropped and reported; `async: true` is stripped so a
  Codex command hook is not silently skipped.
- MCP structures pass through, while exact environment references become
  `env_vars`, bearer headers become `bearer_token_env_var`, and composite
  interpolation requires an answer.
- Claude LSP and output-style components, Codex app declarations, manifest-only
  metadata, and `bin/` PATH semantics are handled according to the embedded
  rulebook and always appear in the report.
- Fence-aware Markdown detectors distinguish live-to-inert loss from
  inert-to-live activation. Literal Codex `!` shell and `@` import syntax is
  escaped when it would activate in a Claude command.

The editable rulebooks are in [`rules/`](rules). `--rules <dir>` requires
complete version-1 `features.yaml`, `detectors.yaml`, and `values.yaml` files.
Only the fixed, validated strategy vocabulary can be selected.

## Safety and limits

`plugxfer` treats plugin input as untrusted data:

- it never executes hooks, scripts, binaries, MCP commands, or Markdown shell
  syntax;
- it rejects symlinks and output path traversal;
- it refuses an output equal to or beneath the input and refuses to overwrite
  any existing output;
- it reads at most 10,000 files, 8 MiB per file, and 256 MiB total;
- it performs no network access and never fetches marketplace sources.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md). The test suite includes parser and
strategy units, golden trees for both directions, semantic round trips,
marketplace integration, atomic-write and path-safety checks, CLI subprocess
tests, and a pinned upstream reference-plugin workflow. The source-verified
format comparison and Claude specification snapshot live under
[`docs/research/`](docs/research/).
The complete test and workflow matrix is in [docs/TESTING.md](docs/TESTING.md).

## License

[MIT](LICENSE)
