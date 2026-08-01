# Claude Code ↔ Codex Plugin Ecosystems — Comparison & Converter Design

> **Date:** 2026-07-13 · **Authors:** multi-agent research (ClaudeRep / CodexRep / EcosystemScout / GeminiRep)
> **Scope:** spec-level comparison of the Claude Code plugin/marketplace format vs. OpenAI Codex
> plugin/marketplace format (+ Gemini CLI extensions, §6), source-of-truth tracking, and a design
> for a bidirectional converter (extensible to other harnesses).
> Claude Code **v2.1.207** (2026-07-11) · Codex plugins launched **2026-03-27** · Gemini CLI
> consumer-sunset **2026-06-18** (→ Antigravity CLI).

---

## 0. The headline: the ecosystem converged

In **H1 2026** the agentic-coding field collapsed onto **one plugin shape**. Claude Code pioneered
`plugin.json` bundling `{rules, commands, skills, subagents, hooks, MCP}` + a `marketplace.json`.
Then, independently:

- **Codex** (2026-03-27) shipped a plugin system that is a **deliberate near-1:1 mirror** of Claude
  Code's — same `plugin.json`, same `SKILL.md`, same hook event names, and explicit
  `CLAUDE_PLUGIN_ROOT`/`CLAUDE_PLUGIN_DATA` env aliases in its hook runtime.
- **Cursor** (2.5/2.6, Feb 2026) launched a Marketplace with the same 6-primitive bundle + `SKILL.md`.
- **Gemini CLI** converged on the same **primitives** (skills, hooks, subagents, MCP, commands,
  context) and adopted the **Agent Skills standard (`SKILL.md`) verbatim** — but kept its **own
  container format** (`gemini-extension.json`, TOML commands, `GEMINI.md`, its own hook event names).
  Convergence ≠ copy here. See §6.
- **opencode** and **Zed** adopted `SKILL.md` verbatim.

> **Nuance that matters for the converter:** only **Codex** copied Claude's *file format*. Gemini
> (and partially Cursor) copied the *primitive set* plus the SKILL.md standard. So Claude↔Codex is a
> file-copy job; Claude↔Gemini is a real (but enumerable) transform layer.

**Consequence:** Claude ↔ Codex is now a *mechanical* conversion (skills/MCP/plugin.json copy nearly
as-is), and a thin superset **intermediate representation (IR)** becomes worth building the moment you
add Cursor/Gemini as targets. **MCP is a free universal primitive** — a shared wire spec, so it is
*pass-through*, never translated.

---

## 1. Side-by-side spec comparison

### 1.1 Packaging manifest

| | **Claude Code** | **Codex CLI** ✅ SOURCE-VERIFIED (`core-plugins/src/manifest.rs`) |
|---|---|---|
| Manifest path | `.claude-plugin/plugin.json` | `.codex-plugin/plugin.json` — **and `.claude-plugin/plugin.json` read natively as fallback** (`plugin_namespace.rs`) |
| Shared fields | `name`, `version`, `description`, `author{name,email,url}`, `homepage`, `repository`, `license`, `keywords` | Only `name`, `version`, `description`, `keywords[]`. **`author`/`homepage`/`repository`/`license` NOT parsed** (marketplace entries flatten them as metadata; `author.name`→`interface.developerName`, `homepage`→`websiteUrl`) |
| Component pointers | `commands`, `agents`, `skills`, `hooks`, `mcpServers`, `lspServers`, `outputStyles` (relative `./` paths; supplement auto-discovery) | `skills` (str or []), `mcpServers` (path or inline), `hooks` (path/[]/inline), `apps` (path). Defaults: `skills/`, `hooks/hooks.json`, `.mcp.json`, `.app.json`. **No `agents` pointer — plugins cannot bundle agents.** Paths must be `./`-relative, no `..`; violations warn-and-drop |
| Presentation metadata | *(none in plugin.json; lives in marketplace.json)* | **`interface{}`** (all optional): `displayName`, `shortDescription`, `longDescription`, `developerName`, `category`, `capabilities[]`, `websiteUrl`, `privacyPolicyUrl`, `termsOfServiceUrl`, `defaultPrompt` (≤3 prompts, ≤128 chars), `brandColor`, `composerIcon`, `logo`, `logoDark`, `screenshots[]` |
| Config declaration | `userConfig{}` → exported as `CLAUDE_PLUGIN_OPTION_<KEY>`; `sensitive` flag; `dependencies`, `allowedSettings`/`disallowedSettings`, `channels` | Config lives in `config.toml` layer, not the manifest |
| Manifest optional? | Yes — auto-discovers components in default dirs if omitted | Yes — everything optional/defaulted; empty `name` falls back to dir basename; unknown keys silently ignored (no `deny_unknown_fields`) |

### 1.2 Component-type mapping

