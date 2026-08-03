package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/session"
	"github.com/pulseaiclub/phi/internal/tools"
)

const ToolCanceledResult = "User cancelled the tool call."

// Executor runs model tool_calls against a tool registry and emits ToolData for the UI.
type Executor struct {
	registry tools.Registry
}

func NewExecutor(registry tools.Registry) *Executor {
	return &Executor{registry: registry}
}

// Run executes tool calls in order, yielding ToolData updates via emit.
// Returns role=tool messages for the next LLM turn (including cancel stubs).
func (e *Executor) Run(
	ctx context.Context,
	calls []llm.ToolCall,
	emit func(session.ToolData) bool,
) []llm.Message {
	results := make([]llm.Message, 0, len(calls))
	for _, call := range calls {
		if ctx.Err() != nil {
			results = append(results, e.cancelResult(call, emit))
			continue
		}
		results = append(results, e.runOne(ctx, call, emit))
	}
	return results
}

func (e *Executor) runOne(ctx context.Context, call llm.ToolCall, emit func(session.ToolData) bool) llm.Message {
	tool, ok := e.registry[call.Function.Name]
	args := json.RawMessage(call.Function.Arguments)
	detail := call.Function.Arguments
	if ok && tool.DetailFromArgs != nil {
		if d := tool.DetailFromArgs(args); d != "" {
			detail = d
		}
	}

	if !emit(session.ToolData{Run: session.ToolRun{
		ToolUseID: call.ID,
		Status:    session.ToolInProgress,
		Detail:    detail,
	}}) {
		return e.toolMessage(call.ID, ToolCanceledResult)
	}

	if !ok {
		errText := fmt.Sprintf("tool '%s' not found", call.Function.Name)
		_ = emit(session.ToolData{Run: session.ToolRun{
			ToolUseID: call.ID,
			Status:    session.ToolError,
			Detail:    detail,
			Error:     errText,
		}})
		return e.toolMessage(call.ID, errText)
	}

	result, err := tool.Run(ctx, args)
	if err != nil {
		if ctx.Err() != nil {
			return e.cancelResult(call, emit)
		}
		errText := err.Error()
		_ = emit(session.ToolData{Run: session.ToolRun{
			ToolUseID: call.ID,
			Status:    session.ToolError,
			Detail:    detail,
			Error:     errText,
			Output:    errText,
		}})
		return e.toolMessage(call.ID, errText)
	}

	out := result.Output
	if out == "" {
		out = result.Content
	}
	if result.Detail != "" {
		detail = result.Detail
	}
	_ = emit(session.ToolData{Run: session.ToolRun{
		ToolUseID: call.ID,
		Status:    session.ToolDone,
		Detail:    detail,
		Output:    out,
	}})
	return e.toolMessage(call.ID, result.Content)
}

func (e *Executor) cancelResult(call llm.ToolCall, emit func(session.ToolData) bool) llm.Message {
	detail := call.Function.Arguments
	if tool, ok := e.registry[call.Function.Name]; ok && tool.DetailFromArgs != nil {
		if d := tool.DetailFromArgs(json.RawMessage(call.Function.Arguments)); d != "" {
			detail = d
		}
	}
	_ = emit(session.ToolData{Run: session.ToolRun{
		ToolUseID: call.ID,
		Status:    session.ToolCancelled,
		Detail:    detail,
		Output:    ToolCanceledResult,
	}})
	return e.toolMessage(call.ID, ToolCanceledResult)
}

func (e *Executor) toolMessage(id, content string) llm.Message {
	return llm.Message{
		Role:       llm.RoleTool,
		ToolCallID: id,
		Content:    content,
	}
}
