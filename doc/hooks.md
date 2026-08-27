# Hooks

Hooks let you run custom logic around each tool call—before permission gating and after execution—without changing Phi’s binary or putting settings into `config.yaml`.

Use hooks when you need organization policy, audit trails, or input rewriting that the permission Gate does not cover.

| Audience | This document |
| --- | --- |
| Hook authors | Create scripts under `.phi/hooks/<plugin>/` |
| Operators | Deploy user- or project-level policy |
| Contributors | See [Related code](#related-code) |

Configuration is an event map: events as keys, matchers, and `type: "command"` shell commands.

---

## Concepts

### Execution order

```text
emit(InProgress)
  → PreToolUse        (deny | modify input)
  → Gate              (Ask UI / permission rules)
  → tool.Run
  → PostToolUse / PostToolUseFailure
  → emit(Done | …)
```

- **PreToolUse** runs before Gate. A deny stops the tool without user approval.
- **PostToolUse** runs after a successful tool run. Hooks can append model-facing `additionalContext` and/or rewrite output (`updatedMCPToolOutput`).
- **PostToolUseFailure** runs when the tool returns an error (same response fields as PostToolUse).
- Aggregated context is wrapped in `<hook_context>…</hook_context>` on the tool result sent to the model only. TUI Detail/Output are unchanged by context.
- Output rewrites affect both the model-facing tool content and the TUI Output string (Detail is unchanged).
- If no hooks are loaded, behavior matches a build with hooks disabled.

### Supported events

| Event | When | `matcher` matches |
| --- | --- | --- |
| `PreToolUse` | Before tool + Gate | Tool name |
| `PostToolUse` | After successful tool run | Tool name |
| `PostToolUseFailure` | After tool error | Tool name |
| `SessionStart` | Session ready (`startup` / `new` / `resume`) | `source` |
| `SessionShutdown` | Active session left (`new` / `resume` / `quit`) — session file may remain | `reason` |
| `SessionBeforeSwitch` | Before `/clear` or `/resume` replaces the engine (`reason`: `new` or `resume`) | `reason` |
| `PostTurn` | After each completed assistant stream (TUI) | Ignored (all bindings run) |
| `Command` | TUI slash command (`/name`) | Slash command name |

### Discovery model

One **plugin** is one directory with a `plugin.json` plus scripts. Phi loads every such directory under the hooks root (one level only). An optional `plugin.json` directly in the hooks root is treated as plugin id `root`.

```text
~/.phi/hooks/                    # user (lower)
  org-policy/
    plugin.json
    guard.sh
  secrets-scan/
    plugin.json
    scan.py

<cwd>/.phi/hooks/                # project (higher; same plugin id replaces user)
  try-hooks/
    plugin.json
    guard.py
```

| Scope | Path | Precedence |
| --- | --- | --- |
| User | `~/.phi/hooks/<plugin>/plugin.json` (and optional `~/.phi/hooks/plugin.json`) | Lower |
| Project | `<cwd>/.phi/hooks/<plugin>/plugin.json` (and optional `<cwd>/.phi/hooks/plugin.json`) | Higher — same **plugin id** (directory name, or `root`) replaces the user plugin entirely |

- Phi creates an empty `~/.phi/hooks/` on startup if needed.
- `command` strings run through the shell (`$SHELL -c` on Unix). Working directory is the directory that contains `plugin.json`.
- Missing `plugin.json` is fine. Parse errors produce warnings and do not block startup.
- Duplicate plugin ids in the same scope: first wins; later files warn and skip.
- Set `PHI_HOOKS=off` to disable discovery and execution entirely.

---

## Getting started

This repo ships a sample plugin at `.phi/hooks/try-hooks/`.

### 1. Create a project plugin

```text
.phi/hooks/guard-bash/
  plugin.json
  guard.sh
```

**`plugin.json`**

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "bash",
        "hooks": [
          {
            "type": "command",
            "command": "./guard.sh",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

**`guard.sh`** (`chmod +x guard.sh`)

```bash
#!/usr/bin/env bash
# Deny bash when tool_input contains "phi-deny".
input=$(cat)
case "$input" in
  *phi-deny*)
    printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"blocked (phi-deny)"}}'
    exit 2
    ;;
esac
printf '%s\n' '{}'
```

### 2. Load hooks

- Restart Phi, or
- Command palette: **hooks → reload** (`Ctrl+K`)

List loaded plugins with **hooks → list**.

### 3. Verify

Ask the agent to run `echo phi-deny`. The PreToolUse hook should deny the call.

Try `echo phi-rewrite` to see input rewriting (`updatedInput`).

Any successful tool should get a PostToolUse stamp from `try-hooks/stamp.py` when that plugin is loaded.

---

## Authoring guide

### Manifest (`plugin.json`)

Top-level object with a `hooks` map: event name → array of matchers.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "./lint.sh",
            "timeout": 30,
            "async": true
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [{ "type": "command", "command": "echo hello" }]
      }
    ]
  }
}
```

| Field (hook command) | Type | Default | Description |
| --- | --- | --- | --- |
| `type` | string | `command` | Only `command` is supported |
| `command` | string | required | Shell command (may include args, e.g. `./lint.sh --strict`) |
| `shell` | string | bash | `bash` (uses `$SHELL`), `powershell`, or `pwsh` |
| `timeout` | number | `5` | Seconds; capped at `60` |
| `async` | boolean | `false` | Fire-and-forget; result ignored |

| Field (matcher) | Type | Default | Description |
| --- | --- | --- | --- |
| `matcher` | string | `*` | Pattern against tool name (Pre/Post) or `source` / `reason` (session events) |
| `hooks` | array | required | Command hooks for this matcher |

**Matcher syntax** (tool names and session fields):

1. Empty or `*` → match all.
2. `Write|Edit` → exact match on any pipe-separated name.
3. Otherwise → regular expression (e.g. `^bash$`).

### PreToolUse response

Write one JSON object on stdout (first line). Empty stdout with exit `0` means allow.

**Preferred shape:**

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "policy violation",
    "updatedInput": { "command": "echo safe" },
    "additionalContext": "optional note"
  }
}
```

