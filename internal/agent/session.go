package agent

import (
	"strings"

	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/session"
)

// Session owns the message store for the engine loop. It wraps a
// session.Manager and projects entries (including compaction summaries)
// into LLM context. Compaction policy lives on Engine, not here.
type Session struct {
	lastID            string
	manager           *session.Manager
	contextCache      []llm.Message
	contextCacheValid bool
}

// NewSession creates an in-memory session manager wrapper.
func NewSession() *Session {
	return &Session{manager: session.NewManager()}
}

func (s *Session) invalidateContextCache() {
	s.contextCacheValid = false
}

// Append records one or more messages.
func (s *Session) Append(message ...llm.Message) error {
	s.invalidateContextCache()
	for _, msg := range message {
		id, err := s.manager.Append(msg)
		if err != nil {
			return err
		}
		s.lastID = id
	}
	return nil
}

// AppendCompaction records a compaction entry and invalidates the context cache.
func (s *Session) AppendCompaction(c session.Compaction) error {
	s.invalidateContextCache()
	id, err := s.manager.AppendCompaction(c)
	if err != nil {
		return err
	}
	s.lastID = id
	return nil
}

// PathEntries returns the current leaf-to-root session entries for compaction.
func (s *Session) PathEntries() []session.MessageEntry {
	return s.manager.BuildContext()
}

// BuildContext returns the messages for LLM inference, oldest first.
// Compaction entries are projected as user messages carrying the summary.
func (s *Session) BuildContext() []llm.Message {
	if s.contextCacheValid {
		return s.contextCache
	}
	entries := s.manager.BuildContext()
	msgs := make([]llm.Message, 0, len(entries))
	for _, entry := range entries {
		switch entry.GetType() {
		case session.EntryCompaction:
			m := entry.(session.CompactionEntry)
			msgs = append(msgs, llm.Message{
				Role:    llm.RoleUser,
				Content: m.Compaction.Summary,
			})
		case session.EntryMessage:
			m := entry.(session.SessionMessageEntry)
			msgs = append(msgs, m.Message)
		}
	}
	s.contextCache = msgs
	s.contextCacheValid = true
	return msgs
}

// Len returns the number of stored entries (including the session header).
func (s *Session) Len() int {
	return s.manager.Len()
}

// LastID returns the ID of the most recently appended entry.
func (s *Session) LastID() string {
	return s.lastID
}

// AddUser records a single user message for this chat turn.
func (s *Session) AddUser(text string) error {
	return s.Append(llm.Message{
		Role:    llm.RoleUser,
		Content: text,
	})
}

// AddAssistant records an assistant message from an LLM response (including tool_calls).
func (s *Session) AddAssistant(assistant llm.Message, usage llm.Usage) error {
	return s.Append(llm.Message{
		Role:             llm.RoleAssistant,
		Content:          assistant.Content,
		ReasoningContent: assistant.ReasoningContent,
		ToolCalls:        assistant.ToolCalls,
		Usage:            usage,
	})
}

// AddFinalAssistant records the last assistant message when it carries text or tool_calls.
func (s *Session) AddFinalAssistant(resp llm.Response) error {
	if len(resp.Choices) == 0 {
		return nil
	}
	final := resp.Choices[0].Message
	if strings.TrimSpace(final.Content) == "" && len(final.ToolCalls) == 0 {
		return nil
	}
	return s.AddAssistant(final, resp.Usage)
}
