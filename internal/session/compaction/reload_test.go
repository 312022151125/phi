package compaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/session"
)

// seedSession writes n user/assistant turns, each assistant reporting tokens,
// and returns the persisted session file.
func seedSession(t *testing.T, turns int) (path string, live []session.MessageEntry) {
	t.Helper()
	dir := t.TempDir()
	m, err := session.NewSessionManager(dir, session.WithSessionDir(dir), session.WithShouldFlush(true))
	require.NoError(t, err)

	for i := range turns {
		_, err = m.Append(llm.Message{Role: llm.RoleUser, Content: "u"})
		require.NoError(t, err)
		_, err = m.Append(llm.Message{
			Role:    llm.RoleAssistant,
			Content: "a",
			Usage:   llm.Usage{TotalTokens: 10 * (i + 1)},
		})
		require.NoError(t, err)
	}
	return m.File(), m.BuildContext()
}

// Compaction reads usage from the entry wrapper, which is the only copy that
// survives persistence. Reading llm.Message.Usage instead made every resumed
// session look like it had spent zero tokens: the cut collapsed to the earliest
// cut point, so the first auto-compaction after a resume summarized nothing.
func TestPrepareCompact_ReloadedSessionMatchesInMemory(t *testing.T) {
	path, liveEntries := seedSession(t, 4)
	settings := Settings{keepRecentTokens: 25}

	inMemory, err := PrepareCompact(liveEntries, settings)
	require.NoError(t, err)

	reloaded, err := session.OpenSession(path)
	require.NoError(t, err)

	afterReload, err := PrepareCompact(reloaded.BuildContext(), settings)
	require.NoError(t, err)

	assert.Equal(t, inMemory.FirstKeptEntryId, afterReload.FirstKeptEntryId)
	assert.Equal(t, inMemory.TokensBefore, afterReload.TokensBefore)
	assert.NotEmpty(t, afterReload.MessagesToSummarize)
}

func TestFindCutIndex_ReloadedEntriesRespectTokenBudget(t *testing.T) {
	path, _ := seedSession(t, 4)

	reloaded, err := session.OpenSession(path)
	require.NoError(t, err)
	entries := reloaded.BuildContext()

	cutPoints := make([]int, 0, len(entries))
	for i, entry := range entries {
		if entry.GetType() == session.EntryMessage {
			cutPoints = append(cutPoints, i)
		}
	}

	// 10+20+30+40 = 100 tokens; a 25-token budget is blown by the tail alone,
	// so the cut must not fall back to the earliest cut point.
	cutIndex := findCutIndex(entries, 0, len(entries), 25, cutPoints)
	assert.NotEqual(t, cutPoints[0], cutIndex)
}
