package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/pulseaiclub/phi/internal/util"
)

const chatCompletionsPath = "/chat/completions"

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type apiTool struct {
	Type     string         `json:"type"`
	Function ToolDefinition `json:"function"`
}

type apiRequest struct {
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	Tools         []apiTool      `json:"tools,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
	ExtraBody     *ExtraBody     `json:"extra_body,omitempty"`
}

// ExtraBody holds provider-specific request fields (e.g. DeepSeek thinking).
type ExtraBody struct {
	Thinking *ThinkingConfig `json:"thinking,omitempty"`
}

// ThinkingConfig enables reasoning mode.
type ThinkingConfig struct {
	Type string `json:"type"`
}

// ModelConfig is the connection config for one OpenAI-compatible endpoint.
// It also carries agent-wide settings like the skill directory path.
type ModelConfig struct {
	Name    string
	APIKey  string
	BaseURL string
	// SkillPath is the directory to scan for SKILL.md files.
	// Defaults to ~/.phi/skills if empty.
	SkillPath string
	// ContextWindow is the model's context window in tokens.
	// Zero disables session compaction (safe default).
	ContextWindow int
}

// Client talks to an OpenAI-compatible /chat/completions endpoint.
type Client struct {
	httpClient *http.Client
	cfg        ModelConfig
	tools      []ToolDefinition
	system     string
}

// NewClient builds a streaming chat client.
func NewClient(cfg ModelConfig, tools []ToolDefinition, systemPrompt string) *Client {
	return &Client{
		httpClient: util.DefaultHTTPClient(),
		cfg:        cfg,
		tools:      tools,
		system:     systemPrompt,
	}
}

// Stream runs a streaming chat completion over messages (+ optional system prompt / tools).
func (c *Client) Stream(ctx context.Context, messages []Message) iter.Seq2[StreamEvent, error] {
	return func(yield func(StreamEvent, error) bool) {
		req := c.buildRequest(messages)
		for ev, err := range StreamChatCompletion(ctx, c.httpClient, c.cfg.BaseURL, c.cfg.APIKey, req) {
			if !yield(ev, err) {
				return
			}
		}
	}
}

// Compact sends a single non-streaming chat request and returns the
// assistant text. It satisfies llm.Compactor for session compaction.
func (c *Client) Compact(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(&apiRequest{
		Model:    c.cfg.Name,
		Messages: []Message{{Role: RoleUser, Content: prompt}},
	})
	if err != nil {
		return "", err
	}

	url := c.cfg.BaseURL
	if !strings.HasSuffix(url, chatCompletionsPath) {
		url += chatCompletionsPath
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	httpResp, err := util.DoWithRetry(c.httpClient, httpReq)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", err
	}
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error: (%d) %s", httpResp.StatusCode, string(respBody))
	}

	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM API error: empty choices")
	}
	return resp.Choices[0].Message.Content, nil
}

func (c *Client) buildRequest(messages []Message) *apiRequest {
	msgs := make([]Message, 0, len(messages)+1)
	if strings.TrimSpace(c.system) != "" {
		msgs = append(msgs, Message{Role: RoleSystem, Content: c.system})
	}
	msgs = append(msgs, messages...)

	tools := make([]apiTool, len(c.tools))
	for i, t := range c.tools {
		tools[i] = apiTool{Type: "function", Function: t}
	}

	var extra *ExtraBody
	if isThinkingModeModel(c.cfg.Name) {
		extra = &ExtraBody{Thinking: &ThinkingConfig{Type: "enabled"}}
	}

	return &apiRequest{
		Model:         c.cfg.Name,
		Messages:      msgs,
		Tools:         tools,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
		ExtraBody:     extra,
	}
}

func isThinkingModeModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(model), "deepseek")
}

// StreamChatCompletion POSTs a streaming chat completion and yields normalized events.
func StreamChatCompletion(
	ctx context.Context,
	httpClient *http.Client,
	baseURL string,
	apiKey string,
	payload any,
) iter.Seq2[StreamEvent, error] {
	return func(yield func(StreamEvent, error) bool) {
		body, err := json.Marshal(payload)
		if err != nil {
			yield(StreamEvent{}, err)
			return
		}

		url := baseURL
		if !strings.HasSuffix(url, chatCompletionsPath) {
			url += chatCompletionsPath
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			yield(StreamEvent{}, err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("Accept", util.ContentEventStream)

		httpResp, err := util.DoWithRetry(httpClient, httpReq)
		if err != nil {
			yield(StreamEvent{}, err)
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(httpResp.Body)
			yield(StreamEvent{}, fmt.Errorf("LLM API error: (%d) %s", httpResp.StatusCode, string(respBody)))
			return
		}

		out := Response{}
		acc := newStreamAccumulator()

		for data, parseErr := range util.ParseDataStream(httpResp.Body) {
			if parseErr != nil {
				yield(StreamEvent{}, parseErr)
				return
			}
			payloadLine := bytes.TrimSpace(data)
			if len(payloadLine) == 0 {
				continue
			}
			if bytes.Equal(payloadLine, []byte("[DONE]")) {
				break
			}
			decodeData := data
			if bytes.Contains(decodeData, []byte("\t")) {
				decodeData = bytes.ReplaceAll(decodeData, []byte("\t"), []byte(" "))
			}

			var chunk StreamChunk
			if err := json.Unmarshal(decodeData, &chunk); err != nil {
				continue
			}
			if chunk.Usage != nil {
				out.Usage = *chunk.Usage
			}
			if len(chunk.Choices) == 0 {
				continue
			}

			sc := chunk.Choices[0]
			delta := sc.Delta
			acc.applyDelta(delta)
			if sc.Message != nil {
				acc.applyMessage(sc.Message)
			}

			if hasStreamDelta(delta, sc.Message) {
				if !yield(StreamEvent{
					Type:    StreamEventTypeDelta,
					Delta:   delta,
					Partial: Response{Usage: out.Usage},
				}, nil) {
					return
				}
			}
		}

		msg := acc.message()
		out.Choices = []Choice{{Message: msg}}
		yield(StreamEvent{Type: StreamEventTypeDone, Partial: out}, nil)
	}
}

func hasStreamDelta(delta StreamDelta, msg *Message) bool {
	if delta.Content != "" || delta.ReasoningContent != "" || delta.Role != "" || len(delta.ToolCalls) > 0 {
		return true
	}
	if msg == nil {
		return false
	}
	return strings.TrimSpace(msg.Content) != "" ||
		strings.TrimSpace(msg.ReasoningContent) != "" ||
		len(msg.ToolCalls) > 0
}
