# Extensions (PXB)

Extensions are **native binaries** that speak the **Phi eXtension Binary (PXB)**
protocol over stdin/stdout. They replace the former yaegi-interpreted `.go`
sources.

> **Security:** Extension processes inherit your permissions. Only install from sources you trust.

## Why not JSON lines?

PXB uses a fixed 16-byte little-endian header (`PXB\x01` + type + flags + id +
length) and **tagged-field** payloads (`tag u16 | kind u8 | value`). Decoders
skip unknown tags; new events are new `Ev*` codes; new frame types are skipped
by `payload_len`. See `ext/pxb` package doc for the full evolution rules.

On a hello-frame microbenchmark (Apple M4), PXB remains far cheaper than a
comparable JSONL object — see `go test ./ext/pxb -bench=.`.

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
	"github.com/pulseaiclub/phi/ext/sdk"
)

func main() {
	m := sdk.New("hello", "0.1.0")
	m.RegisterCommand("hello", ext.CommandDef{
		Description: "Say hi",
		Handler: func(args string, ctx *ext.Context) error {
			m.Notify("info", "Hello!")
			return nil
		},
	})
	m.OnToolCall(func(ev ext.ToolCallEvent) *ext.ToolCallResult {
		// return &ext.ToolCallResult{Block: true, Reason: "..."} to deny
		return nil
	})
	_ = m.Run()
}
```

Build and install:

```bash
go build -o hello .
mkdir -p ~/.phi/extensions/hello
cp hello phi.yaml ~/.phi/extensions/hello/
```

Sample: [examples/extensions/hello](../examples/extensions/hello).

Reload: **Ctrl+K → extensions → reload**.

## Install from GitHub

```bash
phi plugin install alice/greet
```

The repo must ship `phi.yaml` **and** the compiled `exec` binary (or a
release asset layout that includes it). Source-only yaegi repos no longer load.

## Lifecycle

1. Discover `phi.yaml`
2. Spawn `exec` (stderr → `~/.phi/logs/ext-<name>.log`)
3. Ext → `Hello` · Host → `HelloAck`
4. Ext → `Register*` / `Subscribe` · Ext → `Ready`
5. Runtime RPC (`CommandInvoked`, `ToolInvoke`, `Intercept`, `Event`)
6. Host → `Shutdown` · Ext → `ShutdownAck` (then SIGKILL if needed)

Tool loop order remains **ExtensionPre → Gate/Ask → Run → ExtensionPost**.

## Packages

| Path | Role |
|------|------|
| `ext/` | Shared types (`ToolDef`, events) |
| `ext/pxb` | Binary wire protocol |
| `ext/sdk` | Author SDK (`Module.Run`) |
| `internal/extension` | Discover, spawn, Runner shims |

## Migration from yaegi

| Old | New |
|-----|-----|
| `func Extension(phi *ext.API)` in `.go` | `sdk.New` + `m.Run()` binary |
| Drop file under `extensions/` | Directory + `phi.yaml` + binary |
| `phi.On(ext.EventToolCall, …)` | `m.OnToolCall(…)` |

## Migration from shell hooks

| Old hook | Extension |
|----------|-----------|
| PreToolUse deny | `OnToolCall` → `Block: true` |
| PostToolUse context | `OnToolResult` → `Context` |
| Command slash | `RegisterCommand` |
