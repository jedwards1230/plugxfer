# plugxfer — PRD

> **Status:** implemented v1 · 2026-07-13 · owner: justin
> **Research base:** [`docs/research/claude-vs-codex-plugin-conversion.md`](research/claude-vs-codex-plugin-conversion.md)
> (source-verified against `openai/codex @ 0877afbe8` and Claude Code v2.1.207 docs) +
> [`claude-code-plugin-spec-reference.md`](research/claude-code-plugin-spec-reference.md).

## 1. Problem

Claude Code and Codex CLI plugins are near-isomorphic (Codex deliberately mirrored the format,
and even reads `.claude-plugin/` manifests as fallbacks) — but the *content* layer diverges in
ways that fail silently: prompt-type hooks that never run, `@`-imports that become dead text one
way and **live imports** the other, `` !`cmd` `` text that becomes real shell execution, MCP
`${VAR}` refs Codex never interpolates, agents that can't be plugin-bundled. OpenAI's own
first-party migrator handles these edges by **skipping components wholesale**. There is no tool
that converts bidirectionally, degrades gracefully, and *tells you exactly what changed*.

## 2. Goals / non-goals

**Goals**
1. Bidirectional directory-in → directory-out conversion: Claude Code plugin ↔ Codex plugin.
2. **Declarative, file-based rules**: a feature list + compatibility list + reconciliation
   strategy per feature, editable without touching code.
3. **A mandatory conversion report** — every component: `copied | transformed | dropped(why) |
   needs-map`, with file:line for content findings. Silence is a bug.
4. Content-level syntax scanning with two warning classes: **loss** (live→inert) and
   **activation** (inert→live; security-severity for `` !`cmd` ``).
5. Deterministic + CI-friendly: unresolvable values go to a reusable **answers file**
   (`plugxfer.map.yaml`), never an interactive prompt (v1).

**Non-goals (v1)**
- Gemini/Antigravity, Cursor, or any third harness (rulebook keys are `claude`/`codex` only, but
  nothing in the design precludes adding a dialect later).
- Behavioral emulation (e.g. simulating prompt hooks on Codex via wrapper commands).
- Marketplace *publishing* (directory submission flows); we only emit registry files.
- Watching upstream specs for drift (manual rulebook updates; see §9).

## 3. Users

- **Primary:** me — converting `plugins/*` (homelab-k8s, homelab-ops, homelab-tools, …) and the
  public `claude-plugins` repo for Codex users, and importing interesting Codex plugins back.
- **Secondary:** plugin authors in either ecosystem (public repo, single static binary).

## 4. CLI surface (dead simple)

```
plugxfer check   <dir>                      # detect direction, print report, write nothing
plugxfer convert <dir> -o <outdir>          # convert; report → stdout + <outdir>/PLUGXFER-REPORT.md
  [--to claude|codex]                       # override auto-detection
  [--map <file>]                            # answers file (default: ./plugxfer.map.yaml if present)
  [--rules <dir>]                           # override embedded rulebook
  [--strict]                                # exit non-zero if any drop, needs-map, or compat loss remains
```

Direction auto-detect: `.claude-plugin/` vs `.codex-plugin/` (both present → require `--to`;
neither → probe default component dirs). Exit codes: `0` clean, `1` converted-with-losses
(report lists them), `2` needs-map entries unresolved, `3` error. By default only dropped
components (1) and unresolved mappings (2) affect the code; `--strict` additionally escalates
the otherwise-advisory loss/activation warnings (which stay exit-0 by default) so CI can gate
on a perfectly clean conversion.

**Marketplace mode (first-class).** If the input dir's manifest is a *marketplace*
(`.claude-plugin/marketplace.json` or `.agents/plugins/marketplace.json`), plugxfer fans out
over every plugin entry and converts the registry itself:

- **Local-path sources** (`./plugins/<name>`, `{source: local}`) → resolved inside the input
  tree and converted in place in the output tree, same relative layout.
