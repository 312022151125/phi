package compaction

import "testing"

func TestShouldCompact(t *testing.T) {
	settings := Settings{
		enabled:          true,
		reverseTokens:    10,
		keepRecentTokens: 20000,
	}

	// threshold = contextWindow - reverseTokens = 90
	if ShouldCompact(50, 100, settings) {
		t.Fatalf("expected ShouldCompact=false when contextTokens below threshold")
	}
	if ShouldCompact(90, 100, settings) {
		t.Fatalf("expected ShouldCompact=false when contextTokens equals threshold")
	}
	if !ShouldCompact(95, 100, settings) {
		t.Fatalf("expected ShouldCompact=true when contextTokens above threshold")
	}

	disabled := settings
	disabled.enabled = false
	if ShouldCompact(95, 100, disabled) {
		t.Fatalf("expected ShouldCompact=false when compaction disabled")
	}

	if ShouldCompact(95, 0, settings) {
		t.Fatalf("expected ShouldCompact=false when contextWindow <= 0")
	}

	// threshold clamping when reverseTokens > contextWindow
	settings2 := settings
	settings2.reverseTokens = 200
	// threshold becomes 0
	if ShouldCompact(0, 100, settings2) {
		t.Fatalf("expected ShouldCompact=false when contextTokens==0 and threshold clamped to 0")
	}
	if !ShouldCompact(1, 100, settings2) {
		t.Fatalf("expected ShouldCompact=true when contextTokens>0 and threshold clamped to 0")
	}
}
