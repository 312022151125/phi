package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/llm/skills"
	"github.com/pulseaiclub/phi/internal/session"
	"github.com/pulseaiclub/phi/internal/tools"
)

// Engine drives the agent loop: stream → tools → stream…
// and yields session.Event for the TUI reducer.
type Engine struct {
	client    *llm.Client
	executor  *Executor
	maxRounds int
	skillPath string

	session *Session
}

// NewEngine wires an LLM client and tool executor.
func NewEngine(cfg llm.ModelConfig) *Engine {
	toolList := tools.DefaultTools()
	client := llm.NewClient(cfg, tools.Definitions(toolList), Prompt(cfg.SkillPath))
	return &Engine{
		client:    client,
		executor:  NewExecutor(tools.NewRegistry(toolList)),
		maxRounds: defaultMaxToolRounds,
		skillPath: cfg.SkillPath,
		session:   NewSession(),
	}
}

// LoopOpts configures a single agent loop turn.
type LoopOpts struct {
	// PendingSkills are skill names the user selected in the composer.
	// When set, the model is instructed to read those SKILL.md files first.
	PendingSkills []string
}

// Loop appends the user prompt and runs inference + tool rounds until the
// model stops calling tools or the context is cancelled.
func (engine *Engine) Loop(ctx context.Context, prompt string, opts LoopOpts) iter.Seq2[session.Event, error] {
	return func(yield func(session.Event, error) bool) {
		content := prompt
		if instr := pendingSkillsInstruction(engine.skillPath, opts.PendingSkills); instr != "" {
			if content == "" {
				content = instr
			} else {
				content = instr + "\n\n" + content
			}
		}
		engine.session.Append(llm.Message{
			Role:    llm.RoleUser,
			Content: content,
		})

		for round := 0; ; round++ {
			if round > engine.maxRounds {
				yield(nil, fmt.Errorf("agent: exceeded maximum tool rounds (%d)", engine.maxRounds))
				return
			}
			if ctx.Err() != nil {
				return
			}

			msgs := engine.session.BuildContext()

			msg, ok := engine.streamTurn(ctx, yield, msgs)
			if !ok {
				return
			}

			if len(msg.ToolCalls) == 0 {
				engine.session.Append(msg)
				return
			}

			engine.session.Append(msg)

			toolMsgs := engine.executor.Run(ctx, msg.ToolCalls, func(td session.ToolData) bool {
				return yield(td, nil)
			})

			engine.session.Append(toolMsgs...)

			if ctx.Err() != nil {
				return
			}
		}
	}
}

func (engine *Engine) streamTurn(
	ctx context.Context,
	yield func(session.Event, error) bool,
	messages []llm.Message,
) (llm.Message, bool) {
	id := fmt.Sprintf("assistant-%d", time.Now().UnixNano())
	var thinking, text string
	var final llm.Message
	gotDone := false

	for event, err := range engine.client.Stream(ctx, messages) {
		if err != nil {
			if thinking != "" || text != "" {
				_ = yield(emitMessage(id, session.StateError, session.StopNone, thinking, text, nil), nil)
			}
			yield(nil, err)
			return llm.Message{}, false
		}

		switch event.Type {
		case llm.StreamEventTypeError:
			errText := event.Err
			if errText == "" {
				errText = "stream error"
			}
			yield(nil, fmt.Errorf("%s", errText))
			return llm.Message{}, false

		case llm.StreamEventTypeDelta:
			if event.Delta.ReasoningContent != "" {
				thinking += event.Delta.ReasoningContent
			}
			if event.Delta.Content != "" {
				text += event.Delta.Content
			}
			if !yield(emitMessage(id, session.StateStreaming, session.StopNone, thinking, text, nil), nil) {
				return llm.Message{}, false
			}

		case llm.StreamEventTypeDone:
			if len(event.Partial.Choices) == 0 {
				yield(nil, errors.New("agent: stream finished with no assistant choice"))
				return llm.Message{}, false
			}
			final = event.Partial.Choices[0].Message
			gotDone = true
			// Prefer fully accumulated message for the complete event.
			if final.ReasoningContent != "" {
				thinking = final.ReasoningContent
			}
			if final.Content != "" {
				text = final.Content
			}
		}
	}

	if !gotDone {
		if ctx.Err() != nil {
			_ = yield(emitMessage(id, session.StateCancelled, session.StopNone, thinking, text, nil), nil)
			return llm.Message{}, false
		}
		yield(nil, fmt.Errorf("agent: stream closed without assistant output"))
		return llm.Message{}, false
	}

	blocks := engine.toolCallsToBlocks(final.ToolCalls)
	reason := session.StopEndTurn
	if len(blocks) > 0 {
		reason = session.StopToolUse
	}
	if !yield(emitMessage(id, session.StateComplete, reason, thinking, text, blocks), nil) {
		return llm.Message{}, false
	}
	return final, true
}

func (engine *Engine) toolCallsToBlocks(calls []llm.ToolCall) []session.ContentBlock {
	if len(calls) == 0 {
		return nil
	}
	out := make([]session.ContentBlock, 0, len(calls))
	for _, c := range calls {
		input := c.Function.Arguments
		if tool, ok := engine.executor.registry[c.Function.Name]; ok && tool.DetailFromArgs != nil {
			if d := tool.DetailFromArgs(json.RawMessage(c.Function.Arguments)); d != "" {
				input = d
			}
		}
		out = append(out, session.ContentBlock{
			Type:     session.BlockToolUse,
			ID:       c.ID,
			Name:     c.Function.Name,
			Input:    input,
			Complete: true,
		})
	}
	return out
}

func buildContent(thinking, text string, tools []session.ContentBlock) []session.ContentBlock {
	var out []session.ContentBlock
	if thinking != "" {
		out = append(out, session.ContentBlock{Type: session.BlockThinking, Text: thinking})
	}
	if text != "" {
		out = append(out, session.ContentBlock{Type: session.BlockText, Text: text})
	}
	out = append(out, tools...)
	return out
}

func emitMessage(
	id string,
	state session.State,
	reason session.StopReason,
	thinking,
	text string,
	tools []session.ContentBlock,
) session.Event {
	return session.AssistantMessageUpdate{Message: session.Message{
		ID:         id,
		State:      state,
		StopReason: reason,
		Content:    buildContent(thinking, text, tools),
		Text:       text,
	}}
}

// pendingSkillsInstruction tells the model to read SKILL.md files for the
// selected skills (panda-style: reuse the read tool, no dedicated skill tool).
func pendingSkillsInstruction(skillPath string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	list, err := skills.LoadSkills(skillPath)
	targets := make([]string, 0, len(names))
	if err == nil {
		for _, name := range names {
			if s := skills.Find(list, name); s != nil && s.SkillFilePath != "" {
				targets = append(targets, s.SkillFilePath)
				continue
			}
			targets = append(targets, name)
		}
	} else {
		targets = append(targets, names...)
	}
	return fmt.Sprintf(
		"You MUST read these skill files first with the read tool and follow them: %s. Do this immediately before responding.",
		strings.Join(targets, ", "),
	)
}
