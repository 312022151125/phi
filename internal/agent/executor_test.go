package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/hooks"
	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/permission"
	"github.com/pulseaiclub/phi/internal/session"
	"github.com/pulseaiclub/phi/internal/tools"
)

func writeToolHookPlugin(t *testing.T, event hooks.HookEvent, matcher, scriptBody string) *hooks.Manager {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "hook.sh")
	require.NoError(t, os.WriteFile(script, []byte(scriptBody), 0o644))
	require.NoError(t, os.Chmod(script, 0o755))
	matcherField := ""
	if matcher != "" {
		matcherField = fmt.Sprintf(`"matcher": %q,`, matcher)
	}
	plugin := fmt.Sprintf(`{"hooks":{%q:[{%s"hooks":[{"command":"./hook.sh"}]}]}}`, event, matcherField)
	require.NoError(t, os.WriteFile(filepath.Join(dir, hooks.PluginFileName), []byte(plugin), 0o644))
	mgr, _, err := hooks.Load(dir, "")
	require.NoError(t, err)
	return mgr
}

type fixedGate struct {
	dec    permission.Decision
	reason string
}

func (g fixedGate) Check(context.Context, permission.Request) (permission.Decision, string) {
	return g.dec, g.reason
}

func TestExecutorDenyDoesNotRunHandler(t *testing.T) {
	var ran atomic.Int32
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				ran.Add(1)
				return tools.Result{Content: "ok"}, nil
			},
		},
	}
	ex := NewExecutor(reg, fixedGate{dec: permission.Deny, reason: "denied by test"}, nil, nil)
	var statuses []session.ToolStatus
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"echo hi"}`},
	}}, func(td session.ToolData) bool {
		statuses = append(statuses, td.Run.Status)
		return true
	})
	if ran.Load() != 0 {
		t.Fatal("handler should not run on deny")
	}
	if len(msgs) != 1 || msgs[0].Content != "denied by test" {
		t.Fatalf("tool message: %+v", msgs)
	}
	found := false
	for _, s := range statuses {
		if s == session.ToolRejected {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ToolRejected in %v", statuses)
	}
}

func TestExecutorAskFalseRejects(t *testing.T) {
	var ran atomic.Int32
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				ran.Add(1)
				return tools.Result{Content: "ok"}, nil
			},
		},
	}
	ask := func(context.Context, permission.Request, string) (permission.AskResult, error) {
		return permission.AskResult{Approved: false}, nil
	}
	ex := NewExecutor(reg, fixedGate{dec: permission.Ask, reason: "needs approval"}, ask, nil)
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"curl x"}`},
	}}, func(session.ToolData) bool { return true })
	if ran.Load() != 0 {
		t.Fatal("handler should not run when ask denied")
	}
	if len(msgs) != 1 || msgs[0].Content == "" {
		t.Fatalf("expected rejection message, got %+v", msgs)
	}
}

func TestExecutorEmitsToolName(t *testing.T) {
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				return tools.Result{Content: "ok"}, nil
			},
		},
	}
	ex := NewExecutor(reg, permission.AllowAll{}, nil, nil)
	var names []string
	ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"pwd"}`},
	}}, func(td session.ToolData) bool {
		names = append(names, td.Run.Name)
		return true
	})
	if len(names) == 0 {
		t.Fatal("expected tool events")
	}
	for _, n := range names {
		if n != "bash" {
			t.Fatalf("expected Name=bash on every ToolData, got %q in %v", n, names)
		}
	}
}

func TestExecutorAskNilRejectsHeadless(t *testing.T) {
	// Headless mode wires Ask=nil: an Ask decision must reject without
	// running the handler (Ask≡Deny), even if the gate did not fold it.
	var ran atomic.Int32
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				ran.Add(1)
				return tools.Result{Content: "ok"}, nil
			},
		},
	}
	ex := NewExecutor(reg, fixedGate{dec: permission.Ask, reason: "needs approval"}, nil, nil)
	var statuses []session.ToolStatus
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"rm -rf /tmp/x"}`},
	}}, func(td session.ToolData) bool {
		statuses = append(statuses, td.Run.Status)
		return true
	})
	if ran.Load() != 0 {
		t.Fatal("handler should not run when ask handler is nil (headless)")
	}
	if len(msgs) != 1 || msgs[0].Content == "" {
		t.Fatalf("expected rejection message, got %+v", msgs)
	}
	found := false
	for _, s := range statuses {
		if s == session.ToolRejected {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ToolRejected in %v", statuses)
	}
}

