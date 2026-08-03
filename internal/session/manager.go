package session

import (
	"sync"

	"github.com/pulseaiclub/phi/internal/llm"
)

// Manager is the single source of truth for session messages.
// Currently a flat list; will grow to support persistence, compaction,
// branching, and cursor-based context building.
type Manager struct {
	mu      sync.Mutex
	entries []llm.Message
}

// NewManager creates an empty session manager.
func NewManager() *Manager {
	return &Manager{
		entries: make([]llm.Message, 0, 64),
	}
}

func (m *Manager) Append(msg llm.Message) {
	m.mu.Lock()
	m.entries = append(m.entries, msg)
	m.mu.Unlock()
}

func (m *Manager) BuildContext() []llm.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]llm.Message, len(m.entries))
	copy(out, m.entries)
	return out
}

func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}