package session

import (
	"os"
	"testing"

	"github.com/pulseaiclub/phi/internal/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionManager(t *testing.T) {
	t.Run("no flush", func(t *testing.T) {
		dir := t.TempDir()
		manager, err := NewSessionManager(
			dir,
			WithSessionDir(dir),
			WithShouldFlush(true),
		)
		require.NoError(t, err)
		assert.Equal(t, dir, manager.cwd)
		assert.Equal(t, dir, manager.config.sessionDir)
		assert.True(t, manager.config.shouldFlush)
		assert.False(t, manager.flushed)
		assert.Nil(t, manager.leafID)
		assert.NotEmpty(t, manager.sessionID)
		assert.NotEmpty(t, manager.sessionFile)
	})
}

func TestGetBranch(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewSessionManager(
		dir,
		WithSessionDir(dir),
		WithShouldFlush(false),
	)
	require.NoError(t, err)

	id1, err := manager.Append(llm.Message{
		Role:    llm.RoleUser,
		Content: "Hello",
	})
	require.NoError(t, err)

	id2, err := manager.Append(llm.Message{
		Role:    llm.RoleAssistant,
		Content: "Hi there!",
	})
	require.NoError(t, err)

	id3, err := manager.Append(llm.Message{
		Role:    llm.RoleUser,
		Content: "How are you?",
	})
	require.NoError(t, err)

	branch := manager.GetBranch(id3)
	require.Len(t, branch, 3)

	assert.Equal(t, id1, branch[2].GetID())
	assert.Equal(t, id2, branch[1].GetID())
	assert.Equal(t, id3, branch[0].GetID())
}

func TestAppendEntry(t *testing.T) {
	t.Run("flush to disk", func(t *testing.T) {
		dir := t.TempDir()
		manager, err := NewSessionManager(
			dir,
			WithSessionDir(dir),
			WithShouldFlush(true),
		)
		require.NoError(t, err)

		_, err = manager.Append(llm.Message{
			Role:    llm.RoleUser,
			Content: "Hello",
		})
		require.NoError(t, err)

		_, err = manager.Append(llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Hi there!",
		})
		require.NoError(t, err)

		assert.True(t, manager.flushed)

		_, err = os.Stat(manager.sessionFile)
		require.NoError(t, err, "session file should be created")
	})

	t.Run("no flush to disk", func(t *testing.T) {
		dir := t.TempDir()
		manager, err := NewSessionManager(
			dir,
			WithSessionDir(dir),
			WithShouldFlush(false),
		)
		require.NoError(t, err)

		id, err := manager.Append(llm.Message{
			Role:    llm.RoleUser,
			Content: "test",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, id)
		assert.False(t, manager.flushed)
	})
}

func TestBuildSessionContext(t *testing.T) {
	t.Run("no compaction, returns all messages in order", func(t *testing.T) {
		entry1 := SessionMessageEntry{
			SessionBaseEntry: SessionBaseEntry{
				Type: EntryMessage,
				ID:   "msg1",
			},
			Message: llm.Message{
				Role:    llm.RoleUser,
				Content: "hello",
			},
		}
		entry2 := SessionMessageEntry{
			SessionBaseEntry: SessionBaseEntry{
				Type:     EntryMessage,
				ID:       "msg2",
				ParentID: &entry1.ID,
			},
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: "hi",
			},
		}
		entry3 := SessionMessageEntry{
			SessionBaseEntry: SessionBaseEntry{
				Type:     EntryMessage,
				ID:       "msg3",
				ParentID: &entry2.ID,
			},
			Message: llm.Message{
				Role:    llm.RoleUser,
				Content: "how are you?",
			},
		}

		entries := []MessageEntry{entry1, entry2, entry3}
		byID := map[string]MessageEntry{
			entry1.ID: entry1,
			entry2.ID: entry2,
			entry3.ID: entry3,
		}

		ctx := buildSessionContext(entries, entry3.ID, byID)
		require.Len(t, ctx, 3)

		assert.Equal(t, "msg1", ctx[0].GetID())
		assert.Equal(t, "msg2", ctx[1].GetID())
		assert.Equal(t, "msg3", ctx[2].GetID())
	})

	t.Run("with compaction, includes compaction and subsequent messages", func(t *testing.T) {
		entry1 := SessionMessageEntry{
			SessionBaseEntry: SessionBaseEntry{
				Type: EntryMessage,
				ID:   "msg1",
			},
			Message: llm.Message{
				Role:    llm.RoleUser,
				Content: "hello",
			},
		}
		entry2 := SessionMessageEntry{
			SessionBaseEntry: SessionBaseEntry{
				Type:     EntryMessage,
				ID:       "msg2",
				ParentID: &entry1.ID,
			},
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: "hi",
			},
		}
		compaction := CompactionEntry{
			SessionBaseEntry: SessionBaseEntry{
				Type:     EntryCompaction,
				ID:       "cmp1",
				ParentID: &entry2.ID,
			},
			Compaction: Compaction{
				Summary:          "summary",
				FirstKeptEntryID: "msg2",
			},
		}
		entry3 := SessionMessageEntry{
			SessionBaseEntry: SessionBaseEntry{
				Type:     EntryMessage,
				ID:       "msg3",
				ParentID: &compaction.ID,
			},
			Message: llm.Message{
				Role:    llm.RoleUser,
				Content: "after compaction",
			},
		}

		entries := []MessageEntry{entry1, entry2, compaction, entry3}
		byID := map[string]MessageEntry{
			entry1.ID:     entry1,
			entry2.ID:     entry2,
			compaction.ID: compaction,
			entry3.ID:     entry3,
		}

		ctx := buildSessionContext(entries, entry3.ID, byID)

		require.Len(t, ctx, 3)
		assert.Equal(t, EntryCompaction, ctx[0].GetType())
		assert.Equal(t, "cmp1", ctx[0].GetID())
		assert.Equal(t, "msg2", ctx[1].GetID())
		assert.Equal(t, "msg3", ctx[2].GetID())
	})
}

