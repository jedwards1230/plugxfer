# Claude Code Plugin Ecosystem - Comprehensive Spec Reference

**Date**: July 13, 2026
**Claude Code Version**: v2.1.207 (current as of July 11, 2026)
**Source Authority**: Official Anthropic documentation + anthropics/claude-code repository

---

## 1. Plugin Structure & Manifest

### Directory Layout

All Claude Code plugins follow this standard directory structure:

```
plugin-name/
├── .claude-plugin/
│   ├── plugin.json                # Plugin metadata (optional)
│   └── marketplace.json           # Marketplace registry (for distrib.)
├── commands/                      # Slash commands (optional)
│   └── example.md
├── agents/                        # Subagent definitions (optional)
│   └── example.md
├── skills/                        # Agent Skills (optional)
│   ├── skill-a/
│   │   ├── SKILL.md
│   │   ├── reference.md (optional)
│   │   └── scripts/ (optional)
│   └── skill-b/
│       └── SKILL.md
├── hooks/                         # Event handlers (optional)
│   └── hooks.json
├── output-styles/                 # Custom output styles (optional)
│   └── style-name.md
├── .mcp.json                      # MCP server config (optional)
├── .lsp.json                      # LSP server config (optional, alt: inline in plugin.json)
└── README.md
```

**Key Principle**: All component directories (commands/, agents/, skills/, output-styles/, hooks/) must be at plugin root, NOT inside .claude-plugin/.

### plugin.json Schema

Location: `.claude-plugin/plugin.json` (optional; if omitted, Claude Code auto-discovers components in default locations)

**Complete Schema**:

```json
{
  "name": "string (required)",
  "version": "string (required for marketplace)",
  "description": "string",
  "author": {
    "name": "string",
    "email": "string",
    "url": "string"
  },
  "homepage": "string",
  "repository": "string",
  "license": "string",
  "keywords": ["string"],
  "commands": ["./path/to/commands/"],
  "agents": ["./path/to/agents/"],
  "skills": ["./path/to/skills/"],
  "hooks": "./path/to/hooks.json",
  "mcpServers": "./.mcp.json" | {...inline MCP config...},
  "lspServers": "./.lsp.json" | {...inline LSP config...},
  "outputStyles": ["./output-styles/"],
  "userConfig": {
    "option_key": {
      "type": "string|number|boolean",
      "description": "string",
      "sensitive": false,
      "default": "value"
    }
  },
  "allowedSettings": ["string"],
  "disallowedSettings": ["string"],
  "dependencies": ["plugin-name"],
  "channels": [
    {
      "name": "channel-name",
      "description": "string",
      "mcpServer": "mcp-server-key"
    }
  ]
}
```

**Required Fields**:
- `name`: Unique identifier (lowercase, hyphens/numbers only, max 64 chars)
- `version`: Only required for marketplace installations

**Optional Fields**: All others are optional. Custom paths supplement (not replace) defaults—components in both default directories and custom paths load.

**Special Behavior**:
- If `plugin.json` is omitted, plugin name derives from directory name
- If manifest provided, only affects metadata and component paths; auto-discovery still happens in defaults
- `dependencies`: Plugins listed here must be installed for this plugin to load
- `userConfig`: Declares plugin options; users prompted on plugin enable; values exported as `CLAUDE_PLUGIN_OPTION_<KEY>` environment variables

---

## 2. Component Types - File Formats & Schemas

### 2.1 Commands (Slash Commands)

**Location**: `commands/` directory (all `.md` files load automatically)

**File Format**: Markdown with optional YAML frontmatter

**Schema**:

```markdown
---
description: Brief description (~60 chars for autocomplete display)
allowed-tools: Read, Write, Bash(git:*)
model: sonnet | opus | haiku | inherit
argument-hint: "[arg1] [arg2]"
disable-model-invocation: false
---

Command prompt content here. Can reference $ARGUMENTS placeholder.
```

**Frontmatter Fields**:

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `description` | string | First line of prompt | Shown in autocomplete (~60 chars recommended) |
| `allowed-tools` | string/array | Inherit from conversation | Restrict tool access (CSV or YAML list) |
| `model` | string | Inherit from conversation | Override model (sonnet, opus, haiku) |
| `argument-hint` | string | None | Document expected arguments for users |
| `disable-model-invocation` | boolean | false | Prevent programmatic invocation via SlashCommand tool |

