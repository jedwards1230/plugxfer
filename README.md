# plugxfer

`plugxfer` is a deterministic, bidirectional converter for Claude Code and
OpenAI Codex plugins.

The two plugin ecosystems share much of their file format, but differ at the
behavioral edges: hook types and events, MCP environment interpolation,
commands, agents, bundled executables, and harness-specific metadata.
`plugxfer` converts what it can and emits a mandatory report for everything it
copies, transforms, drops, or cannot map without an explicit answer.

> [!NOTE]
> The project is in initial development. The command surface is scaffolded,
> but conversion is not implemented yet.

## Planned CLI

```text
plugxfer check   <dir>
plugxfer convert <dir> -o <outdir>
  [--to claude|codex]
  [--map <file>]
  [--rules <dir>]
  [--strict]
```

`check` analyzes without writing. `convert` writes the converted tree plus
`PLUGXFER-REPORT.md`. Unresolvable value mappings are recorded in a reusable
`plugxfer.map.yaml` file so CI runs remain deterministic and non-interactive.

## Design

The converter is organized around three declarative rule axes:

- component mappings and reconciliation strategies;
- content detectors for live-to-inert loss and inert-to-live activation;
- enum and model value mappings.

A thin intermediate representation separates Claude and Codex readers from
writers. Transformation code is limited to a small fixed strategy set selected
by embedded YAML rulebooks.

See [the product requirements](docs/PRD.md) for the complete scope and
milestones.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
