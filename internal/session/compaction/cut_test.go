package compaction

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/session"
)

func TestFindCutIndex_ExceedsTokensChoosesNearestCutPoint(t *testing.T) {
	entries := []session.MessageEntry{
		msgEntry("e1", llm.RoleUser, 10),
		msgEntry("e2", llm.RoleUser, 20),
		session.CompactionEntry{}, // non-message, should be skipped for token accumulation
		msgEntry("e3", llm.RoleUser, 30),
		msgEntry("e4", llm.RoleUser, 40),
	}

	startIndex := 0
	endIndex := len(entries)
	keepRecentTokens := 50
	cutPoints := []int{1, 3, 4}

	cutIndex := findCutIndex(entries, startIndex, endIndex, keepRecentTokens, cutPoints)

	assert.Equal(t, 3, cutIndex)
}

func TestFindCutIndex_NotExceedTokensReturnsFirstCutPoint(t *testing.T) {
	entries := []session.MessageEntry{
		msgEntry("e1", llm.RoleUser, 10),
		msgEntry("e2", llm.RoleUser, 20),
		msgEntry("e3", llm.RoleUser, 30),
	}

	startIndex := 0
	endIndex := len(entries)
	keepRecentTokens := 200
	cutPoints := []int{0, 1, 2}

	cutIndex := findCutIndex(entries, startIndex, endIndex, keepRecentTokens, cutPoints)

	assert.Equal(t, cutPoints[0], cutIndex)
}