| Primitive | Claude Code | Codex CLI | Fidelity |
|---|---|---|---|
| **Skills** | `skills/<n>/SKILL.md` (YAML frontmatter, 3-level progressive disclosure, `allowed-tools`, `user-invocable`, etc.) | `skills/<n>/SKILL.md` — **identical**, incl. progressive disclosure; OpenAI's *recommended* reusable-instruction mechanism | **1:1 copy** |
| **MCP servers** | `.mcp.json` / inline `mcpServers` — stdio + `type:http` (+ deprecated `sse`); `${...}` env substitution | `.mcp.json` via pointer, or `[mcp_servers.<id>]` in config.toml — stdio + HTTP; `enabled_tools`, `auth`, `bearer_token_env_var`, `scopes` | **1:1** (universal primitive) |
| **Hooks** | `hooks/hooks.json` — 9 events, `type:command` **and** `type:prompt`; matcher regex; stdin JSON / stdout JSON contract; `${CLAUDE_PLUGIN_ROOT}` | `hooks/hooks.json` — event names `SessionStart, SubagentStart, PreToolUse, PermissionRequest, PostToolUse, PreCompact, PostCompact, UserPromptSubmit, SubagentStop, Stop`; `${PLUGIN_ROOT}` + **`CLAUDE_PLUGIN_ROOT` alias** | **~1:1 for `type:command`**; **LOSSY** below |
| **Subagents** | `agents/*.md` (markdown + YAML frontmatter: `name`, `description`, `model`, `tools`, `isolation`, `effort`, `color`…) | Custom agents `~/.codex/agents/*.toml` (`name`, `description`, `developer_instructions`, `model`, `model_reasoning_effort`, `sandbox_mode`, `mcp_servers`) | **Semi-lossy**: md-frontmatter → TOML; **bundling in a plugin unconfirmed** |
| **Slash commands** | `commands/*.md` (frontmatter `description`, `allowed-tools`, `model`, `argument-hint`; `$ARGUMENTS`) | Custom prompts `~/.codex/prompts/*.md` (`$1`–`$9`, `$ARGUMENTS`, `$FILE`) — **DEPRECATED**, and **not a bundled plugin component** | **LOSSY**: fold into skills (recommended) or emit deprecated loose prompts |
| **Project instructions** | `CLAUDE.md` (cascading) | `AGENTS.md` (cascading, 32 KiB cap, `AGENTS.override.md`) | **1:1 concept** (not a plugin component) |
| **LSP servers** | `.lsp.json` / `lspServers` (`command`, `args`, `extensionToLanguage`) | *(none)* | **DROP + warn** |
| **Output styles** | `output-styles/*.md` (`keep-coding-instructions`, `force-for-plugin`) | *(none)* | **DROP + warn** |
| **bin/ on PATH** | `bin/` added to PATH when plugin enabled (v2.1.91+); bare invocation | *(none)* — reference scripts by `${PLUGIN_ROOT}/…` path | **LOSSY / rewrite** |
| **Embedded app UI** | *(none)* | **`.app.json`** → Apps-SDK app (MCP server + iframe UI) | **New in Codex, no Claude source** |

### 1.3 Marketplace / registry

| | **Claude Code** | **Codex CLI** ✅ SOURCE-VERIFIED (`core-plugins/src/marketplace.rs`) |
|---|---|---|
| Registry file | `.claude-plugin/marketplace.json` | `.agents/plugins/marketplace.json` (+ `api_marketplace.json`), under `$HOME` or any registered root — **and `.claude-plugin/marketplace.json` read natively as fallback** |
| Per-plugin entry | `name`, `displayName`, `description`, `author`, `category`, `source{source:git-subdir\|url\|local, url, path, ref, sha}`, `version`, `keywords`, `tags`, `strict` | `name` (req), `source` (req), `policy{installation: NOT_AVAILABLE\|AVAILABLE\|INSTALLED_BY_DEFAULT, authentication: ON_INSTALL\|ON_USE, products}`, `category`, + **any extra fields flattened** as fallback plugin.json metadata |
| Source types | `local` path, `url`, `git-subdir` | bare path, `local`, `url`, `git-subdir` — **plus `npm {package, version?, registry?}`** |
| Install flow | `/plugin marketplace add <repo>` → `/plugin install <plugin>@<marketplace>` | same; git marketplaces staged to `$CODEX_HOME/.tmp/marketplaces/<name>`, plugins cached at `$CODEX_HOME/plugins/cache`, enable state at config `plugins.<name@marketplace>.enabled` |
| Source pinning | `sha` pins exact commit; `renames{}` for backward-compat | **`ref` and `sha` pinning confirmed in source**; `marketplace add --ref`, `--sparse` |
| Official directory | `anthropics/claude-plugins-official` (live) | **Self-serve publishing now LIVE** at platform.openai.com/plugins (was "coming soon" May 2026): draft → review → dev-controlled publish; requires verified identity, domain verification, tool-metadata hints (`readOnlyHint`/`destructiveHint`…), 5+3 test cases |

### 1.4 The hook lossy edge (converter-critical) ✅ SOURCE-VERIFIED (`hooks/src/`, `config/src/hook_config.rs`)

Codex hooks are the one component with real semantic loss:

- **Event sets differ.** Codex = exactly 10: `PreToolUse, PermissionRequest, PostToolUse,
  PreCompact, PostCompact, SessionStart, UserPromptSubmit, SubagentStart, SubagentStop, Stop`.
  Claude = 9. Delta: **Claude-only `SessionEnd`, `Notification`** (drop + report →Codex);
  **Codex-only `PermissionRequest`, `PostCompact`, `SubagentStart`** (drop + report →Claude).
  Codex **ignores matchers on `UserPromptSubmit` and `Stop`** (`events/common.rs:105-119`).
- **Only `type:"command"` executes.** `type:"prompt"`/`type:"agent"` are skipped **with a startup
  warning** (`discovery.rs:548-556` — "not supported yet"), not purely silently. Claude Code's
  *recommended* hook style is prompt-based → those won't run.
- **`async: true` hooks are skipped entirely** (`discovery.rs:474-480`) — not run synchronously.
  Converter must strip `async` (and warn) or the hook vanishes.
- **Trust gate (no Claude equivalent):** non-managed hooks only run once `enabled` +
  hash-trusted (`discovery.rs:527-540`). Converted hooks won't fire until the user trusts them —
  say so in the report.