- **Remote sources** (`url` / `git-subdir` / `npm`) → **listed as skipped** in the report with
  the resolved coordinates (v1 does not fetch; `--fetch` is a stretch flag).
- Emits a converted registry: →Codex writes `.agents/plugins/marketplace.json` (entry schema
  reshaped: `policy` defaults, extra metadata flattened) *and* keeps `.claude-plugin/
  marketplace.json` (Codex reads it natively — emitting both is free compatibility); →Claude
  synthesizes `.claude-plugin/marketplace.json` with `source`/`ref`/`sha` preserved.
- **One aggregate report** (`PLUGXFER-REPORT.md` at output root: per-plugin summary table +
  links to per-plugin reports) and **one shared map file** — `ask`-class stubs from all plugins
  merge into a single `plugxfer.map.yaml`, so `model: sonnet` is answered once for the fleet.
- Exit code = worst across plugins. `check` mode works identically (fan-out, aggregate, no
  writes). Per-plugin selection: `--only <name>[,<name>…]`.

## 5. Architecture — three declarative axes + fixed strategies

```
plugxfer/
├── rules/
│   ├── features.yaml    # axis 1: component mapping (file-tree level)
│   ├── detectors.yaml   # axis 2: content syntax (loss/activation scanning)
│   └── values.yaml      # axis 3: enum/value maps (effort, permissionMode, events, models)
├── internal/
│   ├── model/           # thin IR: dialect, normalized files, findings, reports
│   ├── reader/          # bounded, symlink-free dialect readers
│   ├── engine/          # fixed transform strategies selected by the rulebook
│   ├── marketplace/     # registry parsing, source resolution, schema reshape
│   ├── app/             # plugin/marketplace orchestration + atomic writes
│   ├── answers/         # reusable map loading, merging, and stubs
│   └── report/          # deterministic Markdown rendering
└── testdata/            # golden round-trip fixtures
```

- Rulebooks embedded via `go:embed`; `--rules` overrides at runtime.
- Rulebook **selects** strategies; strategies are the fixed small set in code (~9). New spec
  feature ⇒ YAML edit; genuinely new transform semantics ⇒ new strategy (rare by design).
- **MCP is pass-through structurally** but gets the one semantic strategy: `mcp-env-rewrite`
  (`"KEY":"${KEY}"` → `env_vars=[KEY]`; `Authorization: Bearer ${T}` → `bearer_token_env_var`;
  other `${...}` → needs-map/warn) — Codex has zero interpolation (source-verified).

## 6. Feature rules (v1 rulebook content — from verified research)