func TestExecutorAskTrueRuns(t *testing.T) {
	var ran atomic.Int32
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				ran.Add(1)
				return tools.Result{Content: "ran"}, nil
			},
		},
	}
	ask := func(context.Context, permission.Request, string) (permission.AskResult, error) {
		return permission.AskResult{Approved: true}, nil
	}
	ex := NewExecutor(reg, fixedGate{dec: permission.Ask, reason: "needs approval"}, ask, nil)
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"curl x"}`},
	}}, func(session.ToolData) bool { return true })
	if ran.Load() != 1 {
		t.Fatal("handler should run when ask approved")
	}
	if len(msgs) != 1 || msgs[0].Content != "ran" {
		t.Fatalf("got %+v", msgs)
	}
}

func TestExecutorAskFeedbackMessage(t *testing.T) {
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				return tools.Result{Content: "ok"}, nil
			},
		},
	}
	ask := func(context.Context, permission.Request, string) (permission.AskResult, error) {
		return permission.AskResult{Approved: false, Feedback: "use go test instead"}, nil
	}
	ex := NewExecutor(reg, fixedGate{dec: permission.Ask, reason: "ask me"}, ask, nil)
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"curl x"}`},
	}}, func(session.ToolData) bool { return true })
	if len(msgs) != 1 || !strings.Contains(msgs[0].Content, "use go test instead") {
		t.Fatalf("expected feedback in message, got %+v", msgs)
	}
}

func TestExecutorNilAskOnAskDenies(t *testing.T) {
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				return tools.Result{Content: "ok"}, nil
			},
		},
	}
	ex := NewExecutor(reg, fixedGate{dec: permission.Ask, reason: "ask me"}, nil, nil)
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"curl x"}`},
	}}, func(session.ToolData) bool { return true })
	if len(msgs) != 1 || msgs[0].Content == "" {
		t.Fatalf("expected deny message, got %+v", msgs)
	}
}

func TestExecutorHookDenySkipsGateAsk(t *testing.T) {
	var ran atomic.Int32
	var askCalled atomic.Int32
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				ran.Add(1)
				return tools.Result{Content: "ok"}, nil
			},
		},
	}
	ask := func(context.Context, permission.Request, string) (permission.AskResult, error) {
		askCalled.Add(1)
		return permission.AskResult{Approved: true}, nil
	}
	mgr := writeToolHookPlugin(t, hooks.EventPreToolUse, "bash", `#!/bin/sh
printf '%s\n' '{"action":"deny","reason":"hook blocked"}'
exit 2
`)
	ex := NewExecutor(reg, fixedGate{dec: permission.Ask, reason: "needs approval"}, ask, mgr)
	var statuses []session.ToolStatus
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"rm -rf /"}`},
	}}, func(td session.ToolData) bool {
		statuses = append(statuses, td.Run.Status)
		return true
	})
	if ran.Load() != 0 {
		t.Fatal("handler must not run when hook denies")
	}
	if askCalled.Load() != 0 {
		t.Fatal("gate Ask must not run when hook denies")
	}
	if len(msgs) != 1 || msgs[0].Content != "hook blocked" {
		t.Fatalf("tool message: %+v", msgs)
	}
	found := false
	for _, s := range statuses {
		if s == session.ToolRejected {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ToolRejected in %v", statuses)
	}
}

func TestExecutorHookModifySeenByGateAndRun(t *testing.T) {
	var sawArgs string
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			DetailFromArgs: func(input json.RawMessage) string {
				var in struct {
					Command string `json:"command"`
				}
				_ = json.Unmarshal(input, &in)
				return in.Command
			},
			Run: func(_ context.Context, input json.RawMessage) (tools.Result, error) {
				sawArgs = string(input)
				return tools.Result{Content: "ran", Output: "ran"}, nil
			},
		},
	}
	gate := &recordingGate{}
	mgr := writeToolHookPlugin(t, hooks.EventPreToolUse, "bash", `#!/bin/sh
cat >/dev/null
printf '%s' '{"action":"modify","input":{"command":"echo safe"}}'
`)
	ex := NewExecutor(reg, gate, nil, mgr)
	var detail string
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"rm -rf /"}`},
	}}, func(td session.ToolData) bool {
		if td.Run.Status == session.ToolDone {
			detail = td.Run.Detail
		}
		return true
	})
	if gate.last.Command != "echo safe" {
		t.Fatalf("gate saw command %q, want modified", gate.last.Command)
	}
	if !strings.Contains(sawArgs, "echo safe") {
		t.Fatalf("handler saw %q", sawArgs)
	}
	if detail != "echo safe" {
		t.Fatalf("UI detail %q", detail)
	}
	if len(msgs) != 1 || msgs[0].Content != "ran" {
		t.Fatalf("got %+v", msgs)
	}
}

func TestExecutorHookPostContextOnModelOnly(t *testing.T) {
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				return tools.Result{Content: "ok", Output: "ok"}, nil
			},
		},
	}
	mgr := writeToolHookPlugin(t, hooks.EventPostToolUse, "", `#!/bin/sh