- **`PreToolUse` can't intercept everything** — dispatch gated on `pre_tool_use_payload()`
  (function-payload tools only; `write_stdin` and code-mode `wait` explicitly opt out) → can't
  reliably block all actions.
- **Multiple matching hooks run concurrently** (`FuturesUnordered`, results re-sorted) — one
  can't pre-empt another. `A|B` matchers fire once per call, not per alias.
- **Stdout schema is Claude-compatible where it counts:** universal `{continue, stopReason,
  suppressOutput, systemMessage}`; PreToolUse adds top-level `decision: approve|block` + nested
  `hookSpecificOutput{permissionDecision: allow|deny|ask, permissionDecisionReason, updatedInput,
  additionalContext}` — both field families exist. Outputs are `deny_unknown_fields`; reserved
  PermissionRequest fields **fail closed**. Timeout is seconds (default 600, floor 1). Stdin adds
  Codex extensions `turn_id`, `agent_id`, `model`.
- **Upside:** plugin hooks get `PLUGIN_ROOT`/`PLUGIN_DATA` **and `CLAUDE_PLUGIN_ROOT`/
  `CLAUDE_PLUGIN_DATA`** (explicit OOTB-compat, `discovery.rs:228-236`) — as child env *and*
  `${KEY}` substitution in command text — so `${CLAUDE_PLUGIN_ROOT}/scripts/x.sh` keeps working
  unmodified.

> Also note Claude's v2.1.207 shell-injection hardening: emit exec-form `args` arrays or env vars,
> never shell-form `${user_config.*}` substitution.

### 1.5 Claude-side detailed schemas ✅ DOCS-VERIFIED (2026-07-13)

Full standalone reference: [`claude-code-plugin-spec-reference.md`](claude-code-plugin-spec-reference.md).
Converter-relevant detail extracted here; every field below is an IR slot and a potential loss line.

**plugin.json (complete field set).** Required: `name` (lowercase/hyphens/numbers, ≤64 chars;
`version` required only for marketplace). Optional: `description`, `author{name,email,url}`,
`homepage`, `repository`, `license`, `keywords[]`, component pointers (`commands[]`, `agents[]`,
`skills[]`, `hooks`, `mcpServers` (path or inline), `lspServers` (path or inline),
`outputStyles[]` — paths *supplement* auto-discovery, never replace it), `userConfig{}`,
`allowedSettings[]`/`disallowedSettings[]`, `dependencies[]` (plugin-level deps — must be
installed to load), `channels[]` ({name, description, mcpServer} — message-injection binding).
Manifest fully optional: name falls back to directory name.

**userConfig → env contract.** Each key becomes `CLAUDE_PLUGIN_OPTION_<KEY>` in all plugin
subprocesses. `sensitive: true` = env-only (never persisted to settings.json);
`sensitive: false` = persisted + env. v2.1.207: options no longer read from project
`.claude/settings.json`. No Codex plugin.json analog → converter maps to Codex config.toml
guidance or drop+report (verify pending).

**`${CLAUDE_PLUGIN_ROOT}` scope limitation (converter-critical):** substituted in JSON configs
(hooks/MCP/LSP) and executed scripts, but **NOT in markdown frontmatter or command/skill bodies**.
A detector must not "fix" occurrences in md bodies — they were already dead text on the Claude side.

**Command frontmatter (all optional):** `description` (~60 chars, autocomplete), `allowed-tools`
(CSV or YAML list; patterns: `Write`, `Write|Edit`, `Bash(git:*)`, `mcp__.*_delete.*`), `model`
(sonnet|opus|haiku|inherit), `argument-hint`, `disable-model-invocation`. Body: `$ARGUMENTS`,
`$1`–`$9`, `@file` refs, `` !`cmd` `` execution.

**Agent frontmatter (full set — each row needs a Codex-TOML mapping decision):**

| Field | Codex TOML target | Status |
|---|---|---|
| `name`, `description` | `name`, `description` | map |
| `model` | `model` | **value-map** (sonnet/opus/haiku → Codex model IDs) |
| `tools` / `disallowedTools` | (tool vocab differs) | partial map + report |
| `permissionMode` | `sandbox_mode` | **enum-map** (oracle: migrator has the table) |
| `effort` | `model_reasoning_effort` | enum-map |
| `mcpServers` | `mcp_servers` | map |
| `maxTurns`, `skills`, `hooks`, `memory`, `background`, `isolation`, `color` | *(none known)* | **drop + report** (verify pending) |

**Skill frontmatter:** open-standard core (`name`, `description`, `version`, `author`,
`license`) + CC extensions (`allowed-tools`, `user-invocable`, `disable-model-invocation`,
`argument-hint`, `model`, `context`, `agent`, `hooks`) — Codex honors only
name/description/metadata.short-description (§4.7); `disable-model-invocation` →
`openai.yaml policy.allow_implicit_invocation: false`; the rest are per-key loss lines.
3-level progressive disclosure is a *runtime* behavior on both sides — no conversion action.

**Hooks I/O contract (both types):** stdin JSON `{session_id, transcript_path, cwd,
permission_mode, hook_event_name, tool_name, tool_input, tool_result}`; stdout JSON
`{continue, suppressOutput, systemMessage, permissionDecision: allow|deny|ask,
permissionDecisionReason}`; exit 0 = ok, 2 = blocking, other = non-blocking. 9 events
(PreToolUse, PostToolUse, UserPromptSubmit, Stop, SubagentStop, SessionStart, SessionEnd,
PreCompact, Notification). Multiple matching hooks run in parallel; hooks load at session start
only. Matchers are case-sensitive regex (`*` wildcard, `mcp__<server>__<tool>` for MCP tools).
Codex stdout field names must be compared post-verification (suspected `decision` vs
`permissionDecision` rename).