func TestReplaySnapshotEmpty(t *testing.T) {
	m := NewManager(t.TempDir())

	snap := ReplaySnapshot(m.BuildContext(), nil)

	assert.Empty(t, snap.Messages)
	assert.Empty(t, snap.Tools)
	assert.False(t, snap.Compacting)
}

func TestReplaySnapshotMessages(t *testing.T) {
	m := NewManager(t.TempDir())

	userID, err := m.Append(llm.Message{
		Role:    llm.RoleUser,
		Content: "hello",
		Images:  []llm.Image{{Data: "aW1n", MimeType: "image/png"}},
	})
	require.NoError(t, err)

	asstID, err := m.Append(llm.Message{
		Role:             llm.RoleAssistant,
		Content:          "hi there",
		ReasoningContent: "thought about it",
	})
	require.NoError(t, err)

	snap := ReplaySnapshot(m.BuildContext(), nil)
	require.Len(t, snap.Messages, 2)

	user := snap.Messages[0]
	assert.Equal(t, userID, user.ID)
	assert.Equal(t, RoleUser, user.Role)
	assert.Equal(t, "hello", user.Text)
	assert.Equal(t, []llm.Image{{Data: "aW1n", MimeType: "image/png"}}, user.Images)

	asst := snap.Messages[1]
	assert.Equal(t, asstID, asst.ID)
	assert.Equal(t, RoleAssistant, asst.Role)
	assert.Equal(t, StateComplete, asst.State)
	assert.Equal(t, "hi there", asst.Text)
	assert.Equal(t, []ContentBlock{
		{Type: BlockThinking, Text: "thought about it"},
		{Type: BlockText, Text: "hi there"},
	}, asst.Content)
}

func TestReplaySnapshotTools(t *testing.T) {
	m := NewManager(t.TempDir())

	_, err := m.Append(llm.Message{Role: llm.RoleUser, Content: "read the file"})
	require.NoError(t, err)

	_, err = m.Append(llm.Message{
		Role:    llm.RoleAssistant,
		Content: "calling tool",
		ToolCalls: []llm.ToolCall{{
			ID:   "t1",
			Type: "function",
			Function: llm.Function{
				Name:      "Read",
				Arguments: `{"path":"a.go"}`,
			},
		}},
	})
	require.NoError(t, err)

	_, err = m.Append(llm.Message{
		Role:       llm.RoleTool,
		ToolCallID: "t1",
		Content:    "package main",
	})
	require.NoError(t, err)

	detail := func(toolName, _ string) string {
		if toolName == "Read" {
			return "read a.go"
		}
		return ""
	}
	snap := ReplaySnapshot(m.BuildContext(), detail)
	require.Len(t, snap.Messages, 2)

	asst := snap.Messages[1]
	assert.Equal(t, StateComplete, asst.State)
	assert.Equal(t, StopToolUse, asst.StopReason)
	assert.Equal(t, []ContentBlock{
		{Type: BlockText, Text: "calling tool"},
		{Type: BlockToolUse, ID: "t1", Name: "Read", Input: "read a.go", Complete: true},
	}, asst.Content)

	run, ok := snap.Tools["t1"]
	require.True(t, ok)
	assert.Equal(t, "Read", run.Name)
	assert.Equal(t, ToolDone, run.Status)
	assert.Equal(t, "read a.go", run.Detail)
	assert.Equal(t, "package main", run.Output)

	items := Project(snap)
	require.Len(t, items, 3)
	assert.Equal(t, ItemTool, items[2].Kind)
	assert.Equal(t, "Read", items[2].ToolName)
	assert.Equal(t, "package main", items[2].ToolRun.Output)
}

func TestReplaySnapshotCompactionDropsOldMessages(t *testing.T) {
	m := NewManager(t.TempDir())

	_, err := m.Append(llm.Message{Role: llm.RoleUser, Content: "old question"})
	require.NoError(t, err)
	_, err = m.Append(llm.Message{Role: llm.RoleAssistant, Content: "old answer"})
	require.NoError(t, err)
	keptID, err := m.Append(llm.Message{Role: llm.RoleUser, Content: "still here"})
	require.NoError(t, err)
	compactionID, err := m.AppendCompaction(Compaction{
		Summary:          "summary of the past",
		FirstKeptEntryID: keptID,
	})
	require.NoError(t, err)

	snap := ReplaySnapshot(m.BuildContext(), nil)
	require.Len(t, snap.Messages, 2)

	assert.Equal(t, compactionID, snap.Messages[0].ID)
	assert.Equal(t, RoleCompaction, snap.Messages[0].Role)

	assert.Equal(t, keptID, snap.Messages[1].ID)
	assert.Equal(t, RoleUser, snap.Messages[1].Role)
	assert.Equal(t, "still here", snap.Messages[1].Text)
}