**Legacy Phi shape** (still parsed when `hookSpecificOutput` is absent):

```json
{ "action": "allow" }
{ "action": "deny", "reason": "policy violation" }
{ "action": "modify", "input": { "command": "echo safe" } }
```

| Exit code | Behavior |
| --- | --- |
| `0` | Parse stdout; empty body → allow |
| `2` | Hard deny (stderr optional; stdout JSON can add `reason`) |
| other | Non-blocking error — hook skipped, tool loop continues |

`permissionDecision`: `allow` | `deny` | `ask` (`ask` falls through to Gate).

### PostToolUse / PostToolUseFailure response

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PostToolUse",
    "additionalContext": "note for the model",
    "updatedMCPToolOutput": "rewritten tool result text"
  }
}
```

Legacy: `{ "context": "…", "output": "…", "stop": false }`.

| Field | Effect |
| --- | --- |
| `additionalContext` / `context` | Model-only note; aggregated (joined; capped at 4 KiB) |
| `updatedMCPToolOutput` / `output` | Rewrites tool result for model and TUI Output; last sync hook wins |
| `stop` / exit `2` | Stop signal (not yet wired into the agent loop) |

`async: true` hooks are fire-and-forget — stdout is ignored.

### Command (`Command`)

Registers a TUI slash command. `matcher` is the command name (case-insensitive at runtime). `/review` runs the matching hook.

```json
{
  "hooks": {
    "Command": [
      {
        "matcher": "review",
        "hooks": [{ "type": "command", "command": "./review.sh" }]
      }
    ]
  }
}
```

stdin:

```json
{
  "session_id": "…",
  "cwd": "/path/to/project",
  "hook_event_name": "Command",
  "command": "review",
  "args": ["the", "diff"]
}
```

stdout (first JSON line). Apply order: **status** → **toast** → **list** → **submit** (submit skipped when `list` is set).

```json
{ "submit": "optional user message", "toast": "done", "status": "footer text", "list": { "title": "Findings", "items": [] } }
```

### SessionBeforeSwitch / PostTurn

**SessionBeforeSwitch** runs serially; first deny wins (`action: deny` or exit `2`). Matcher filters on `reason` (`new`, `resume`, …).

**PostTurn** is audit-only (stdout not injected into the model). Runs in parallel; `async: true` recommended.

stdin adds `message_id` and `usage` on **PostTurn**; **SessionBeforeSwitch** may include `target_session_id`.

### SessionStart / SessionShutdown

Phi uses **`SessionShutdown`**, not CC **`SessionEnd`**: hooks run when the TUI **stops using** the current session (switch or quit), not when a session is deleted from disk. `SessionEnd` in `plugin.json` is accepted as a deprecated alias.

| Event | Phi `reason` / `source` | Matcher value |
| --- | --- | --- |
| `SessionStart` | `startup`, `new`, `resume` | `source` |
| `SessionShutdown` | `new`, `resume`, `quit` | `reason` |

stdin includes `session_id`, `cwd`, `hook_event_name`, and session fields. stdout may set `systemMessage` or `hookSpecificOutput.initialUserMessage` (shown as toast).

### Ordering and concurrency

- Matching **PreToolUse** hooks run **serially**. First deny wins; `updatedInput` chains.
- Matching **PostToolUse** / **PostToolUseFailure** hooks run **in parallel** (except `async`, detached).
- **SessionBeforeSwitch** runs **serially**; first deny wins.
- **SessionStart** / **SessionShutdown** run in parallel (except `async`).
- **PostTurn** runs in parallel; stdout is audit-only (ignored).
- **Command** runs a single matching hook (first binding wins).
- Order across multiple hooks is **not** guaranteed — combine logic in one command when order matters.

### Result aggregation

| Field | PreToolUse | PostToolUse / Failure | Session* | Command |
| --- | --- | --- | --- | --- |
| `additionalContext` / `context` | `\n\n` join, 4 KiB cap | `\n\n` join, 4 KiB cap | N/A (not injected to model) | N/A |
| `updatedInput` | Chain (serial) | — | — | — |
| `updatedMCPToolOutput` / `output` | — | Last sync hook wins | — | — |
| deny / block | First deny stops | exit `2` → stop flag | BeforeSwitch: first deny | exit ≠ 0 → error |
| toast / status | — | — | Last hook wins | One hook only |

Pre and Post context are both injected into the model tool result (inside `<hook_context>`). `async: true` hooks never contribute to merged results.

---

## Protocol reference

External hooks receive one JSON line on stdin and may write one JSON line on stdout. stdout/stderr are capped at **1 MiB** each. Aggregated model context is capped at **4 KiB**.

### Request (stdin)

```json
{
  "session_id": "…",
  "cwd": "/path/to/project",
  "hook_event_name": "PreToolUse",
  "tool_name": "bash",
  "tool_use_id": "call_…",
  "tool_input": { "command": "ls" }
}
```

| Field | PreToolUse | PostToolUse | PostToolUseFailure | SessionStart | SessionShutdown | SessionBeforeSwitch | PostTurn | Command |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `hook_event_name` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `tool_name` | ✓ | ✓ | ✓ | — | — | — | — | — |
| `tool_use_id` | ✓ | ✓ | ✓ | — | — | — | — | — |
| `tool_input` | ✓ | ✓ | ✓ | — | — | — | — | — |
| `tool_response` | — | ✓ | — | — | — | — | — | — |
| `error` | — | — | ✓ | — | — | — | — | — |
| `source` | — | — | — | ✓ | — | — | — | — |
| `reason` | — | — | — | — | ✓ | ✓ | — | — |
| `previous_session_id` | — | — | — | ✓ | ✓ | — | — | — |
| `target_session_id` | — | — | — | — | — | ✓ | — | — |
| `message_id` | — | — | — | — | — | — | ✓ | — |
| `usage` | — | — | — | — | ✓ | ✓ | — | — |
| `command` / `args` | — | — | — | — | — | — | — | ✓ |

Events not in the table above use the same base fields (`session_id`, `cwd`, `hook_event_name`) plus event-specific fields documented in [Authoring guide](#authoring-guide).

### Environment

Sensitive parent environment keys are stripped before spawn.

| Variable | Value |
| --- | --- |
| `PHI_HOOK_EVENT` | `PreToolUse`, `PostToolUse`, etc. |
| `PHI_SESSION_ID` | Session id |
| `PHI_CWD` | Workspace cwd |
| `PHI_PROJECT_DIR` | Same as cwd |
| `PHI_PLUGIN_ROOT` | Directory containing `plugin.json` |

---

## Operations

| Action | How |
| --- | --- |
| Disable all hooks | `PHI_HOOKS=off` |
| Inspect load warnings | `PHI_DEBUG=1` |
| List / reload in TUI | `Ctrl+K` → **hooks → list** / **hooks → reload** |
| Override a user plugin | Same plugin id under `<cwd>/.phi/hooks/<plugin>/` |

Configuration for hooks is **not** stored in `~/.phi/config.yaml`.

In `permissions.mode: readonly`, all loaded hooks still run (there is no `fail_closed` flag in the v1 schema). Use fast hooks or `async` for audit-only work.

---

## Limitations

- No file-watch hot reload (use palette reload or restart)
- Hooks cannot register new tools (use `tooldef.Tool`)
- Only `type: "command"` hooks (no prompt / agent / http yet)

---

## Related code

| Path | Role |
| --- | --- |
| `internal/hooks/` | Types, Manager, discovery, shell execution, Load |
| `internal/agent/executor.go` | PreToolUse → Gate → Run → PostToolUse |
| `internal/project` | `HooksDir()`, directory bootstrap |
| `internal/tui` | Engine wiring; list / reload |
