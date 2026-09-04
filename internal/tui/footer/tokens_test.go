package footer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/session"
)

func TestFormatTokens(t *testing.T) {
	cases := map[int]string{
		0:       "0",
		999:     "999",
		1200:    "1.2k",
		15000:   "15k",
		1500000: "1.5M",
	}
	for n, want := range cases {
		if got := formatTokens(n); got != want {
			t.Fatalf("formatTokens(%d)=%q want %q", n, got, want)
		}
	}
}

func TestFormatContextLabel(t *testing.T) {
	u := session.TokenUsage{PromptTokens: 5120, TotalTokens: 6000}
	got := formatContextLabel(u, 128000)
	if got != "4%/128k" {
		t.Fatalf("got %q", got)
	}
	if formatContextLabel(session.TokenUsage{}, 128000) != "" {
		t.Fatal("empty usage should hide label")
	}
	if formatContextLabel(u, 0) != "" {
		t.Fatal("zero window should hide label")
	}
}

func TestFormatUsageStats(t *testing.T) {
	got := formatUsageStats(session.TokenUsage{
		PromptTokens:     1200,
		CompletionTokens: 800,
		TotalTokens:      2000,
	})
	if got != "↑1.2k ↓800 Σ2.0k" {
		t.Fatalf("got %q", got)
	}
	got = formatUsageStats(session.TokenUsage{
		PromptTokens:     1200,
		CompletionTokens: 800,
		CachedTokens:     900,
		TotalTokens:      2000,
	})
	if got != "↑1.2k ↓800 C900 Σ2.0k" {
		t.Fatalf("got %q", got)
	}
}

func TestJoinBorderParts(t *testing.T) {
	if got := joinBorderParts(
		"↑1.2k ↓800 Σ2.0k",
		"context: 4% of 128k",
	); got != "↑1.2k ↓800 Σ2.0k context: 4% of 128k" {
		t.Fatalf("got %q", got)
	}
	if got := joinBorderParts("", "context: 4% of 128k"); got != "context: 4% of 128k" {
		t.Fatalf("got %q", got)
	}
	if got := joinBorderParts("↑1.2k", ""); got != "↑1.2k" {
		t.Fatalf("got %q", got)
	}
	if got := joinBorderParts("", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestTokenStatusLabelAmbient(t *testing.T) {
	th := components.DefaultTheme()
	u := session.TokenUsage{PromptTokens: 1200, CompletionTokens: 800, TotalTokens: 2000}
	label := tokenStatusLabel(th, u, 128000)
	assert.Equal(t, ChromeLabelStyle(th), label.Style)
	assert.Contains(t, label.Text, "↑1.2k")
	assert.Contains(t, label.Text, "%/128k")
	assert.Empty(t, label.Spans)
}

func TestTokenStatusLabelPressureEscalatesContextOnly(t *testing.T) {
	th := components.DefaultTheme()
	// 95% of 100k → danger tier on default window thresholds.
	u := session.TokenUsage{PromptTokens: 95000, TotalTokens: 95000}
	label := tokenStatusLabel(th, u, 100000)
	require.NotEmpty(t, label.Spans)
	assert.Equal(t, ChromeLabelStyle(th), label.Spans[0].Style)
	assert.Equal(t, th.Destructive, label.Spans[1].Style)
	assert.Contains(t, label.Spans[1].Text, "%")
}
