package compaction

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/llm"
)

// captureCompactor records the prompts handed to the LLM so tests can assert on
// what actually reaches the provider.
type captureCompactor struct {
	prompts []string
}

func (c *captureCompactor) Compact(_ context.Context, prompt string) (string, error) {
	c.prompts = append(c.prompts, prompt)
	return "SUMMARY", nil
}

func TestGenerateTurnPrefixSummary_IncludesPrompt(t *testing.T) {
	c := &captureCompactor{}

	got, err := generateTurnPrefixSummary(
		t.Context(),
		c,
		[]llm.Message{{Role: llm.RoleUser, Content: "fix the parser"}},
	)

	require.NoError(t, err)
	assert.Equal(t, "SUMMARY", got)
	require.Len(t, c.prompts, 1)
	assert.Contains(t, c.prompts[0], "</conversation>\n\n"+compactionTurnPrefixPrompt,
		"the turn-prefix prompt must follow the conversation")
	assert.Contains(t, c.prompts[0], "## Original Request")
	assert.Contains(t, c.prompts[0], "## Context for Suffix")
}

func TestGenerateSummary_IncludesPrompt(t *testing.T) {
	c := &captureCompactor{}

	_, err := generateSummary(
		t.Context(),
		c,
		[]llm.Message{{Role: llm.RoleUser, Content: "hello"}},
		"",
	)

	require.NoError(t, err)
	require.Len(t, c.prompts, 1)
	assert.Contains(t, c.prompts[0], "[User]: hello")
	assert.Contains(t, c.prompts[0], compactionSummaryPrompt)
}