**Key Notes**:
- All frontmatter is optional—commands work without any
- `allowed-tools` can be CSV (older) or YAML list (modern spec-compliant)
- Tool patterns: exact (`Write`), multiple (`Write|Edit|Read`), regexes (`mcp__.*_delete.*`), wildcards

**Sources**:
- [Command development reference](https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/skills/command-development/references/frontmatter-reference.md)

### 2.2 Agents (Subagents)

**Location**: `agents/` directory (`.md` files)

**File Format**: Markdown with YAML frontmatter + system prompt body

**Schema**:

```markdown
---
name: agent-name
description: "Invocation trigger - when to use this agent"
model: sonnet | opus | haiku | inherit | claude-opus-4-6
tools: Tool1, Tool2, Tool3
disallowedTools: DisabledTool
permissionMode: ask | allow | deny
maxTurns: number
skills: skill-name1, skill-name2
mcpServers: mcp-server1, mcp-server2
hooks: ./hooks.json
memory: {"key": "value"}
effort: low | medium | high
background: true | false
isolation: worktree | remote | none
color: blue | green | red | ...
---

Agent system prompt / instructions here.

## When to use

Example context that triggers this agent...
```

**Frontmatter Fields**:

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `name` | string | Required | Unique identifier (lowercase, hyphens/numbers) |
| `description` | string | Required | Trigger/context for when agent activates; use "PROACTIVELY" for auto-invocation |
| `model` | string | Inherit | Specific model override |
| `tools` | string (CSV) | All tools | Allowlist of accessible tools |
| `disallowedTools` | string (CSV) | None | Deny specific tools (applied before tools allowlist) |
| `permissionMode` | string | Inherit | ask, allow, deny—overrides conversation setting |
| `maxTurns` | number | Unlimited | Max conversation turns before agent stops |
| `skills` | string (CSV) | None | Skills to load for this agent |
| `mcpServers` | string (CSV) | None | MCP servers accessible to agent |
| `hooks` | string | None | Path to hooks.json for agent-specific hooks |
| `memory` | object | None | Persistent key-value state across invocations |
| `effort` | string | None | low, medium, high—reasoning effort |
| `background` | boolean | false | true = run without user intervention |
| `isolation` | string | None | worktree, remote, none—execution context |
| `color` | string | None | UI display color |

**Key Notes**:
- Both `tools` and `disallowedTools` can be set; `disallowedTools` applies first
- Description field is critical—determines whether/when agent invokes
- Agents inherit conversation's permission mode, model, and tools unless overridden
- File name becomes agent ID in plugin namespace (e.g., `my-agent.md` → plugin:my-agent)