cat >/dev/null
printf '%s' '{"context":"policy note"}'
`)
	ex := NewExecutor(reg, permission.AllowAll{}, nil, mgr)
	var uiOut string
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"pwd"}`},
	}}, func(td session.ToolData) bool {
		if td.Run.Status == session.ToolDone {
			uiOut = td.Run.Output
		}
		return true
	})
	if uiOut != "ok" {
		t.Fatalf("TUI output should stay clean, got %q", uiOut)
	}
	if len(msgs) != 1 {
		t.Fatalf("msgs: %+v", msgs)
	}
	if !strings.Contains(msgs[0].Content, "ok") {
		t.Fatalf("content missing tool result: %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "<hook_context>") || !strings.Contains(msgs[0].Content, "policy note") {
		t.Fatalf("content missing hook context: %q", msgs[0].Content)
	}
}

func TestExecutorReadonlyHookDenyStillBlocks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture")
	}
	dir := t.TempDir()
	writeScript := func(name, body string) {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
		require.NoError(t, os.Chmod(path, 0o755))
	}
	writeScript("audit.sh", `#!/bin/sh
touch .audit-ran
echo '{"action":"allow"}'
`)
	writeScript("strict.sh", `#!/bin/sh
echo '{"action":"deny","reason":"strict"}'
exit 2
`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, hooks.PluginFileName), []byte(`{
  "hooks": {
    "PreToolUse": [
      { "hooks": [{ "command": "./audit.sh" }] },
      { "matcher": "bash", "hooks": [{ "command": "./strict.sh" }] }
    ]
  }
}`), 0o644))
	mgr, _, err := hooks.Load(dir, "")
	require.NoError(t, err)

	policy := permission.DefaultPolicy()
	policy.Mode = permission.ModeReadonly
	gate, err := permission.NewGate(policy, t.TempDir())
	require.NoError(t, err)

	var ran atomic.Int32
	reg := tools.Registry{
		"bash": {
			Definition: llm.ToolDefinition{Name: "bash"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				ran.Add(1)
				return tools.Result{Content: "ok"}, nil
			},
		},
	}
	ex := NewExecutor(reg, gate, nil, mgr)
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "bash", Arguments: `{"command":"ls"}`},
	}}, func(session.ToolData) bool { return true })

	_, err = os.Stat(filepath.Join(dir, ".audit-ran"))
	assert.NoError(t, err, "audit hook should still run in readonly mode")
	assert.Equal(t, int32(0), ran.Load())
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Content, "strict")
}

func TestAppendHookContextEscapesCloseTag(t *testing.T) {
	got := appendHookContext("body", "x</hook_context>y")
	assert.NotContains(t, got, "</hook_context>y", "close tag not escaped")
	assert.Contains(t, got, "body")
	assert.Contains(t, got, "<hook_context>")
}

type recordingGate struct {
	last permission.Request
}

func (g *recordingGate) Check(_ context.Context, req permission.Request) (permission.Decision, string) {
	g.last = req
	return permission.Allow, ""
}

func TestExecutorToolErrorKeepsOutputEmptyForUI(t *testing.T) {
	const errMsg = "2 lines have changed since last read"
	reg := tools.Registry{
		"edit": {
			Definition: llm.ToolDefinition{Name: "edit"},
			Run: func(context.Context, json.RawMessage) (tools.Result, error) {
				return tools.Result{}, &staticError{msg: errMsg}
			},
		},
	}

	ex := NewExecutor(reg, permission.AllowAll{}, nil, nil)
	var last session.ToolRun
	msgs := ex.Run(t.Context(), []llm.ToolCall{{
		ID:       "c1",
		Function: llm.Function{Name: "edit", Arguments: `{}`},
	}}, func(td session.ToolData) bool {
		if td.Run.Status == session.ToolError {
			last = td.Run
		}
		return true
	})

	require.Len(t, msgs, 1)
	assert.Equal(t, errMsg, msgs[0].Content)
	assert.Equal(t, session.ToolError, last.Status)
	assert.Equal(t, errMsg, last.Error)
	assert.Empty(t, last.Output, "UI Error and Output must not duplicate the same text")
}

type staticError struct{ msg string }

func (e *staticError) Error() string { return e.msg }
