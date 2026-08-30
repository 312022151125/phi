package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pulseaiclub/phi/internal/extension"
	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/permission"
	"github.com/pulseaiclub/phi/internal/session"
	"github.com/pulseaiclub/phi/internal/tools"
	"github.com/pulseaiclub/phi/internal/util"
)

// ToolCanceledResult is returned to the model when a user cancels a tool call.
const ToolCanceledResult = "User cancelled the tool call."

const (
	extContextOpen  = "<ext_context>"
	extContextClose = "</ext_context>"
)

// Executor runs model tool_calls against a tool registry and emits ToolData for the UI.
type Executor struct {
	registry  tools.Registry
	gate      permission.Gate
	ask       permission.AskFunc
	ext       *extension.Runner // nil = no extensions
	sessionID string
	cwd       string
}

// NewExecutor builds an executor. extRunner may be nil.
func NewExecutor(
	registry tools.Registry,
	gate permission.Gate,
	ask permission.AskFunc,
	extRunner *extension.Runner,
) *Executor {
	if gate == nil {
		gate = permission.AllowAll{}
	}
	return &Executor{registry: registry, gate: gate, ask: ask, ext: extRunner}
}

// SetMeta attaches session identity used in extension Event payloads.
func (e *Executor) SetMeta(sessionID, cwd string) {
	if e == nil {
		return
	}
	e.sessionID = sessionID
	e.cwd = cwd
	if e.ext != nil {
		e.ext.SetMeta(sessionID, cwd)
	}
}

func (e *Executor) activeExt() *extension.Runner {
	if e == nil {
		return nil
	}
	return e.ext
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
	ctx = tools.WithCwd(ctx, e.cwd)
	tool, ok := e.registry[call.Function.Name]
	args := json.RawMessage(call.Function.Arguments)
	detail := call.Function.Arguments
	if ok && tool.DetailFromArgs != nil {
		if d := tool.DetailFromArgs(args); d != "" {
			detail = d
		}
	}

	if !emit(session.ToolData{Run: e.toolRun(call, session.ToolInProgress, detail, "", "")}) {
		return e.toolMessage(call.ID, ToolCanceledResult)
	}

	if !ok {
		errText := fmt.Sprintf("tool '%s' not found", call.Function.Name)
		_ = emit(session.ToolData{Run: e.toolRun(call, session.ToolError, detail, errText, "")})
		return e.toolMessage(call.ID, errText)
	}

	extRunner := e.activeExt()
	if extRunner != nil {
		extRunner.EmitToolExecutionStart(call.Function.Name, call.ID, args)
	}

	// ExtensionPre → Gate → Run → ExtensionPost. Pre runs before permission Ask
	// so org policy can deny without prompting the user.
	var preContext string
	if extRunner != nil {
		newArgs, blocked, reason, ctxText := extRunner.PreTool(ctx, call.Function.Name, call.ID, args)
		preContext = ctxText
		if blocked {
			if reason == "" {
				reason = "tool execution denied by extension"
			}
			reason = appendExtContext(reason, preContext)
			if extRunner != nil {
				extRunner.EmitToolExecutionEnd(call.Function.Name, call.ID, true)
			}
			return e.rejectResult(call, detail, reason, emit)
		}
		if len(newArgs) > 0 {
			args = newArgs
			if tool.DetailFromArgs != nil {
				if d := tool.DetailFromArgs(args); d != "" {
					detail = d
				}
			} else {
				detail = string(args)
			}
		}
	}

	if msg, rejected := e.checkPermission(ctx, call, args, detail, emit); rejected {
		if extRunner != nil {
			extRunner.EmitToolExecutionEnd(call.Function.Name, call.ID, true)
		}
		return msg
	}

	result, err := tool.Run(tools.WithToolCallID(ctx, call.ID), args)

	var (
		errText string
		content string
		output  string
	)
	if err != nil {
		if ctx.Err() != nil {
			if extRunner != nil {
				extRunner.EmitToolExecutionEnd(call.Function.Name, call.ID, true)
			}
			return e.cancelResult(call, emit)
		}
		errText = err.Error()
		content = errText
	} else {
		content = result.Content
		output = result.Output
		if output == "" {
			output = result.Content
		}
		if result.Detail != "" {
			detail = result.Detail
		}
	}

	var postContext string
	if extRunner != nil {
		newContent, ctxText, _, _ := extRunner.PostTool(
			ctx,
			call.Function.Name,
			call.ID,
			args,
			content,
			err != nil,
			errText,
		)
		postContext = ctxText
		if newContent != "" {
			content = newContent
			output = newContent
		}
		extRunner.EmitToolExecutionEnd(call.Function.Name, call.ID, err != nil)
	}

	modelContent := appendExtContext(content, joinExtContexts(preContext, postContext))

	if err != nil {
		_ = emit(session.ToolData{Run: e.toolRun(call, session.ToolError, detail, errText, output)})
		return e.toolMessage(call.ID, modelContent)
	}
	_ = emit(session.ToolData{Run: e.toolRun(call, session.ToolDone, detail, "", output)})
	return e.toolMessage(call.ID, modelContent)
}

