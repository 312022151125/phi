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
		s.manager.Append(m)
	}
}

// BuildContext returns all messages for LLM inference.
func (s *Session) BuildContext() []llm.Message {
	return s.manager.BuildContext()
}

// Len returns the number of stored messages.
func (s *Session) Len() int {
	return s.manager.Len()
}