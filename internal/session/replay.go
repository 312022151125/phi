package session

import (
	"strings"

	"github.com/pulseaiclub/phi/internal/llm"
)

// ToolDetail resolves a friendly one-line display detail for a tool call's raw
// JSON arguments, mirroring the live executor's DetailFromArgs. It returns ""
// when no friendly detail is available, in which case replay falls back to the
// raw arguments. nil means always use the raw arguments.
type ToolDetail func(toolName, args string) string

// ReplaySnapshot rebuilds a UI transcript snapshot from context path entries
// (user/assistant text, thinking, tool runs, compaction markers) so /resume
// and /clear render the same styled rows as the live session without
// re-streaming. detail resolves tool-call arguments into the same friendly
// one-line detail the live turn shows (e.g. "read foo.go:10-20" instead of raw
// JSON); pass nil to keep raw arguments.
func ReplaySnapshot(entries []MessageEntry, detail ToolDetail) Snapshot {
	var snap Snapshot
	for _, entry := range entries {
		switch entry.GetType() {
		case EntryCompaction:
			snap = Apply(snap, CompactionComplete{ID: entry.GetID()})
		case EntryMessage:
			snap = replayEntry(snap, entry.(SessionMessageEntry), detail)
		}
	}
	return snap
}

func replayEntry(snap Snapshot, entry SessionMessageEntry, detail ToolDetail) Snapshot {
	msg := entry.Message
	switch msg.Role {
	case llm.RoleUser:
		images := make([]llm.Image, 0, len(msg.Images))
		return Apply(snap, UserAppend{
			ID:     entry.GetID(),
			Text:   msg.Content,
			Images: append(images, msg.Images...),
		})
	case llm.RoleAssistant:
		return Apply(snap, AssistantMessageUpdate{Message: replayAssistant(entry.GetID(), msg, detail)})
	case llm.RoleTool:
		// The persisted result is the model-facing content; Name/Detail of the
		// run are carried over from the tool_use block by Apply's merge.
		return Apply(snap, ToolData{Run: ToolRun{
			ToolUseID: msg.ToolCallID,
			Status:    ToolDone,
			Output:    msg.Content,
		}})
	}
	return snap
}

// replayAssistant converts a persisted assistant llm.Message into a session
// Message. Tool calls become tool_use content blocks so the snapshot carries
// the same tool runs (and thus styled tool rows) as the original turn.
func replayAssistant(id string, msg llm.Message, detail ToolDetail) Message {
	text := msg.Content
	var blocks []ContentBlock
	if strings.TrimSpace(msg.ReasoningContent) != "" {
		blocks = append(blocks, ContentBlock{Type: BlockThinking, Text: msg.ReasoningContent})
	}
	if text != "" {
		blocks = append(blocks, ContentBlock{Type: BlockText, Text: text})
	}
	for _, call := range msg.ToolCalls {
		blocks = append(blocks, ContentBlock{
			Type:     BlockToolUse,
			ID:       call.ID,
			Name:     call.Function.Name,
			Input:    replayToolInput(call, detail),
			Complete: true,
		})
	}
	return Message{
		ID:         id,
		State:      StateComplete,
		StopReason: replayStopReason(blocks),
		Text:       text,
		Content:    blocks,
	}
}

// replayToolInput prefers the friendly detail (matching the live turn) and
// falls back to the raw JSON arguments.
func replayToolInput(call llm.ToolCall, detail ToolDetail) string {
	if detail != nil {
		if d := detail(call.Function.Name, call.Function.Arguments); d != "" {
			return d
		}
	}
	return call.Function.Arguments
}

func replayStopReason(blocks []ContentBlock) StopReason {
	for _, b := range blocks {
		if b.Type == BlockToolUse {
			return StopToolUse
		}
	}
	return StopNone
}
