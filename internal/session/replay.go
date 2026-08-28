package session

import (
	"strings"

	"github.com/pulseaiclub/phi/internal/llm"
)

// ReplaySnapshot builds a UI transcript snapshot from context path entries
// (user/assistant text; tool rows simplified away). Used to re-render the
// transcript after resume/clear without re-streaming.
func ReplaySnapshot(entries []MessageEntry) Snapshot {
	var snap Snapshot
	for _, entry := range entries {
		switch entry.GetType() {
		case EntryCompaction:
			snap = Apply(snap, CompactionComplete{ID: entry.GetID()})
		case EntryMessage:
			msg := entry.(SessionMessageEntry).Message
			switch msg.Role {
			case llm.RoleUser:
				images := make([]llm.Image, 0, len(msg.Images))
				snap = Apply(snap, UserAppend{
					ID:     entry.GetID(),
					Text:   msg.Content,
					Images: append(images, msg.Images...),
				})
			case llm.RoleAssistant:
				text := msg.Content
				var blocks []ContentBlock
				if strings.TrimSpace(msg.ReasoningContent) != "" {
					blocks = append(
						blocks,
						ContentBlock{Type: BlockThinking, Text: msg.ReasoningContent},
					)
				}
				if text != "" {
					blocks = append(blocks, ContentBlock{Type: BlockText, Text: text})
				}
				snap = Apply(snap, AssistantMessageUpdate{Message: Message{
					ID:      entry.GetID(),
					State:   StateComplete,
					Text:    text,
					Content: blocks,
				}})
			}
		}
	}
	return snap
}
