# Extensions (PXB)

Extensions are **native binaries** that speak the **Phi eXtension Binary (PXB)**
protocol over stdin/stdout. They replace the former yaegi-interpreted `.go`
sources.

> **Security:** Extension processes inherit your permissions. Only install from sources you trust.

## Full event chain

Shaped for subprocess PXB:

```
user_input ──(transform|handled)──► before_agent_start ──► agent_start
  └─ turn_start → LLM → tool_execution_start → tool_call → Gate/Ask → Run
       → tool_result → tool_execution_end → turn_end
  └─ (no tools) turn_stopping ──(continue+message)──► another turn
  └─ agent_end
session_before_switch → session_shutdown → session_start
session_compact (after auto compaction)
```

| Event | Mode | Can |
|-------|------|-----|
| `user_input` | intercept | rewrite text / `Handled` swallow |
| `before_agent_start` | intercept | rewrite prompt / append context |
| `tool_call` | intercept | block / rewrite args / context |
| `tool_result` | intercept | rewrite content / context / **Stop** agent loop |
| `turn_stopping` | intercept | `Continue` + steer message |
| `session_before_switch` | intercept | cancel switch |
| `session_*` / `agent_*` / `turn_*` / `tool_execution_*` / `session_compact` | subscribe | observe (payload via `SubscribeEvent`) |

Tool loop order remains **ExtensionPre → Gate/Ask → Run → ExtensionPost**.

## Why not JSON lines?

PXB uses a fixed 16-byte little-endian header (`PXB\x01` + type + flags + id +
length) and **tagged-field** payloads (`tag u16 | kind u8 | value`). Decoders
skip unknown tags; new events are new `Ev*` codes; new frame types are skipped
by `payload_len`. See `ext/pxb` package doc for the full evolution rules.

## Layout

| Location | Scope |
|----------|-------|
| `~/.phi/extensions/<name>/phi.yaml` | Global |
| `<cwd>/.phi/extensions/<name>/phi.yaml` | Project-local |

Same name: project wins. Disable all with `PHI_EXTENSIONS=off`.

### `phi.yaml`

```yaml
name: hello
version: "0.1.0"
exec: ./hello          # relative to this directory
description: optional
enabled: true          # optional, default true
```

## Authoring (Go SDK)

```go
package main

import (
	"github.com/pulseaiclub/phi/ext"
	"github.com/pulseaiclub/phi/ext/phi"
)

func main() {
	m := phi.New("hello", "0.1.0")
	m.RegisterCommand("hello", ext.Command{
		Description: "Say hi",
		Handler: func(args string, ctx *ext.Context) error {
			m.Notify("info", "Hello!")
			// m.Submit("follow-up") // after /hello returns
			// m.SendUserMessage("…") // enqueue a turn anytime
			return nil
		},
	})
	m.OnUserInput(func(ev ext.UserInputEvent) *ext.UserInputResult {
		// return &ext.UserInputResult{Handled: true} to swallow
		// return &ext.UserInputResult{Text: "rewritten"} to transform
		return nil
	})
	m.OnToolCall(func(ev ext.ToolCallEvent) *ext.ToolCallResult {
		// return &ext.ToolCallResult{Block: true, Reason: "..."} to deny
		return nil
	})
	m.OnToolResult(func(ev ext.ToolResultEvent) *ext.ToolResultResult {
		// return &ext.ToolResultResult{Stop: true} to end the agent loop
		return nil
	})
	m.OnTurnStopping(func(ev ext.TurnStoppingEvent) *ext.TurnStoppingResult {
		// return &ext.TurnStoppingResult{Continue: true, Message: "check X"} to steer
		return nil
	})
	m.SubscribeEvent(ext.EventSessionStart, func(ev pxb.EventNotify) {
		_ = ev // Reason, PreviousSessionID, …
	})
	_ = m.Run()
}
```

Requires `import "github.com/pulseaiclub/phi/ext/pxb"` for `SubscribeEvent` payloads.

UI surface today: **toast** (`Notify`), **footer status** (`SetStatus`), **Submit** / **SendUserMessage**,
**Confirm** / **ConfirmOpts**.

```go
ok := m.ConfirmOpts(ext.ConfirmRequest{
	Title: "Delete?", Message: "Remove /tmp/x", Yes: "Delete", No: "Cancel", Danger: true,
}).OK
```

Build and install:

```bash
go build -o hello .
mkdir -p ~/.phi/extensions/hello
cp hello phi.yaml ~/.phi/extensions/hello/
```

Samples:
- [examples/extensions/hello](../examples/extensions/hello) — minimal slash + notify
- [examples/extensions/showcase](../examples/extensions/showcase) — Confirm, intercepts, tools

Reload: **Ctrl+K → extensions → reload**.

## Install from GitHub

```bash
phi plugin install alice/greet
```

The repo must ship `phi.yaml` **and** the compiled `exec` binary (or a
release asset layout that includes it). Source-only yaegi repos no longer load.

## Lifecycle (process)

1. Discover `phi.yaml`
2. Spawn `exec` (stderr → `~/.phi/logs/ext-<name>.log`)
3. Ext → `Hello` · Host → `HelloAck`
4. Ext → `Register*` / `Subscribe` · Ext → `Ready`
5. Runtime RPC (`CommandInvoked`, `ToolInvoke`, `Intercept`, `Event`, `HostRequest`, `SessionMeta`)
6. Host → `Shutdown` · Ext → `ShutdownAck` (then SIGKILL if needed)

## Packages

| Path | Role |
|------|------|
| `ext/` | Shared types (`Tool`, events) |
| `ext/pxb` | Binary wire protocol |
| `ext/phi` | Author SDK (`ExtensionAPI.Run`) |
| `internal/extension` | Discover, spawn, Runner shims |

## Migration from yaegi

| Old | New |
|-----|-----|
| `func Extension(phi *ext.API)` in `.go` | `phi.New` + `m.Run()` binary |
| Drop file under `extensions/` | Directory + `phi.yaml` + binary |
| `phi.On(ext.EventToolCall, …)` | `m.OnToolCall(…)` |

## Migration from shell hooks

| Old hook | Extension |
|----------|-----------|
| PreToolUse deny | `OnToolCall` → `Block: true` |
| PostToolUse context | `OnToolResult` → `Context` |
| Stop / steer | `OnTurnStopping` / `OnToolResult{Stop}` |
| UserPromptSubmit | `OnUserInput` |
| Command slash | `RegisterCommand` |