**Sources**:
- [Subagents documentation](https://docs.claude.com/en/docs/claude-code/sub-agents)

### 2.3 Skills

**Location**: `skills/` directory as subdirectories, each with `SKILL.md`

**File Format**: Markdown with YAML frontmatter + skill instructions

**Schema**:

```markdown
---
name: skill-name
description: "What this skill does and when to use it"
version: "1.0.0"
author: "Author Name"
license: MIT
allowed-tools: Read, Bash(git:*)
user-invocable: true
disable-model-invocation: false
argument-hint: "[arg]"
model: sonnet | opus | haiku
context: user-context | session-context | inherit
agent: agent-name (Claude Code only)
hooks: ./hooks.json
---

Skill instructions and context...

## Examples

Example usage patterns...
```

**Frontmatter Fields (Core)**:

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `name` | string | Directory name | Unique identifier |
| `description` | string | Required | Determines if/when skill activates (most important field) |
| `allowed-tools` | string/array | None | Tool allowlist (CSV or YAML list) |
| `version` | string | None | Version string |
| `author` | string | None | Author information |
| `license` | string | None | License type (e.g., MIT) |

**Frontmatter Fields (Claude Code Extensions)**:

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `user-invocable` | boolean | true | Can user invoke directly (vs. auto-only) |
| `disable-model-invocation` | boolean | false | Prevent auto-invocation |
| `argument-hint` | string | None | Document expected arguments |
| `model` | string | Inherit | Model override |
| `context` | string | Inherit | Context scope (user-context, session-context) |
| `agent` | string | None | Bind skill to specific agent |
| `hooks` | string | None | Skill-specific hooks |

**Key Notes**:
- Skills load in 3 levels: Level 1 (name + description only, ~100 tokens, at startup for discovery), Level 2 (full content on demand)
- AgentSkills.io spec (open standard) requires only `name` and `description`
- Claude Code extends with tool control, model selection, invocation modes
- Skills can include supporting scripts in `scripts/` subdirectory
- Registered as `<plugin-name>:<skill-name>` in Claude Code

**Sources**:
- [SKILL.md specification](https://github.com/anthropics/skills)
- [Skill development reference](https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/skills/skill-development/SKILL.md)

### 2.4 Hooks

**Location**: `hooks/hooks.json` (auto-loaded by convention, do NOT add "hooks" field to plugin.json)

**File Format**: JSON with event-type keys mapping to hook handler arrays

**Complete Schema**:

```json
{
  "description": "Optional explanation of hooks",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "prompt",
            "prompt": "Natural language prompt for LLM to evaluate...",
            "timeout": 30
          },
          {
            "type": "command",
            "command": "bash ${CLAUDE_PLUGIN_ROOT}/scripts/validate.sh",
            "timeout": 60
          }
        ]
      }
    ],
    "PostToolUse": [...],
    "UserPromptSubmit": [...],
    "Stop": [...],
    "SubagentStop": [...],
    "SessionStart": [...],
    "SessionEnd": [...],
    "PreCompact": [...],
    "Notification": [...]
  }
}
```

**Event Types**:

| Event | When | Input Fields | Use For |
|-------|------|-------------|---------|
| `PreToolUse` | Before tool execution | `tool_name`, `tool_input` | Validation, modification, approval/denial |
| `PostToolUse` | After tool completion | `tool_name`, `tool_input`, `tool_result` | Feedback, logging, validation |
| `UserPromptSubmit` | User submits prompt | `user_prompt` | Context addition, validation |
| `Stop` | Main agent stopping | `reason` | Verify task completeness |
| `SubagentStop` | Subagent stopping | `reason` | Ensure subagent completion |
| `SessionStart` | Session begins | None | Load context, environment setup |
| `SessionEnd` | Session ends | None | Cleanup, logging, state preservation |
| `PreCompact` | Before context compaction | None | Preserve critical info |
| `Notification` | User notifications sent | `notification` | React to notifications |

**Matcher Patterns**:

- Exact: `"matcher": "Write"`
- Multiple: `"matcher": "Write|Edit|Read"`
- Wildcard: `"matcher": "*"`
- Regex: `"matcher": "mcp__.*_delete.*"`
- MCP tools: `mcp__<server>__<tool>` (e.g., `mcp__filesystem__read_file`)
- Case-sensitive

**Hook Types**:

**Prompt-Based Hooks** (Recommended):
```json
{
  "type": "prompt",
  "prompt": "Evaluate if this action is safe: $TOOL_INPUT",
  "timeout": 30
}
```
- Sends prompt to Claude for single-turn evaluation
- Returns JSON with yes/no decision
- Timeout in seconds

**Command Hooks**:
```json
{
  "type": "command",
  "command": "bash ${CLAUDE_PLUGIN_ROOT}/scripts/validate.sh",
  "timeout": 60
}
```
- Runs shell command with event JSON on stdin
- Returns JSON via stdout
- Exit codes: 0 (success), 2 (blocking error), other (non-blocking)

**Hook Input (stdin JSON)**:
```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript.txt",
  "cwd": "/working/dir",
  "permission_mode": "ask|allow|deny",
  "hook_event_name": "PreToolUse",
  "tool_name": "Write",
  "tool_input": {...},
  "tool_result": "..."
}
```

**Hook Output (stdout JSON)**:
```json
{
  "continue": true,
  "suppressOutput": false,
  "systemMessage": "Optional message for Claude",
  "permissionDecision": "allow|deny|ask",
  "permissionDecisionReason": "Why deny/ask"
}
```

**Key Notes**:
- Hooks load at session start only (changes require restart)
- Multiple hooks per event run in parallel
- Always use `${CLAUDE_PLUGIN_ROOT}` for portability
- v2.1.207+: Shell injection prevention—use exec form (`args` array) or env vars, not shell-form `${user_config.*}` substitution
- Prompt-based recommended for most cases; command hooks for deterministic checks

**Sources**:
- [Hooks reference](https://docs.claude.com/en/docs/claude-code/hooks)
- [Hook development skill](https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/skills/hook-development/SKILL.md)

### 2.5 MCP Servers

**Location**: `.mcp.json` OR inline in `plugin.json` under `"mcpServers"`

**File Format**: JSON configuration mapping server names to connection details

**Schema (stdio - local process)**:

```json
{
  "mcpServers": {
    "my-server": {
      "command": "node",
      "args": ["/path/to/server.js"],
      "env": {
        "DEBUG": "${DEBUG:-false}",
        "API_KEY": "${CLAUDE_PLUGIN_OPTION_API_KEY}"
      }
    }
  }
}
```

**Schema (HTTP - remote)**:

```json
{
  "mcpServers": {
    "http-service": {
      "type": "http",
      "url": "https://api.example.com/mcp",
      "headers": {
        "X-API-Key": "${CLAUDE_PLUGIN_OPTION_API_KEY}",
        "Authorization": "Bearer ${CLAUDE_PLUGIN_OPTION_TOKEN}"
      }
    }
  }
}
```

**Schema (SSE - deprecated)**:

```json
{
  "mcpServers": {
    "legacy-service": {
      "type": "sse",
      "url": "https://api.example.com/sse",
      "headers": {
        "Authorization": "Bearer ${CLAUDE_PLUGIN_OPTION_TOKEN}"
      }
    }
  }
}
```

**Transport Types**:

| Type | Use | Scope |
|------|-----|-------|
| **stdio** (default) | Local process, direct system access | Project or user |
| **http** | Remote servers, cloud services (recommended for remote) | Project or user |
| **sse** | Server-sent events (deprecated, use HTTP) | Project or user |

**Configuration Scopes**:
- User (global): `~/.claude.json` — available in all projects
- Project: `.mcp.json` — project-specific

**Environment Variable Substitution**:
- `${CLAUDE_PLUGIN_OPTION_<KEY>}`: Plugin option values
- `${user_config.<key>}`: Non-sensitive plugin options (deprecated syntax)
- `${ENV_VAR:-default}`: Shell-style default values
- `${CLAUDE_SESSION_ID}`: Current session identifier

**Key Notes**:
- Stdio servers run as local processes; ideal for custom scripts
- HTTP is recommended for remote/cloud services
- SSE is deprecated; migrate to HTTP
- Environment variables with CLAUDE_PLUGIN_OPTION_* prefix (sensitive options)
- CLI: `claude mcp add <server-name> [--transport http|sse] <url>`

**Sources**:
- [MCP reference](https://code.claude.com/docs/en/mcp)
- [MCP quickstart](https://code.claude.com/docs/en/mcp-quickstart)

### 2.6 LSP Servers

**Location**: `.lsp.json` OR inline in `plugin.json` under `"lspServers"`

**File Format**: JSON configuration mapping language names to LSP server details

**Schema**:

```json
{
  "lspServers": {
    "go": {
      "command": "gopls",
      "args": ["serve"],
      "initializationOptions": {},
      "extensionToLanguage": {
        ".go": "go"
      }
    },
    "python": {
      "command": "python",
      "args": ["-m", "pylance"],
      "extensionToLanguage": {
        ".py": "python"
      }
    }
  }
}
```

**Fields**:

| Field | Type | Purpose |
|-------|------|---------|
| `command` | string | Executable name (must be on PATH) |
| `args` | array | Arguments to pass to command |
| `initializationOptions` | object | LSP initialization options (optional) |
| `extensionToLanguage` | object | Maps file extensions to LSP language IDs |
| `env` | object | Environment variables (v2.1.72+) |

**Key Notes**:
- LSP servers provide code intelligence (diagnostics, go-to-definition, hover info)
- Claude Code exposes 9 LSP operations via built-in LSP tool
- Real-time diagnostics similar to VS Code integration
- `extensionToLanguage` critical for matching files to language servers
- Official support added v2.1.74; diagnostics available via `textDocument/diagnostic`

**Sources**:
- [Language servers and LSP integration](https://github.com/boostvolt/claude-code-lsps)

### 2.7 Output Styles

**Location**: `output-styles/` directory OR `~/.claude/output-styles` (user) OR `.claude/output-styles` (project)

**File Format**: Markdown with YAML frontmatter + system prompt instructions

**Schema**:

```markdown
---
name: Custom Style Name
description: How this style modifies Claude's behavior
keep-coding-instructions: true
force-for-plugin: false
---

Instructions to add to Claude's system prompt...

When explaining code, always start with a diagram.

## Diagram conventions

Use flowchart TD for control flow, sequenceDiagram for request paths.
```

**Frontmatter Fields**:

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `name` | string | Filename | Display name in `/config` picker |
| `description` | string | None | Description shown in style picker |
| `keep-coding-instructions` | boolean | false | Keep Claude's software engineering instructions |
| `force-for-plugin` | boolean | false | Auto-apply when plugin is enabled (plugin styles only) |

**Key Notes**:
- Modifies system prompt directly (affects every response)
- Leave `keep-coding-instructions` unset/false if Claude won't be coding
- Set to true if changing tone/format but keeping coding behavior
- Custom instructions added to end of system prompt
- Plugin can force apply via `force-for-plugin: true` (overrides user `outputStyle` setting)
- Changes take effect after `/clear` or new session
- Contributes to prompt caching cost (cached after first request in session)

**Sources**:
- [Output styles documentation](https://code.claude.com/docs/en/output-styles)

---

## 3. Marketplace Configuration

### marketplace.json Schema

Location: `.claude-plugin/marketplace.json` (optional; used for plugin distribution)

**Complete Schema**:

```json
{
  "$schema": "https://anthropic.com/claude-code/marketplace.schema.json",
  "name": "my-marketplace",
  "description": "Curated plugins for my team",
  "owner": {
    "name": "Owner Name",
    "email": "owner@example.com"
  },
  "version": "1.0.0",
  "renames": {
    "old-plugin-name": "new-plugin-name"
  },
  "plugins": [
    {
      "name": "plugin-name",
      "displayName": "Display Name",
      "description": "What this plugin does",
      "author": {
        "name": "Author",
        "email": "author@example.com",
        "url": "https://github.com/author"
      },
      "category": "development|database|security|productivity|other",
      "version": "1.0.0",
      "homepage": "https://github.com/org/repo",
      "license": "MIT",
      "keywords": ["keyword1", "keyword2"],
      "tags": ["community-managed", "official"],
      "source": {
        "source": "git-subdir|url|local",
        "url": "https://github.com/org/repo.git",
        "path": "plugins/my-plugin",
        "ref": "main",
        "sha": "abc123def456"
      },
      "skills": ["./skill-a", "./skill-b"],
      "lspServers": {...},
      "mcpServers": {...},
      "strict": false,
      "commands": [...],
      "agents": [...]
    }
  ]
}
```

**Plugin Entry Fields**:

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `name` | string | ✓ | Unique identifier (kebab-case) |
| `displayName` | string | | User-friendly display name |
| `description` | string | ✓ | What plugin does |
| `author` | object | | Author info (name, email, url) |
| `category` | string | ✓ | development, database, security, productivity, other |
| `source` | object/string | ✓ | Plugin source location |
| `version` | string | | Plugin version |
| `homepage` | string | | Project/docs URL |
| `license` | string | | License type (e.g., MIT) |
| `keywords` | array | | Search keywords |
| `tags` | array | | Metadata tags (e.g., community-managed, official) |
| `skills` | array | | Paths to skills (relative to source.path) |
| `lspServers` | object | | LSP server configurations |
| `mcpServers` | object | | MCP server configurations |
| `strict` | boolean | | If false, allows skills without plugin.json manifest |
| `commands` | array | | Paths to commands |
| `agents` | array | | Paths to agents |

**Source Field Types**:

**URL (single repo)**:
```json
"source": {
  "source": "url",
  "url": "https://github.com/org/repo.git",
  "sha": "abc123def456"
}
```

**Git subdirectory (monorepo)**:
```json
"source": {
  "source": "git-subdir",
  "url": "https://github.com/org/monorepo.git",
  "path": "packages/plugin",
  "ref": "main",
  "sha": "abc123def456"
}
```

**Local path**:
```json
"source": "./plugins/my-plugin"
```

**Strict Mode**:
- `strict: true` (default): Requires plugin.json manifest; auto-discovers components in default directories
- `strict: false`: Allows plugins without manifest; requires explicit `skills` array pointing to SKILL.md locations; paths relative to `source.path`

**Example with strict: false**:
```json
{
  "name": "example-bundle",
  "source": {
    "source": "git-subdir",
    "url": "https://github.com/example-org/sdk.git",
    "path": "packages/agent-skills"
  },
  "strict": false,
  "skills": ["./skill-a", "./skill-b", "./skill-c"]
}
```

**Key Notes**:
- Marketplace registry enables plugin discovery and installation
- Each plugin maps to source location (URL, git-subdir, or local)
- `renames` field allows backward-compatible plugin name changes
- `sha` field pins exact commit for reproducibility
- Version field optional but recommended for tracking

**Sources**:
- [Create and distribute plugin marketplaces](https://docs.claude.com/en/docs/claude-code/plugin-marketplaces)
- [Official marketplace repository](https://github.com/anthropics/claude-plugins-official)

---

## 4. Environment Variables & Configuration

### CLAUDE_PLUGIN_ROOT

**What it is**: Absolute path to plugin's installation directory

**Usage**: Reference scripts, binaries, config files bundled with plugin

**Scope**: Works in JSON configs (hooks, MCP, LSP) and command-form scripts

**Example**:
```json
{
  "command": "bash ${CLAUDE_PLUGIN_ROOT}/scripts/validate.sh",
  "mcpServers": {
    "custom": {
      "command": "${CLAUDE_PLUGIN_ROOT}/bin/server.sh"
    }
  }
}
```

**Limitations**: Does NOT work in Markdown frontmatter or command markdown bodies (workaround: use scripts)

### CLAUDE_PLUGIN_OPTION_* Variables

**What they are**: Environment variables for plugin user config values

**Mapping**: Each option key in `userConfig` becomes `CLAUDE_PLUGIN_OPTION_<KEY>`

**Scope**: Available to all plugin subprocesses (scripts, command hooks, MCP servers)

**Example**:
```json
{
  "userConfig": {
    "api_key": {
      "type": "string",
      "sensitive": true,
      "description": "API key for service"
    },
    "endpoint": {
      "type": "string",
      "sensitive": false,
      "description": "API endpoint"
    }
  }
}
```

Usage in command hook:
```bash
#!/bin/bash
curl -H "Authorization: Bearer ${CLAUDE_PLUGIN_OPTION_api_key}" \
     "${CLAUDE_PLUGIN_OPTION_endpoint}/validate"
```

**Sensitive vs. Non-Sensitive**:
- **Sensitive** (`sensitive: true`): Only exported as env var; NOT stored in settings.json
- **Non-sensitive** (`sensitive: false`): Stored in settings.json AND exported as env var

### bin/ Directory on PATH

**Version**: Added in v2.1.91

**Behavior**: Executables in `plugin-name/bin/` are added to PATH when plugin is enabled

**Usage**: Bare command invocation from Bash tool (no path prefix needed)

**Example Structure**:
```
plugin-name/
├── bin/
│   ├── my-script
│   ├── my-tool
│   └── lib/
│       └── core.sh
└── commands/
    └── run.md
```

In command or bash:
```bash
my-script --flag
```

**Portability**: CLAUDE_PLUGIN_ROOT env var tells scripts where plugin is installed

### Related Variables

- `CLAUDE_SESSION_ID`: Current session identifier
- `CLAUDE_PLUGIN_NAME`: Name of the plugin being executed
- `${DEBUG:-false}`: Shell-style defaults in env vars

---

## 5. Source of Truth & Specification Authority

### Official Documentation

- **Primary**: [Claude Code Documentation](https://code.claude.com/docs/) (updated live)
  - Plugins reference: https://code.claude.com/docs/en/plugins-reference
  - Hooks reference: https://code.claude.com/docs/en/hooks
  - MCP reference: https://code.claude.com/docs/en/mcp
  - Output styles: https://code.claude.com/docs/en/output-styles

- **Fallback (redirects to code.claude.com)**: https://docs.claude.com/en/docs/claude-code/

### GitHub Repository

- **anthropics/claude-code**: https://github.com/anthropics/claude-code
  - **Source of implementation**: Core plugin system, hooks, MCP integration
  - **Plugin examples**: `/plugins/` directory contains 13+ official reference plugins
  - **CHANGELOG.md**: Release notes and breaking changes
  - **Schema references**: Plugin structure and validation

- **anthropics/claude-plugins-official**: https://github.com/anthropics/claude-plugins-official
  - **Official marketplace**: Curated plugins by Anthropic
  - **marketplace.json**: Example of canonical registry format
  - **Quality benchmarks**: Reference implementations for plugin developers

- **anthropics/skills**: https://github.com/anthropics/skills
  - **AgentSkills.io standard**: Open standard for skill format
  - **SKILL.md specification**: Core skill schema (extends to Claude Code)

### Current Version

- **Latest**: v2.1.207 (July 11, 2026)
- **Check releases**: https://github.com/anthropics/claude-code/releases

### Recent Plugin-Spec Changes (2026)

| Version | Change | Impact |
|---------|--------|--------|
| v2.1.207 | Shell injection prevention in hooks: reject `${user_config.*}` in shell form | Use exec arrays or env vars instead |
| v2.1.207 | Plugin options no longer read from project `.claude/settings.json` | Only user, --settings, managed settings honored |
| v2.1.205 | Fixed LSP-only plugins incorrectly flagged for disuse | LSP diagnostics + navigation no longer mark plugin unused |
| v2.1.203 | LSP server init failure no longer blocks other plugins' LSP servers | Graceful degradation on LSP error |
| v2.1.200 | Fixed duplicate skill instructions on re-invocation | Skills no longer append duplicate copies |
| v2.1.91 | Added `bin/` directory on PATH support | Plugin executables now directly invocable |

---

## 6. Converter-Relevant Concepts

### Claude Code Specific (No Direct Equivalent Elsewhere)

1. **Skills' Progressive Disclosure**
   - Level 1: Name + description only (~100 tokens, loaded at startup for discovery)
   - Level 2: Full skill content loaded on-demand
   - Unique to Claude Code; most plugin systems load full content always

2. **Prompt-Based Hooks**
   - Hooks can use natural-language prompts to Claude for evaluation
   - Command hooks return JSON via exit codes/stdout
   - Unique decision model vs. binary approval gates in other systems

3. **Monitors & Statusline**
   - Real-time session state exposed to plugins
   - Custom statusline components (informal standard, not formalized)
   - Plugins read context usage, active tools, running agents

4. **bin/ Directory on PATH**
   - Plugin executables automatically added to PATH when enabled
   - Portable via ${CLAUDE_PLUGIN_ROOT} env var
   - Most plugin systems require explicit path references

5. **${CLAUDE_PLUGIN_ROOT} Variable**
   - Absolute path to plugin installation directory
   - Enables portable plugin packaging (git clones to different paths)
   - Works in JSON configs but NOT in Markdown frontmatter

6. **Output Styles**
   - Modify system prompt directly (not per-task instructions)
   - Affect every response (not invocation-specific)
   - Plugins can force-apply via `force-for-plugin: true`
   - Contributes to prompt-cache cost

7. **Agent Subagents**
   - Full agentic capability (tools, model selection, isolation modes)
   - Support proactive auto-invocation via description field
   - Can run in worktree or remote isolation

8. **MCP Server Integration in Plugins**
   - Plugins bundle MCP servers alongside skills/agents
   - Environment variables substitution in configs
   - Channel binding for message injection into sessions

### Portable Concepts (Translatable to Other Formats)

1. **Slash Commands**
   - Simple markdown with optional frontmatter
   - Tool allowlisting, model selection
   - Maps to "custom commands" or "macro" systems in other editors

2. **Subagents/Agents**
   - Specialized AI assistants for specific tasks
   - Equivalent to: custom modes, specialized agents, personas
   - Tools + model + instructions pattern

3. **MCP Servers**
   - Open standard (Model Context Protocol)
   - Portable across Claude Code, Claude Desktop, APIs
   - HTTP + SSE transports work across platforms

4. **Hooks/Event Handlers**
   - Pre/post tool execution, session start/end lifecycle
   - Similar to: GitHub Actions hooks, pre-commit, shell hooks
   - Matcher-based routing pattern

5. **Marketplace Registry**
   - JSON-based plugin catalog
   - Source abstraction (git URL, git-subdir, local path)
   - Version pinning and checksums

6. **User Configuration**
   - Plugin options with type, description, sensitive flag
   - Exported as environment variables to subprocesses
   - Maps to: environment-based plugin config, credentials management

---

## 7. Plugin Validation & Packaging

### Official Plugin Examples

The anthropics/claude-code repository ships 13 reference plugins in `/plugins/`:

1. **agent-sdk-dev** — Claude Agent SDK development kit (setup wizard, validators)
2. **code-review** — Automated PR review (5 parallel specialized agents)
3. **commit-commands** — Git workflow automation (`/commit`, `/commit-push-pr`)
4. **explanatory-output-style** — Educational insights output style
5. **feature-dev** — 7-phase structured feature development
6. **frontend-design** — Production-grade frontend design guidance
7. **hookify** — Custom hook creation utility
8. **learning-output-style** — Interactive learning mode
9. **plugin-dev** — Comprehensive plugin development toolkit (7 skills, 3 agents)
10. **pr-review-toolkit** — Specialized PR review agents
11. **ralph-wiggum** — Autonomous iteration loops
12. **security-guidance** — Security vulnerability warning hooks
13. **claude-opus-4-5-migration** — Model migration automation

### Installation Commands

```bash
# From official marketplace
/plugin install code-review@claude-plugins-official

# From custom marketplace
/plugin marketplace add https://github.com/my-org/my-marketplace
/plugin install my-plugin@my-marketplace

# CLI non-interactive
claude plugin install code-review@claude-plugins-official --scope user
claude plugin marketplace update
```

---

## 8. Specification Governance

### Version & Compatibility

- Specifications evolve with Claude Code releases
- New features backward-compatible (old plugins continue working)
- Breaking changes rare and documented in CHANGELOG.md
- Minimum version gating via `dependencies` field in plugin.json

### Schema Validation

- Unofficial JSON Schema: https://github.com/hesreallyhim/claude-code-json-schema
- Official schema references inline in plugin.json comments
- Marketplace schema: https://anthropic.com/claude-code/marketplace.schema.json

### Documentation Updates

Watch for spec changes:
1. **GitHub releases**: https://github.com/anthropics/claude-code/releases (official announcements)
2. **CHANGELOG.md**: https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md (complete history)
3. **Docs site**: https://code.claude.com/docs/ (live documentation)
4. **Plugin dev skill**: https://github.com/anthropics/claude-code/tree/main/plugins/plugin-dev (reference implementations)

---

## Key Takeaways for Converter

1. **Two-file system**: `plugin.json` optional (auto-discovery fallback); manifest needed only for custom paths, metadata, dependencies
2. **Component registry pattern**: Each type (commands, agents, skills, etc.) has standardized path + frontmatter schema
3. **Environment-driven config**: Plugin options export as env vars; plugins receive context via stdin JSON (hooks)
4. **Portable via variables**: `${CLAUDE_PLUGIN_ROOT}` and `${CLAUDE_PLUGIN_OPTION_*}` enable cross-platform packaging
5. **Marketplace abstraction**: Source field abstracts location (git URL, subdir, local) and supports version pinning
6. **Formal standards**: AgentSkills.io for skills; MCP for external tools; emerging standards for CLI/API integration
7. **Proactive activation**: Agents and skills use description/trigger fields for auto-invocation (not static registration)
8. **Composability**: Skills load on-demand; agents inherit tool/permission context; hooks chain in parallel

---

## Sources

Official Anthropic Documentation:
- [Claude Code Plugins Reference](https://code.claude.com/docs/en/plugins-reference)
- [Hooks Reference](https://code.claude.com/docs/en/hooks)
- [MCP Reference](https://code.claude.com/docs/en/mcp)
- [Output Styles](https://code.claude.com/docs/en/output-styles)
- [Subagents](https://docs.claude.com/en/docs/claude-code/sub-agents)

GitHub Repositories:
- [anthropics/claude-code](https://github.com/anthropics/claude-code)
- [anthropics/claude-plugins-official](https://github.com/anthropics/claude-plugins-official)
- [anthropics/skills](https://github.com/anthropics/skills)

Component References:
- [Command development](https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/skills/command-development/references/frontmatter-reference.md)
- [Hook development](https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/skills/hook-development/SKILL.md)
- [Skill development](https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/skills/skill-development/SKILL.md)
- [Plugin structure](https://github.com/anthropics/claude-code/blob/main/plugins/plugin-dev/skills/plugin-structure/SKILL.md)

Release Information:
- [Claude Code Releases](https://github.com/anthropics/claude-code/releases)
- [CHANGELOG](https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md)

---

**End of Specification Reference**

*This document is a living snapshot of Claude Code's plugin ecosystem as of July 13, 2026, based on v2.1.207 release and live documentation.*
