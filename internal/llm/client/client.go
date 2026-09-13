package client

import (
	"context"
	"iter"
	"net/http"

	"github.com/pulseaiclub/phi/internal/llm"
	"github.com/pulseaiclub/phi/internal/llm/anthropic"
	"github.com/pulseaiclub/phi/internal/llm/gemini"
	"github.com/pulseaiclub/phi/internal/llm/openai"
	"github.com/pulseaiclub/phi/internal/util"
)

// Client talks to the configured LLM endpoint: OpenAI-compatible by default,
// or Anthropic / Gemini when cfg.API is set accordingly.
type Client struct {
	httpClient *http.Client
	cfg        llm.ModelConfig
	hooks      Hooks
	tools      []llm.ToolDefinition
	system     string
}

// NewClient builds a streaming chat client.
func NewClient(cfg llm.ModelConfig, hooks Hooks, tools []llm.ToolDefinition, systemPrompt string) *Client {
	return &Client{
		httpClient: util.DefaultHTTPClient(),
		cfg:        cfg,
		hooks:      hooks,
		tools:      tools,
		system:     systemPrompt,
	}
}

// Stream runs a streaming chat completion over messages (+ optional system prompt / tools).
func (c *Client) Stream(ctx context.Context, messages []llm.Message) iter.Seq2[llm.StreamEvent, error] {
	switch c.cfg.API {
	case llm.Anthropic:
		req := anthropic.BuildRequest(c.cfg, c.system, messages, c.tools)
		if err := applyHook(ctx, c.hooks.Anthropic, &req, c.cfg); err != nil {
			return errorSeq(err)
		}
		return anthropic.Stream(ctx, c.httpClient, c.cfg, &req)
	case llm.Gemini:
		req := gemini.BuildRequest(c.system, messages, c.tools)
		if err := c.applyGeminiThinking(ctx, &req); err != nil {
			return errorSeq(err)
		}
		return gemini.Stream(ctx, c.httpClient, c.cfg, &req)
	default: // llm.OpenAI or empty — both route to OpenAI-compatible
		req := openai.BuildRequest(c.cfg, c.system, messages, c.tools)
		if err := applyHook(ctx, c.hooks.OpenAI, req, c.cfg); err != nil {
			return errorSeq(err)
		}
		return openai.StreamChatCompletion(ctx, c.httpClient, c.cfg.BaseURL, c.cfg.APIKey, req)
	}
}

// Compact sends a single non-streaming chat request and returns the
// assistant text. It satisfies llm.Compactor for session compaction.
func (c *Client) Compact(ctx context.Context, prompt string) (string, error) {
	switch c.cfg.API {
	case llm.Anthropic:
		return anthropic.Compact(ctx, c.httpClient, c.cfg, prompt)
	case llm.Gemini:
		req := gemini.BuildRequest("", []llm.Message{{Role: llm.RoleUser, Content: prompt}}, nil)
		if err := c.applyGeminiThinking(ctx, &req); err != nil {
			return "", err
		}
		return gemini.CompactRequest(ctx, c.httpClient, c.cfg, &req)
	default:
		req := openai.NewCompactRequest(c.cfg.Name, prompt)
		if err := applyHook(ctx, c.hooks.OpenAI, req, c.cfg); err != nil {
			return "", err
		}
		return openai.CompactRequest(ctx, c.httpClient, c.cfg, req)
	}
}

// applyGeminiThinking runs the Gemini hook. Without a preset hook, budget
// style is the safe default for unknown models.
func (c *Client) applyGeminiThinking(ctx context.Context, req *gemini.GeminiRequest) error {
	if c.hooks.Gemini != nil {
		return applyHook(ctx, c.hooks.Gemini, req, c.cfg)
	}
	req.ApplyBudgetThinking(c.cfg.Think)
	return nil
}

func errorSeq(err error) iter.Seq2[llm.StreamEvent, error] {
	return func(yield func(llm.StreamEvent, error) bool) {
		yield(llm.StreamEvent{}, err)
	}
}
