package agent

import (
	"context"
	"strings"

	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/session"
	"github.com/pulseaiclub/phi/internal/session/compaction"
)

// Session owns the message store for the engine loop. It wraps a
// session.Manager and triggers compaction when context usage approaches
// the model's context window.
type Session struct {
	lastID            string
	manager           *session.Manager
	contextWindow     int
	compactHandler    llm.Compactor
	contextCache      []llm.Message
	contextCacheValid bool
}

// NewSession creates an in-memory session. Pass a nil compactHandler or a
// zero contextWindow to disable compaction (safe default).
func NewSession(compactHandler llm.Compactor, contextWindow int) *Session {
	return &Session{
		manager:        session.NewManager(),
		contextWindow:  contextWindow,
		compactHandler: compactHandler,
	}
}

func (s *Session) invalidateContextCache() {
	s.contextCacheValid = false
}

// Append records one or more messages. Assistant messages may trigger
// compaction based on their token usage.
func (s *Session) Append(ctx context.Context, message ...llm.Message) error {
	s.invalidateContextCache()
	for _, msg := range message {
		if msg.Role == llm.RoleAssistant && s.compactHandler != nil {
			if err := s.Compact(ctx, msg.Usage.TotalTokens, s.compactHandler); err != nil {
				return err
			}
		}

		id, err := s.manager.Append(msg)
		if err != nil {
			return err
		}
		s.lastID = id
	}
	return nil
}

// BuildContext returns the messages for LLM inference, oldest first.
// Compaction entries are projected as user messages carrying the summary.
func (s *Session) BuildContext() []llm.Message {
	if s.contextCacheValid {
		return s.contextCache
	}
	message := s.manager.BuildContext()
	msgs := make([]llm.Message, 0, len(message))
	for _, msg := range message {
		switch msg.GetType() {
		case session.EntryCompaction:
			m := msg.(session.CompactionEntry)
			msgs = append(msgs, compactionToMessage(m))
		case session.EntryMessage:
			m := msg.(session.SessionMessageEntry)
			msgs = append(msgs, m.Message)
		}
	}
	s.contextCache = msgs
	s.contextCacheValid = true
	return msgs
}

func compactionToMessage(msg session.CompactionEntry) llm.Message {
	return llm.Message{
		Role:    llm.RoleUser,
		Content: msg.Compaction.Summary,
	}
}

// Compact runs compaction when context usage exceeds the threshold: gets path
// entries, prepares compaction, generates a summary via the LLM, and appends
// the compaction entry to the session.
func (s *Session) Compact(ctx context.Context, usage int, llm llm.Compactor) error {
	settings := compaction.DefaultSettings()
	if !compaction.ShouldCompact(usage, s.contextWindow, settings) {
		return nil
	}
	s.invalidateContextCache()
	pathEntries := s.manager.BuildContext()
	err := compaction.Run(ctx, pathEntries, s.manager, llm, settings)
	if err == nil {
		s.invalidateContextCache()
	}
	return err
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
func (s *Session) AddUser(ctx context.Context, text string) error {
	return s.Append(ctx, llm.Message{
		Role:    llm.RoleUser,
		Content: text,
	})
}

// AddAssistant records an assistant message from an LLM response (including tool_calls).
func (s *Session) AddAssistant(ctx context.Context, assistant llm.Message, usage llm.Usage) error {
	return s.Append(ctx, llm.Message{
		Role:             llm.RoleAssistant,
		Content:          assistant.Content,
		ReasoningContent: assistant.ReasoningContent,
		ToolCalls:        assistant.ToolCalls,
		Usage:            usage,
	})
}

// AddFinalAssistant records the last assistant message when it carries text or tool_calls.
func (s *Session) AddFinalAssistant(ctx context.Context, resp llm.Response) error {
	if len(resp.Choices) == 0 {
		return nil
	}
	final := resp.Choices[0].Message
	if strings.TrimSpace(final.Content) == "" && len(final.ToolCalls) == 0 {
		return nil
	}
	return s.AddAssistant(ctx, final, resp.Usage)
}
