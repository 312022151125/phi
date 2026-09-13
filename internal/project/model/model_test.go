package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/llm/gemini"
	"github.com/pulseaiclub/phi/internal/llm/openai"
)

func TestLookupDeepSeekFlash(t *testing.T) {
	p, ok := Lookup("deepseek-flash")
	require.True(t, ok)
	assert.Equal(t, "deepseek-flash", p.Config.Name)
	assert.Equal(t, "https://api.deepseek.com", p.Config.BaseURL)
	assert.Equal(t, 1_000_000, p.Config.ContextWindow)
	assert.True(t, p.Config.ImageEnabled, "V4.1-Flash accepts image input")
	assert.NotNil(t, p.Hooks.OpenAI)
}

func TestLookupDeepSeekV4Pro(t *testing.T) {
	p, ok := Lookup("deepseek-v4-pro")
	require.True(t, ok)
	assert.Equal(t, "deepseek-v4-pro", p.Config.Name)
	assert.Equal(t, "https://api.deepseek.com", p.Config.BaseURL)
	assert.Equal(t, 1_000_000, p.Config.ContextWindow)
	assert.False(t, p.Config.ImageEnabled, "V4-Pro has no image understanding")
	assert.NotNil(t, p.Hooks.OpenAI)
}

func TestLookupUnknownFallsThrough(t *testing.T) {
	// Legacy / custom names are not presets; callers apply the generic
	// OpenAI default themselves.
	_, ok := Lookup("deepseek-chat")
	assert.False(t, ok)
	_, ok = Lookup("some-unknown-model")
	assert.False(t, ok)

	// A preset never leaks api_key or skill path — those stay caller-owned.
	p, ok := Lookup("deepseek-flash")
	require.True(t, ok)
	assert.Empty(t, p.Config.APIKey)
	assert.Empty(t, p.Config.SkillPath)
	assert.Equal(t, llm.ModelConfig{
		Name:          "deepseek-flash",
		BaseURL:       "https://api.deepseek.com",
		ContextWindow: 1_000_000,
		ImageEnabled:  true,
		API:           llm.OpenAI,
		Think:         llm.ThinkConfig{Enabled: true, Mode: llm.High},
	}, p.Config)
}

func TestDeepSeekInterceptorSetsExtraBody(t *testing.T) {
	req := &openai.Request{Model: "deepseek-flash"}
	require.NoError(t, deepseekThinking{}.Before(t.Context(), req, llm.ModelConfig{}))
	require.NotNil(t, req.ExtraBody)
	require.NotNil(t, req.ExtraBody.Thinking)
	assert.Equal(t, "enabled", req.ExtraBody.Thinking.Type)

	body, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"extra_body"`)
	assert.Contains(t, string(body), `"thinking"`)
}

func TestHooksForUnknownIsZero(t *testing.T) {
	h := HooksFor("no-such-model")
	assert.Nil(t, h.OpenAI)
	assert.Nil(t, h.Anthropic)
	assert.Nil(t, h.Gemini)
}

func TestGeminiBudgetHookUsesLiveThink(t *testing.T) {
	p, ok := Lookup("gemini-2.5-flash")
	require.True(t, ok)
	require.NotNil(t, p.Hooks.Gemini)

	req := &gemini.GeminiRequest{}
	cfg := llm.ModelConfig{Think: llm.ThinkConfig{Enabled: true, Mode: llm.Low}}
	require.NoError(t, p.Hooks.Gemini.Before(t.Context(), req, cfg))
	body, err := json.Marshal(req.ThinkingConfig)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"thinkingBudget":2048`)
}

func TestGeminiLevelHookOffFloor(t *testing.T) {
	p, ok := Lookup("gemini-3-pro")
	require.True(t, ok)
	require.NotNil(t, p.Hooks.Gemini)

	req := &gemini.GeminiRequest{}
	require.NoError(t, p.Hooks.Gemini.Before(t.Context(), req, llm.ModelConfig{}))
	body, err := json.Marshal(req.ThinkingConfig)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"thinkingLevel":"LOW"`)
}