**Marketplace source pinning:** `source: "./local"` | `{source: url, url, sha}` |
`{source: git-subdir, url, path, ref, sha}`; `strict: false` allows manifest-less skill bundles
(explicit `skills[]` paths); `renames{}` for name migrations. `sha` = exact-commit
reproducibility.

**bin/ on PATH (v2.1.91+):** whole `bin/` dir prepended to PATH while enabled — bare invocation
from any Bash context. LSP servers (`command`, `args`, `initializationOptions`,
`extensionToLanguage`, `env` v2.1.72+) and output styles (`keep-coding-instructions`,
`force-for-plugin` — the latter overrides the user's own style setting) complete the
Claude-only set.

### 1.6 Codex-side detailed schemas ✅ SOURCE-VERIFIED (openai/codex @ 0877afbe8, 2026-07-13)

**Custom agents ("agent roles", `core/src/config/agent_roles.rs`):** TOML files with `name`
(required standalone), `description` (**required non-blank**), `nickname_candidates[]`, and a
**flattened full ConfigToml** — any config key is legal per-role (`developer_instructions`
**required non-empty**, `model`, `model_reasoning_effort`, `sandbox_mode`, `mcp_servers`,
`approval_policy`, …). Load paths: `~/.codex/agents/**/*.toml` (user), **project
`.codex/agents/`**, `/etc/codex/agents` (system), + inline `[agents.<name>]` config tables;
layers merge per-role. **Never from plugin dirs** → converter emits agents to the *project*
`.codex/agents/` next to the converted plugin, with a report note.

**`.mcp.json` (`codex-mcp/src/plugin_config.rs`):** `{"mcpServers":{...}}`. Transports: `stdio
{command, args, env, env_vars, cwd}` and streamable-HTTP `{url, bearer_token_env_var,
http_headers, env_http_headers}`; `type` accepted leniently (`http|streamable_http|
streamable-http|stdio`; unknown → warn). Extra per-server fields: `enabled_tools`,
`disabled_tools`, `auth: oauth|chatgpt`, `scopes`, `oauth{client_id}`, `enabled`, `required`,
timeouts, `default_tools_approval_mode`. **NO `${VAR}` interpolation anywhere** — `env` is
literal; the pass-through mechanism is `env_vars: [NAME]` (forwards the host var). Converter rule:
Claude `"KEY": "${KEY}"` → `env_vars=[KEY]`; `Authorization: Bearer ${TOK}` header →
`bearer_token_env_var`; other `${...}` values → **unresolvable, warn** (the first-party migrator
aborts that server).

**Skills discovery outside plugins (`core-skills/src/loader.rs`):** project `.codex/skills/`,
user `$CODEX_HOME/skills/` (deprecated) + `~/.agents/skills/`, system `/etc/codex/skills` —
and the **repo `.agents/skills/` alias is honored** (probed from project root down to cwd).
Same cross-tool alias Gemini honors — `.agents/` is becoming the neutral namespace.

**`.app.json` (`connectors/src/plugin_config.rs`):** `{"apps": {"<display-name>": {"id":
"<connector-id>", "category"?}}}` — declares ChatGPT app connectors; can reroute the plugin's MCP
servers through the connector under ChatGPT auth. Do-not-emit for Claude→Codex; drop + report on
reverse.

**Value enums (converter map tables):**
- `model_reasoning_effort`: `none|minimal|low|medium|high|xhigh|max|ultra` (+ unknown passthrough).
  First-party migrator maps Claude `effort: max`→`xhigh`, passes `none..xhigh`, drops the rest.
- `sandbox_mode`: `read-only` (default) | `workspace-write` | `danger-full-access`. Migrator:
  `permissionMode: acceptEdits`→`workspace-write`, `readOnly`→`read-only`; **`default`/`plan`/
  `bypassPermissions`/`dontAsk` → dropped** (no sandbox_mode emitted).
- Bundled model slugs: `gpt-5.6-sol|terra|luna`, `gpt-5.5`, `gpt-5.4`, `gpt-5.4-mini`, `gpt-5.2`,
  `codex-auto-review` (free-form string validated against catalog) → Claude `model:
  sonnet|opus|haiku` has **no static mapping — this is the canonical value-map/`ask` case**.
- Plugin id: `<name>@<marketplace>`; hook timeout seconds (default 600).

**First-party migrator scope (the oracle's blind spots):** `external-agent-migration` handles
`.mcp.json` + `~/.claude.json` MCP, settings hooks, `agents/*.md`, `commands/*.md` — and
**nothing else** (no plugin.json/marketplace.json/output-styles/.lsp.json/bin//CLAUDE.md; it only
token-rewrites "CLAUDE.md"→"AGENTS.md", "Claude Code"→"Codex" in migrated bodies). Agent
frontmatter: consumes only `name`, `description`, `permissionMode`, `effort`; **silently drops
`model`, `tools`, `disallowedTools`, `skills`**. Hooks: only `type:command` with key set ⊆
{type, command, timeout(Sec), statusMessage, async}; drops `async:true`, `if`-groups, unknown
keys; rewrites `.claude/hooks/` paths → `.codex/hooks/` and copies scripts. Every one of these
drops is a place plugxfer does better (value-map model, translate tools, report everything).

---

## 2. Source of truth & change tracking

### 2.1 Claude Code
- **Docs (canonical):** `https://code.claude.com/docs/` — `plugins-reference`, `hooks`, `mcp`, `output-styles`. (`docs.claude.com/en/docs/claude-code/` redirects here.)
- **Source of record:** `github.com/anthropics/claude-code` — `CHANGELOG.md`, releases, `/plugins/` reference plugins (13+).
- **Skills standard:** `github.com/anthropics/skills` (AgentSkills.io / `SKILL.md`).
- **Official marketplace:** `github.com/anthropics/claude-plugins-official`.
- **Version now:** v2.1.207 (2026-07-11).
- **Unofficial JSON Schema:** `github.com/hesreallyhim/claude-code-json-schema`.
- **Tracking:** watch releases + CHANGELOG. The *format* isn't independently versioned — diff docs pages between releases.

### 2.2 Codex
- **Docs (canonical):** `https://learn.chatgpt.com/docs/` — `plugins`, `build-plugins`, `hooks`, `custom-prompts`, `agent-configuration/subagents`, `config-file/config-reference`. (`developers.openai.com/codex/*` **308-redirects here**.)
- **Source of record (authoritative for undocumented behavior):** `github.com/openai/codex` — CHANGELOG, releases, the plugin/hook parser source (e.g. plugin-marketplace CLI PR #21396).
- **Apps SDK (distinct surface):** `https://developers.openai.com/apps-sdk/`; examples `github.com/openai/openai-apps-sdk-examples`.
- **Launch facts:** plugins launched 2026-03-27; **self-serve directory publishing LIVE** as of
  2026-07 (platform.openai.com/plugins; see §1.3).
- **Community (cross-check only):** `codex-marketplace.com`, `github.com/hashgraph-online/awesome-codex-plugins`.
- **Tracking:** diff `learn.chatgpt.com/docs/{plugins,hooks,build-plugins}` + the codex repo's plugin/hook parser between releases. Format not independently versioned.

### 2.3 Shared
- **MCP spec:** `modelcontextprotocol.io` — the one primitive both track against as an external standard. Convert as pass-through.

### 2.4 Disambiguation — do NOT conflate
The "ChatGPT/Codex marketplace" is **several distinct surfaces**:
1. **Codex CLI plugins + marketplace** — CURRENT, the converter target.
2. **Codex IDE / cloud** — same host, shares `config.toml` + plugin/MCP config. Not separate.
3. **2023 ChatGPT Plugins** (`ai-plugin.json` + OpenAPI) — **DEAD** (shut down 2024-04-09).
4. **GPTs / Actions (GPT Store)** — legacy-but-live; OpenAPI-schema Actions. Low relevance.
5. **ChatGPT Apps / Apps SDK** — CURRENT consumer "app" story: MCP server + iframe UI; App Directory submissions open in 2026. A Codex plugin can *embed* one via `.app.json`.

---

## 3. Cross-harness capability matrix (for IR scope)

**F** full · **P** partial/semantic-mismatch · **—** none.

| Primitive | Claude Code | Codex | Cursor | Gemini CLI | opencode | Continue | Windsurf | Aider |
|---|---|---|---|---|---|---|---|---|
| Instructions/rules | F | F | F | F | F | F | F | F(file) |
| Slash-commands/prompts | F | F(deprecated) | P | F | F | F | F(workflows) | — |
| Subagents/modes | F | F | F | F | F | P | — | P |
| Hooks/events | F | F(cmd only) | F | F | F | — | — | — |
| MCP servers | F | F | F | F | F | F | F | P |
| Packaging manifest | F | F | F | F | P | F | — | — |
| Marketplace/registry | F | F | F | P | P | F | P | — |
| bin-scripts-on-PATH | F | P | P | — | P | — | — | — |
| Skills (SKILL.md) | F(origin) | F | F | F | F | — | P | — |

**Read:** CC / Codex / Cursor / Gemini share the convergent 6-primitive core → round-trippable.
Continue / Windsurf / Aider (+ Zed/Roo) are **one-way *export* targets** — emit rules + MCP (+ prompts), drop the rest.

---

## 4. Converter design

### 4.1 Recommendation
1. **Build Claude ↔ Codex pairwise first.** It's near-isomorphic and validates the model.
2. **Route through a thin IR** (normalized plugin model) even for the first pair, so Cursor/Gemini
   targets amortize later. IR node set = the convergent core.
3. **MCP = pass-through IR node** (zero translation).
4. **Emit a conversion report** every run: mapped-clean / transformed / dropped(lossy) per component.
5. **Down-map-only** adapters for Continue/Windsurf/Aider.

### 4.2 Intermediate representation (superset)
```
Plugin
├── meta        { name, version, description, author, homepage, repository, license, keywords }
├── instructions[]   # CLAUDE.md / AGENTS.md / GEMINI.md / .mdc  (content + activation hint)
├── skills[]         # SKILL.md (frontmatter + body + bundled files)   ← copy verbatim
├── commands[]       # slash commands (desc, args, allowed-tools, body)
├── agents[]         # subagents (name, desc, sysprompt, model, tools, effort, sandbox)
├── hooks[]          # {event, matcher, [{type, command|prompt, timeout}]}
├── mcpServers{}     # PASS-THROUGH (stdio/http)
├── binScripts[]     # PATH-injected executables (Claude-only source)
├── lspServers{}     # Claude-only
├── outputStyles[]   # Claude-only
├── app              # Codex-only (.app.json Apps-SDK ref)
├── interface{}      # Codex marketplace card metadata
└── marketplace{}    # registry entry (source, category, policy, pinning)
```

### 4.3 Transform rules — Claude → Codex
| Source | Action |
|---|---|
| `.claude-plugin/plugin.json` | → `.codex-plugin/plugin.json`; copy `name/version/description/keywords`; lift `author/homepage` into `interface{developerName, websiteUrl}` (not parsed as top-level by Codex); synthesize `interface{}` from name/description |
| `skills/**` | **copy verbatim**; per-key report for CC-only frontmatter (§4.7); `disable-model-invocation` → `agents/openai.yaml` sidecar |
| `.mcp.json` / `mcpServers` | copy structure; **rewrite `${VAR}`** (Codex has no interpolation): `"KEY":"${KEY}"`→`env_vars=[KEY]`, bearer headers→`bearer_token_env_var`, other `${...}`→warn |
| `hooks/hooks.json` `type:command` | copy; `${CLAUDE_PLUGIN_ROOT}` works via Codex alias; **strip `async:true` + warn** (Codex skips async hooks); drop `SessionEnd`/`Notification` events + report; note trust-gate in report |
| `hooks/hooks.json` `type:prompt`/`type:agent` | **DROP → report** (Codex warns + skips them) |
| `commands/*.md` | **convert → `skills/<cmd>/SKILL.md`** (recommended) or emit `~/.codex/prompts/*.md` (deprecated) → report |
| `agents/*.md` | reformat frontmatter → **project `.codex/agents/*.toml`** (`developer_instructions` = body; plugins **cannot** bundle agents — SOURCE-VERIFIED); enum-map `effort`→`model_reasoning_effort` (`max`→`xhigh`), `permissionMode`→`sandbox_mode` (`acceptEdits`→`workspace-write`, `readOnly`→`read-only`, else drop+report); `model` via value-map (`ask`) |
| `bin/**` | rewrite call-sites to `${PLUGIN_ROOT}/bin/…`; **warn** no PATH injection (SOURCE-VERIFIED absent) |
| `.lsp.json`, `output-styles/**` | **DROP → report** |
| `marketplace.json` | reshape to `.agents/plugins/marketplace.json` (`{name,source,policy,category}`) |
| `CLAUDE.md` | → `AGENTS.md` |

### 4.4 Transform rules — Codex → Claude (reverse)
| Source | Action |
|---|---|
| `.codex-plugin/plugin.json` | → `.claude-plugin/plugin.json`; drop `interface{}` (or lift into marketplace.json) |
| `skills/**`, `.mcp.json` | **copy verbatim** |
| `hooks/hooks.json` | copy (`${PLUGIN_ROOT}`→`${CLAUDE_PLUGIN_ROOT}`); Claude runs a superset, no loss |
| `~/.codex/agents/*.toml` | TOML → md-frontmatter (`developer_instructions` → body) |
| `.app.json` (Apps-SDK) | **DROP → report** (no Claude equivalent; the MCP server underneath *can* be surfaced as an `.mcp.json` entry) |
| `AGENTS.md` | → `CLAUDE.md` |

### 4.5 Lossy-edge checklist (surface in every report)
- Prompt/agent-type hooks (Claude→Codex) — **dropped** (Codex warns + skips).
- `async: true` hooks — **stripped + warned** (Codex skips them entirely).
- Hook events `SessionEnd`/`Notification` (→Codex) and `PermissionRequest`/`PostCompact`/
  `SubagentStart` (→Claude) — **dropped per direction**.
- `bin/` PATH injection — **rewritten to path refs** (confirmed no Codex equivalent).
- Slash commands — **folded into skills** (or deprecated prompts).
- Subagents — **cannot be plugin-bundled on Codex (confirmed)**; emitted to project
  `.codex/agents/`; `permissionMode: default|plan|bypassPermissions|dontAsk` and unmapped models
  have no target.
- MCP `${VAR}` refs — **rewritten** to `env_vars`/`bearer_token_env_var` or **warned**.
- LSP servers, output styles, `userConfig` — **Claude-only, dropped/reported**.
- `.app.json` embedded apps, `interface{}` — **Codex-only, dropped on reverse**.
- Converted hooks are **trust-gated** on Codex — user must approve before first run.

### 4.6 Formerly-open questions — ALL RESOLVED ✅ SOURCE-VERIFIED 2026-07-13
1. Can a Codex plugin bundle custom agents? **NO** — no `agents` pointer; agents load only from
   `~/.codex/agents/`, project `.codex/agents/`, `/etc/codex/agents`, or inline config tables.
   Converter emits to project `.codex/agents/`.
2. Any Codex `bin/`-on-PATH equivalent? **NO** — confirmed absent from all plugin/skill loaders.
3. Is `interface{}` mandatory? **NO** — every manifest field optional/defaulted; unknown keys
   silently ignored.
4. Self-serve Plugin Directory publishing? **OPEN/LIVE** — platform.openai.com/plugins
   (draft → review → publish; identity + domain verification, tool-annotation hints, 5+3 test
   cases).

### 4.6b Validation pass (2026-07-13) — reversals & confirmations
Source-level audit (openai/codex @ `0877afbe8`) over the original web-research claims.

**Reversals (first pass got these wrong):**
- "Same manifest field set" — **wrong**: Codex parses only `name/version/description/keywords`
  (+ pointers + `interface{}`); `author/homepage/repository/license` ignored at plugin.json level.
- "Codex `${...}` env substitution in MCP config" — **wrong**: no interpolation at all;
  `env_vars` passthrough is the mechanism.
- "Prompt/agent hooks silently skipped" — **softened**: skipped with a visible startup warning.
- "`async` parsed but not implemented" — **sharpened**: `async:true` hooks are *skipped
  entirely*, not run sync.
- Hook event lists asserted identical-ish — **wrong**: Claude-only `SessionEnd`/`Notification`;
  Codex-only `PermissionRequest`/`PostCompact`/`SubagentStart`; Codex ignores matchers on
  `UserPromptSubmit`/`Stop`.

**New facts the first pass missed:**
- **Codex natively reads `.claude-plugin/plugin.json` and `.claude-plugin/marketplace.json` as
  fallback paths** — a Claude plugin's manifest layer partially works in Codex *unconverted*;
  the converter's real job is the content (hooks types, commands, agents, bin/, MCP env).
- Hooks are **trust-gated** (hash + enable) before first run.
- Marketplace supports an **`npm` source type**; policy enums (`installation`/`authentication`).
- Codex honors the **`.agents/skills/` repo alias** (same neutral namespace Gemini honors).
- Complete value-enum tables for `model_reasoning_effort`, `sandbox_mode`, model slugs (§1.6).

### 4.7 Content-level syntax deltas (verified against `openai/codex` source, 2026-07-13)

Beyond file-tree mapping, markdown *bodies* carry syntax that is live in one harness and inert in
the other → two warning classes: **loss** (live→inert, Claude→Codex) and **activation**
(inert→live, Codex→Claude — incl. the security-grade case of literal `` !`cmd` `` text becoming
real shell execution in a Claude command).

- **Codex has NO `@`-imports anywhere** (AGENTS.md = pure concatenation, `agents_md.rs`; skills
  never preprocessed, `render.rs`; prompts = `$`-substitution only). Feature request #17401 cites
  Claude parity. `@path` passes through as dead text.
- **Codex has NO inline shell execution** anywhere; skills have *zero* placeholder syntax (bundled
  `scripts/` only run if the model chooses to).
- **Codex skill loader honors only** `name`, `description`, `metadata.short-description`
  (`loader.rs`, no `deny_unknown_fields`). `allowed-tools`, `context: fork`, `agent`, `model`,
  `user-invocable`, `disable-model-invocation`, `hooks` → **silently ignored**. Codex's analogs
  live in a per-skill **`agents/openai.yaml` sidecar** (`interface`, `dependencies.tools`,
  `policy.allow_implicit_invocation` ← maps from `disable-model-invocation`). Frontmatter is
  **mandatory** in Codex (missing = hard load error) — converter must always emit it.
- **Codex-only syntax to scan on the reverse trip:** `$UPPERCASE` named placeholders + `$$` escape
  (deprecated prompts), `agents/openai.yaml`, `AGENTS.override.md` / fallback-filename layering.
- **`{{…}}` handlebars** — OpenAI's migrator flags it unsupported; add to the scanner.
- **Conformance oracle:** `codex-rs/external-agent-migration/` is OpenAI's own **first-party
  Claude→Codex migrator** — commands → skills (`## Command Template` embedding), agents → TOML
  (`effort`→`model_reasoning_effort`, `permissionMode`→`sandbox_mode`), hooks with path rewriting.
  It **rejects/skips** any command containing `$ARGUMENTS`, `$<digit>`, `{{}}`, `` !`cmd` ``, or
  `@file` — exactly the gap our converter fills by degrading gracefully (materialize imports,
  escape activations, warn on dynamics) instead of skipping. Also answers §4.6 Q1–Q2 in spirit:
  mirror its mappings, beat its drop behavior.

### 4.8 Implementation notes
- Parsing surface: JSON (manifests/hooks/mcp), YAML frontmatter (skills/commands/agents), TOML (Codex agents/config). A single tool must handle all three.
- Suggested shape: a CLI `plugxfer <from> <to> --in <dir> --out <dir> [--report]` over the IR, with per-harness reader/writer adapters. Go (single static binary, easy homelab distribution) or TS (npm, sits next to plugin-dev tooling) both fit — Go recommended for portability.
- Golden-file tests: round-trip Claude→Codex→Claude on the 13 official reference plugins; assert the IR is stable and the report lists exactly the known lossy edges.

---

## 6. Gemini CLI — the third harness (GeminiRep, verified 2026-07-13)

### 6.1 Strategic status — read this first
**Gemini CLI was sunset for consumers on 2026-06-18** (announced at I/O 2026-05-19). Free/Pro/Ultra
users were directed to **Antigravity CLI** (`agy`, closed-source), which keeps Agent Skills, Hooks,
Subagents, and Extensions ("plugins") and migrates via `agy plugin import gemini`. Gemini CLI lives
on **only for Gemini Code Assist Standard/Enterprise** licensees. Converter stance: treat the
`gemini-extension.json` spec as **frozen** (enterprise + OSS installed base), and track Antigravity
plugins as a fourth, under-documented dialect — its internals (`plugin.json` marker,
`mcp_config.json` under `~/.gemini/antigravity-cli/plugins/`) are third-party-reported only,
**unconfirmed**. Subagent compatibility already broke in the Gemini→Antigravity transition.

### 6.2 Format verdict: primitives yes, format no
- **SKILL.md — TRUE 1:1.** `skills/<name>/SKILL.md` per the Agent Skills open standard (agentskills.io),
  landed PR #16045 (~v0.25–0.26). Four discovery tiers; honors a cross-tool **`.agents/skills/`
  alias** directory (deliberate interop). Only `name`/`description` frontmatter documented — Claude
  extras (`allowed-tools`, `context: fork`, `agent`) presumed ignored → lossy note.
- **`hooks/hooks.json` filename — shared;** everything else is Google's own shape:
  - Manifest: **`gemini-extension.json`** at repo root — `name`, `version`, `mcpServers`,
    `contextFileName`, `excludeTools` (command-granular), `settings[]` ({name, envVar, sensitive} —
    extensions do **not** inherit shell env), `themes`, `plan`, `migratedTo`.
  - Commands: **TOML** (`commands/git/commit.toml` → `/git:commit`); `prompt` + `description` only;
    `{{args}}`, `!{...}` shell, `@{...}` file injection. No allowed-tools, no per-command model.
  - Subagents: `agents/*.md` — md+frontmatter, structurally close to Claude's, plus Gemini-only
    `kind: local|remote` (A2A), inline `mcpServers`, `temperature`, `max_turns`, `timeout_mins`.
  - Hooks: **command-type only** (no prompt hooks); own event names — `BeforeTool`/`AfterTool`
    (≈PreToolUse/PostToolUse), `BeforeAgent`/`AfterAgent`, `SessionStart/End`, `PreCompress`
    (≈PreCompact), **plus `BeforeModel`/`AfterModel`/`BeforeToolSelection`** (LLM request/response
    rewriting — no Claude equivalent). Exit-code contract mirrors Claude's (0=parse stdout,
    2=block).
  - MCP: declared **in the manifest** `mcpServers` block (or settings.json / agent frontmatter) —
    **no `.mcp.json` support**; env must route through `settings[]`→`envVar`.
  - Variables: `${extensionPath}` (≈`CLAUDE_PLUGIN_ROOT`), `${workspacePath}` — **config-file string
    substitution, not env export** → converter rewrites strings.
  - Registry: **no marketplace.json at all** — the gallery (geminicli.com/extensions) is a daily
    **GitHub topic crawl** (`gemini-cli-extension` + root manifest). No curation/signing.
    Install: `gemini extensions install <github-url>` with `--ref`; platform release archives
    (`{platform}.{arch}.{name}.tar.gz`).

### 6.3 Adapter rules (IR ↔ Gemini)
| IR node | Gemini form | Claude→Gemini | Gemini→Claude |
|---|---|---|---|
| meta | `gemini-extension.json` | transform (reshape; lowercase-dash name) | transform (drop `settings[]`/`excludeTools`/`themes` → report) |
| instructions | `GEMINI.md` / `contextFileName` | ~1:1 rename | ~1:1 |
| skills | `skills/<n>/SKILL.md` | **1:1**; Claude-extra frontmatter flagged ignored | **1:1** clean |
| commands | `commands/**.toml` | transform MD→TOML; `$ARGUMENTS`→`{{args}}`, frontmatter-bash→`!{...}`, `@`→`@{...}`; lossy: allowed-tools/model | transform TOML→MD, low loss |
| agents | `agents/*.md` | transform (tools vocab, model names) | transform; lossy: `kind:remote`, inline mcpServers, temperature/max_turns |
| hooks | `hooks/hooks.json` + settings | transform (event + stdout-field renames, e.g. `permissionDecision`→`decision`); **drop prompt hooks** | transform; **drop BeforeModel/AfterModel/BeforeToolSelection** |
| mcpServers | manifest `mcpServers` | transform (inline; env→`settings[]` — **semantic**, extensions don't inherit env) | transform (extract to `.mcp.json`) |
| binScripts | none | drop→`${extensionPath}` path refs + warn | n/a |
| marketplace | GitHub topic crawl | drop/synthesize (per-repo topic + release naming) | synthesize marketplace.json from crawl metadata |

**IR addition:** a capability-flags layer — prompt hooks (Claude-only), model-wrapping hooks
(Gemini-only), remote/A2A agents (Gemini-only), per-command tool restriction (Claude/Codex-only),
extension env isolation (Gemini-only).

### 6.4 Source of truth
`github.com/google-gemini/gemini-cli` (`docs/` canonical; rendered at geminicli.com/docs) ·
skills repo `google-gemini/gemini-skills` · Agent Skills standard `agentskills.io` ·
Antigravity: antigravity.google/docs (JS-rendered, poorly fetchable) + transition post on
developers.googleblog.com. Post-sunset, expect the OSS spec to be static and Antigravity to drift.

---

## 7. Bottom line
The market did the hard convergence work in H1 2026 — but at **two different depths**:
- **Codex copied Claude's file format** → Claude ↔ Codex is mechanical (skills, MCP, `plugin.json`
  copy nearly as-is; real work = prompt-hook drop, agents md↔TOML, commands→skills, Claude-only
  extras).
- **Gemini copied the primitive set + the SKILL.md standard, not the container** → Claude ↔ Gemini
  is a genuine adapter (manifest reshape, MD↔TOML commands, hook event/schema renames, MCP inlined
  into the manifest with env rerouted through `settings[]`) — fully enumerable, but not a copy job.
  And Gemini CLI itself is sunset for consumers (2026-06-18) in favor of Antigravity, so the Gemini
  adapter targets a frozen spec.

Build through the thin IR with a capability-flags layer; skills + MCP are the universal bridges;
**the conversion report is the product** — every drop (prompt hooks, model-wrapping hooks, A2A
agents, bin/ PATH) must be surfaced, never silent. Continue/Windsurf/Aider remain export-only.

And the validation pass's headline: **Codex already reads `.claude-plugin/` manifests natively.**
The manifest layer is not the problem the converter solves — the *content* layer is (hook types
and events, commands, non-bundleable agents, MCP env interpolation, bin/, and the loss/activation
syntax scan). That's precisely the layer OpenAI's own migrator handles by *skipping*; plugxfer's
job is to handle it by *degrading gracefully and reporting*.
