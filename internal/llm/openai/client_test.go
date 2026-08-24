package openai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/llm"
)

func TestBuildRequestImages(t *testing.T) {
	cfg := llm.ModelConfig{Name: "gpt-4o", APIKey: "k", BaseURL: "https://api.openai.com/v1"}
	req := BuildRequest(cfg, "sys", []llm.Message{
		{
			Role:    llm.RoleUser,
			Content: "what is this?",
			Images: []llm.Image{
				{Data: "QUJD", MimeType: "image/png"},
			},
		},
	}, nil)

	require.Len(t, req.Messages, 2) // system + user
	parts, ok := req.Messages[1].Content.([]any)
	require.True(t, ok, "expected content parts array with images, got %T", req.Messages[1].Content)
	require.Len(t, parts, 2)

	text := parts[0].(map[string]string)
	assert.Equal(t, "text", text["type"])
	assert.Equal(t, "what is this?", text["text"])

	img := parts[1].(map[string]any)
	assert.Equal(t, "image_url", img["type"])
	imageURL := img["image_url"].(map[string]string)
	assert.Equal(t, "data:image/png;base64,QUJD", imageURL["url"])

	body, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"image_url"`)
	assert.NotContains(t, string(body), `"images"`)
}

func TestBuildRequestNoImagesKeepsStringContent(t *testing.T) {
	cfg := llm.ModelConfig{Name: "gpt-4o", APIKey: "k", BaseURL: "https://api.openai.com/v1"}
	req := BuildRequest(cfg, "", []llm.Message{{Role: llm.RoleUser, Content: "hi"}}, nil)

	body, err := json.Marshal(req)
	require.NoError(t, err)
	// content must stay a JSON string, not an array, when there are no images.
	var raw struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(body, &raw))
	require.Len(t, raw.Messages, 1)
	assert.Equal(t, `"hi"`, string(raw.Messages[0].Content))
}