func (e *Executor) checkPermission(
	ctx context.Context,
	call llm.ToolCall,
	args json.RawMessage,
	detail string,
	emit func(session.ToolData) bool,
) (llm.Message, bool) {
	req, err := permission.ExtractAt(call.Function.Name, args, e.cwd)
	if err != nil {
		reason := fmt.Sprintf("permission check failed: %v", err)
		return e.rejectResult(call, detail, reason, emit), true
	}

	dec, reason := e.gate.Check(ctx, req)
	switch dec {
	case permission.Allow:
		return llm.Message{}, false
	case permission.Deny:
		if reason == "" {
			reason = "tool execution denied by permissions"
		}
		return e.rejectResult(call, detail, reason, emit), true
	case permission.Ask:
		if e.ask == nil {
			if reason == "" {
				reason = "tool requires approval but no ask handler is configured"
			}
			return e.rejectResult(call, detail, reason, emit), true
		}
		res, askErr := e.ask(ctx, req, reason)
		if askErr != nil {
			msg := fmt.Sprintf("approval failed: %v", askErr)
			return e.rejectResult(call, detail, msg, emit), true
		}
		if !res.Approved {
			msg := "tool execution rejected by user"
			if res.Feedback != "" {
				msg = "This tool call was rejected by the user with feedback: " + res.Feedback
			}
			return e.rejectResult(call, detail, msg, emit), true
		}
		return llm.Message{}, false
	default:
		return e.rejectResult(call, detail, "unknown permission decision", emit), true
	}
}

func (e *Executor) rejectResult(
	call llm.ToolCall,
	detail, reason string,
	emit func(session.ToolData) bool,
) llm.Message {
	_ = emit(session.ToolData{Run: e.toolRun(call, session.ToolRejected, detail, reason, "")})
	return e.toolMessage(call.ID, reason)
}

func (e *Executor) cancelResult(call llm.ToolCall, emit func(session.ToolData) bool) llm.Message {
	detail := call.Function.Arguments
	if tool, ok := e.registry[call.Function.Name]; ok && tool.DetailFromArgs != nil {
		if d := tool.DetailFromArgs(json.RawMessage(call.Function.Arguments)); d != "" {
			detail = d
		}
	}
	_ = emit(session.ToolData{Run: e.toolRun(call, session.ToolCancelled, detail, "", ToolCanceledResult)})
	return e.toolMessage(call.ID, ToolCanceledResult)
}

// toolRun builds a ToolData payload with Name always set so headless JSONL
// and stderr logs never omit toolName.
func (*Executor) toolRun(
	call llm.ToolCall,
	status session.ToolStatus,
	detail, errText, output string,
) session.ToolRun {
	return session.ToolRun{
		ToolUseID: call.ID,
		Name:      call.Function.Name,
		Status:    status,
		Detail:    detail,
		Error:     errText,
		Output:    output,
	}
}

func (*Executor) toolMessage(id, content string) llm.Message {
	return llm.Message{
		Role:       llm.RoleTool,
		ToolCallID: id,
		Content:    content,
	}
}

func joinExtContexts(parts ...string) string {
	var nonempty []string
	for _, p := range parts {
		if p != "" {
			nonempty = append(nonempty, p)
		}
	}
	return strings.Join(nonempty, "\n\n")
}

// appendExtContext adds model-facing extension notes. TUI Detail/Output stay clean.
func appendExtContext(content, ctx string) string {
	if ctx == "" {
		return content
	}
	escaped := util.ReplaceAll(ctx, extContextClose, "</ext_context\u200b>")
	block := extContextOpen + "\n" + escaped + "\n" + extContextClose
	if content == "" {
		return block
	}
	return content + "\n\n" + block
}
