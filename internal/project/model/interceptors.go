package model

import (
	"context"

	"github.com/pulseaiclub/phi/internal/llm"
	llmclient "github.com/pulseaiclub/phi/internal/llm/client"
	"github.com/pulseaiclub/phi/internal/llm/gemini"
	"github.com/pulseaiclub/phi/internal/llm/openai"
)

// deepseekThinking enables DeepSeek's extra_body.thinking on OpenAI-shaped requests.
type deepseekThinking struct{}

func (deepseekThinking) Before(_ context.Context, req *openai.Request, _ llm.ModelConfig) error {
	req.ExtraBody = &openai.ExtraBody{Thinking: &openai.ThinkingConfig{Type: "enabled"}}
	return nil
}

func deepseekHooks() llmclient.Hooks {
	return llmclient.Hooks{OpenAI: deepseekThinking{}}
}

// geminiBudgetThinking maps ThinkMode → thinkingBudget (Gemini 2.x).
type geminiBudgetThinking struct{}

func (geminiBudgetThinking) Before(_ context.Context, req *gemini.GeminiRequest, cfg llm.ModelConfig) error {
	req.ApplyBudgetThinking(cfg.Think)
	return nil
}

// geminiLevelThinking maps ThinkMode → thinkingLevel (Gemini 3.x).
// offLevel is the floor when thinking is disabled.
type geminiLevelThinking struct {
	offLevel string
}

func (h geminiLevelThinking) Before(_ context.Context, req *gemini.GeminiRequest, cfg llm.ModelConfig) error {
	req.ApplyLevelThinking(cfg.Think, h.offLevel)
	return nil
}

func geminiBudgetHooks() llmclient.Hooks {
	return llmclient.Hooks{Gemini: geminiBudgetThinking{}}
}

func geminiLevelHooks(offLevel string) llmclient.Hooks {
	return llmclient.Hooks{Gemini: geminiLevelThinking{offLevel: offLevel}}
}
