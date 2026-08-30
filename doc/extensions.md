# Extensions

Extensions are Go source files loaded with [yaegi](https://github.com/traefik/yaegi). They replace the former shell `plugin.json` hooks system.

> **Security:** Extensions run with your full process permissions. Only install from sources you trust.

## Locations

| Location | Scope |
|----------|-------|
| `~/.phi/extensions/*.go` | Global (all projects) |
| `~/.phi/extensions/*/index.go` | Global (subdirectory) |
| `<cwd>/.phi/extensions/*.go` | Project-local |
| `<cwd>/.phi/extensions/*/index.go` | Project-local (subdirectory) |

Same extension id (file stem or directory name): project replaces user. Disable all with `PHI_EXTENSIONS=off`.

## Quick start

Create `~/.phi/extensions/hello.go` (or use the sample under `.phi/extensions/`):

```go
package main

import (
	"context"
	"encoding/json"
	"github.com/pulseaiclub/phi/ext"
)

func Extension(phi *ext.API) {
	phi.RegisterTool(ext.ToolDef{
		Name:        "greet",
		Description: "Greet someone by name",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (ext.ToolResult, error) {
			var in struct{ Name string `json:"name"` }
			_ = json.Unmarshal(args, &in)
			return ext.ToolResult{Content: "Hello, " + in.Name + "!"}, nil
		},
	})

	phi.On(ext.EventToolCall, func(ev ext.ToolCallEvent, ctx *ext.Context) *ext.ToolCallResult {
		// return &ext.ToolCallResult{Block: true, Reason: "..."} to deny
		return nil
	})

	phi.RegisterCommand("hello", ext.CommandDef{
		Description: "Say hello",
		Handler: func(args string, ctx *ext.Context) error {
			if ctx.UI != nil {
				ctx.UI.Notify("Hello!", "info")
			}
			return nil
		},
	})
}
```

Entry point: export `func Extension(phi *ext.API)` in `package main`.

Reload in TUI: **Ctrl+K → extensions → reload**. List: **extensions → list**.

## Events

| Event | When | Result |
|-------|------|--------|
| `tool_call` | Before permission Gate | `{Block, Reason, Input, Context}` |
| `tool_result` | After tool run | `{Content, Context, Stop, Reason}` |
| `tool_execution_start` / `tool_execution_end` | Around tool run | notify |
| `session_start` / `session_shutdown` / `session_before_switch` | Session lifecycle | before_switch may `{Cancel}` |
| `before_agent_start` | After user submit | `{SystemPromptAppend}` |
| `agent_start` / `agent_end` | Around Loop | notify |
| `turn_start` / `turn_end` | Per LLM round | notify |

Tool loop order remains **ExtensionPre → Gate/Ask → Run → ExtensionPost** (does not bypass the permission gate).

Model-only notes from handlers are wrapped in `<ext_context>…</ext_context>` on the tool message (TUI Detail/Output unchanged).

## API

Import `github.com/pulseaiclub/phi/ext` (symbols are injected into yaegi; no extra module download needed for the `ext` package itself).

| Method | Purpose |
|--------|---------|
| `On(event, handler)` | Subscribe |
| `RegisterTool(ToolDef)` | LLM-callable tool |
| `RegisterCommand(name, CommandDef)` | Slash command (cannot override builtins) |
| `GetActiveTools` / `SetActiveTools` / `GetAllTools` | Tool set (after host bind) |
| `Exec` / `SendUserMessage` | Host actions (after bind) |

`ctx.UI`: `Notify`, `Confirm`, `SetStatus` (TUI). Headless `phi run` may have no UI.

## Migration from hooks

Shell `plugin.json` hooks under `~/.phi/hooks` / `.phi/hooks` are **removed**. Rewrite policy as Go:

| Old hook | Extension |
|----------|-----------|
| PreToolUse deny | `On(EventToolCall, …)` → `Block: true` |
| PostToolUse context | `On(EventToolResult, …)` → `Context` |
| Command slash | `RegisterCommand` |
| SessionStart / Shutdown / BeforeSwitch | matching `session_*` events |

## Layout

| Path | Role |
|------|------|
| `ext/` | Public types + `API` for authors |
| `internal/extension/` | Discover, yaegi loader, Runner |
| `.phi/extensions/` | Project samples (`hello.go`, `guard_bash.go`) |
