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
	tools      []llm.ToolDefinition
	system     string
}

// NewClient builds a streaming chat client.
func NewClient(cfg llm.ModelConfig, tools []llm.ToolDefinition, systemPrompt string) *Client {
	return &Client{
		httpClient: util.DefaultHTTPClient(),
		cfg:        cfg,
		tools:      tools,
		system:     systemPrompt,
	}
}

// Stream runs a streaming chat completion over messages (+ optional system prompt / tools).
func (c *Client) Stream(ctx context.Context, messages []llm.Message) iter.Seq2[llm.StreamEvent, error] {
	switch c.cfg.API {
	case llm.Anthropic:
		req := anthropic.BuildRequest(c.cfg, c.system, messages, c.tools)
		return anthropic.Stream(ctx, c.httpClient, c.cfg, &req)
	case llm.Gemini:
		req := gemini.BuildRequest(c.system, messages, c.tools)
		return gemini.Stream(ctx, c.httpClient, c.cfg, &req)
	default: // llm.OpenAI or empty — both route to OpenAI-compatible
		req := openai.BuildRequest(c.cfg, c.system, messages, c.tools)
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
		return gemini.Compact(ctx, c.httpClient, c.cfg, prompt)
	default:
		return openai.Compact(ctx, c.httpClient, c.cfg, prompt)
	}
}
