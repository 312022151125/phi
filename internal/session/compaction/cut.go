package compaction

import (
	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/session"
)

// CutPointResult identifies where to cut the session history: the index of
// the first entry to keep and, when the cut splits a turn, that turn's
// starting index.
type CutPointResult struct {
	/** Index of first entry to keep */
	firstKeptEntryIndex int
	/** Index of user message that starts the turn being split, or -1 if not splitting */
	turnStartIndex int
	/** Whether this cut splits a turn (cut point is not a user message) */
	isSplitTurn bool
}

func findCutPoint(
	messageEntries []session.MessageEntry,
	startIndex int,
	endIndex int,
	keepRecentTokens int,
) CutPointResult {
	cutPoints := collectCutPoints(messageEntries, startIndex, endIndex)
	if len(cutPoints) == 0 {
		return CutPointResult{
			firstKeptEntryIndex: startIndex,
			turnStartIndex:      -1,
			isSplitTurn:         false,
		}
	}

	cutIndex := findCutIndex(
		messageEntries,
		startIndex,
		endIndex,
		keepRecentTokens,
		cutPoints,
	)

	// Entry at the cut point;
	// used to tell if we cut at a user message (cutting there does not split a turn).
	cutEntry := messageEntries[cutIndex]
	isUserMessage := cutEntry.GetType() == session.EntryMessage &&
		cutEntry.(session.SessionMessageEntry).Message.Role == llm.RoleUser

	// [userMessage, cutIndex) is the turn
	turnStartIndex := -1
	if !isUserMessage {
		turnStartIndex = findTurnStartIndex(messageEntries, cutIndex, startIndex)
	}

	return CutPointResult{
		firstKeptEntryIndex: cutIndex,
		turnStartIndex:      turnStartIndex,
		isSplitTurn:         !isUserMessage && turnStartIndex != -1,
	}
}

// findCutIndex walks entries backward from endIndex and
// accumulates token counts until they exceed keepRecentTokens. It then
// finds the first valid cut point that is at or after the position where
// the limit was exceeded. If no such point is found, it falls back to
// the earliest candidate cut point.
func findCutIndex(
	entries []session.MessageEntry,
	startIndex int,
	endIndex int,
	keepRecentTokens int,
	cutPoints []int,
) int {
	accumulatedTokens := 0
	cutIndex := cutPoints[0]

	for i := endIndex - 1; i >= startIndex; i-- {
		entry := entries[i]
		if entry.GetType() != session.EntryMessage {
			continue
		}

		msgEntry := entry.(session.SessionMessageEntry)
		accumulatedTokens += msgEntry.Message.Usage.TotalTokens

		if accumulatedTokens > keepRecentTokens {
			for _, point := range cutPoints {
				if point >= i {
					cutIndex = point
					break
				}
			}
			break
		}
	}

	return cutIndex
}

// collectCutPoints returns the indices of every user/assistant message
// entry in [startIndex, endIndex), in chronological order: the positions
// where a compaction cut may land. Picking among them is findCutIndex's job.
func collectCutPoints(entries []session.MessageEntry, startIndex, endIndex int) []int {
	var cutPoints []int

	for i := startIndex; i < endIndex; i++ {
		e := entries[i]
		if e.GetType() != session.EntryMessage {
			continue
		}
		msgEntry := e.(session.SessionMessageEntry)
		role := msgEntry.Message.Role
		if role == llm.RoleUser || role == llm.RoleAssistant {
			cutPoints = append(cutPoints, i)
		}
	}
	return cutPoints
}

// findTurnStartIndex walks backward from entryIndex to find the user message
// that starts the current turn.
func findTurnStartIndex(entries []session.MessageEntry, entryIndex, startIndex int) int {
	for i := entryIndex; i >= startIndex; i-- {
		entry := entries[i]
		ty := entry.GetType()

		if ty == session.EntryMessage {
			msgEntry := entry.(session.SessionMessageEntry)
			if msgEntry.Message.Role == llm.RoleUser {
				return i
			}
		}
	}
	return -1
}