| Feature | →Codex | →Claude |
|---|---|---|
| plugin.json meta | reshape; `author/homepage` → `interface{developerName,websiteUrl}` | reshape; `interface{}` → marketplace.json entry or drop |
| skills/** | copy; per-key report for CC-only frontmatter; `disable-model-invocation` → `openai.yaml policy` | copy (always emit frontmatter — Codex requires it, Claude tolerates) |
| .mcp.json | copy + `mcp-env-rewrite` | copy + reverse rewrite (`env_vars` → `${VAR}` refs) |
| hooks type:command | copy; strip `async` (warn); event-map via values.yaml; drop `SessionEnd`/`Notification`; trust-gate note | copy; drop `PermissionRequest`/`PostCompact`/`SubagentStart`; field-map stdout schema |
| hooks type:prompt/agent | drop-warn | copy |
| commands/*.md | fold-into-skill (`## Command Template` embedding, oracle-compatible naming) | copy |
| agents | md-toml-agent → **project `.codex/agents/`** (not bundleable — verified); enum-maps below | toml→md; unmappable keys dropped+reported |
| bin/ | rewrite call-sites `${PLUGIN_ROOT}/bin/…`; warn no PATH | n/a |
| lsp / output-styles / userConfig | drop-warn | n/a |
| .app.json / interface{} | n/a | drop-warn (surface underlying MCP if any) |
| marketplace.json | reshape (note: Codex reads `.claude-plugin/marketplace.json` natively — emit both) | reshape; synthesize `sha`/`ref` pins if present |

**Detectors (axis 2):** `@`-import (→Codex: loss, fix=materialize; →Claude: activation,
fix=escape), `` !`cmd` `` (loss / **activation-HIGH**), `$UPPERCASE` + `$$` (Codex prompts →
Claude: arrive literal), `{{…}}` (both: flagged unsupported), `${CLAUDE_PLUGIN_ROOT}` in md
bodies (dead on both — do not "fix"). Fence-aware scanning; uncertain hits reported as
`possible`, never silently ignored or auto-fixed.

**Value maps (axis 3):** `effort→model_reasoning_effort` (`max→xhigh`, passthrough
`none..xhigh`); `permissionMode→sandbox_mode` (`acceptEdits→workspace-write`,
`readOnly→read-only`, else drop); hook event names per direction; **models = `ask` class** (no
static map; goes to plugxfer.map.yaml).

## 7. The report + answers file

- `PLUGXFER-REPORT.md`: a header (source/target/mode + which upstream CLI specs the rules were
  verified against, from the rulebook's optional `verified:` block), a summary table (per
  component: status), then detail sections — drops with reasons, detector findings with file:line
  + class (loss/activation/possible) + applied fix, needs-map stubs, and environment notes
  (trust-gate on Codex, agents emitted outside plugin). In marketplace `check` mode (which writes
  nothing) the per-plugin findings are embedded in the aggregate report instead of linked files.
- `plugxfer.map.yaml`: written/merged when `ask`-class values are unmapped; re-run picks it up.
  Reusable across plugins; committable.

## 8. Milestones

- **M1 — `check`:** readers + rulebook + detectors + report. No writes. Validates the whole
  model against real plugins (`plugins/*` here + `anthropics/claude-code` reference plugins).
- **M2 — `convert` Claude→Codex + marketplace fan-out:** writers + strategies + map file +
  marketplace mode. Acceptance: `plugxfer convert` on the home-orchestration root converts the
  whole `jedwards1230-home-orchestration` marketplace in one run; converted `homelab-tools`
  loads in Codex; aggregate report lists exactly the known losses (prompt hooks, bin/,
  userConfig).
- **M3 — `convert` Codex→Claude + round-trip goldens:** Claude→Codex→Claude on the 13 official
  reference plugins; IR stable; activation warnings fire on seeded fixtures.
- **Stretch:** interactive `--ask` mode (fills the same map file), `plugxfer diff` (re-convert +
  compare after upstream spec changes), additional dialects.

## 9. Risks & mitigations

- **Spec drift** (neither format versioned): rulebook is data → cheap updates; `plugxfer diff`
  (stretch) makes drift visible; pin research doc as the audit trail.
- **Codex fallback-reading `.claude-plugin/`** could tempt "no conversion needed" — the report's
  job is to show what *silently doesn't work* in that mode; `check` on an unconverted plugin is
  the killer demo.
- **Detector false positives** (emails, decorators): fence-aware lexing + `possible` class keeps
  them non-blocking.
- **Oracle divergence:** where OpenAI's migrator and our output could both be "valid", follow the
  migrator's conventions (skill naming, `## Command Template`) for ecosystem compatibility.

## 10. Repo & implementation

- New repo `jedwards1230/plugxfer` (public), Go, single static binary, cobra-free stdlib `flag`
  preferred (two subcommands don't need a framework). Add to `scripts/repos.conf`.
- Parsing: JSON + YAML frontmatter + TOML (BurntSushi/toml, goccy/go-yaml or gopkg.in/yaml.v3).
- Work in `repos/plugxfer/worktrees/<branch>/` per worktree convention; conventional commits;
  standard repo-standards baseline on creation.
