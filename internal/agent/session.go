package agent

import (
	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/session"
)

// Session wraps session.Manager with agent-level conveniences.
// It owns the message store for the engine loop.
type Session struct {
	manager *session.Manager
}

// NewSession creates an empty session.
func NewSession() *Session {
	return &Session{
		manager: session.NewManager(),
	}
}

// Append adds one or more messages to the session.
func (s *Session) Append(msg ...llm.Message) {
	for _, m := range msg {
		_, _ = s.manager.Append(m)
	}
}

// BuildContext returns all messages for LLM inference.
// Compaction entries are projected as user messages carrying the summary.
func (s *Session) BuildContext() []llm.Message {
	entries := s.manager.BuildContext()
	msgs := make([]llm.Message, 0, len(entries))
	for _, entry := range entries {
		switch entry.GetType() {
		case session.EntryCompaction:
			m := entry.(session.CompactionEntry)
			msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: m.Compaction.Summary})
		case session.EntryMessage:
			m := entry.(session.SessionMessageEntry)
			msgs = append(msgs, m.Message)
		}
	}
	return msgs
}

// Len returns the number of stored messages.
func (s *Session) Len() int {
	return s.manager.Len()
}
