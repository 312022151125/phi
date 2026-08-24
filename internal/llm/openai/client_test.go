package openai

import (
	"encoding/json"
	"strings"
	"testing"

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

	if len(req.Messages) != 2 {
		t.Fatalf("expected system + user messages, got %d", len(req.Messages))
	}
	parts, ok := req.Messages[1].Content.([]any)
	if !ok {
		t.Fatalf("expected content parts array with images, got %T", req.Messages[1].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (text + image), got %d", len(parts))
	}
	text := parts[0].(map[string]string)
	if text["type"] != "text" || text["text"] != "what is this?" {
		t.Fatalf("unexpected text part: %+v", text)
	}
	img := parts[1].(map[string]any)
	if img["type"] != "image_url" {
		t.Fatalf("expected image_url part, got %+v", img)
	}
	imageURL := img["image_url"].(map[string]string)
	if want := "data:image/png;base64,QUJD"; imageURL["url"] != want {
		t.Fatalf("expected url %q, got %q", want, imageURL["url"])
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"image_url"`) {
		t.Fatalf("expected image_url in wire body: %s", string(body))
	}
	if strings.Contains(string(body), `"images"`) {
		t.Fatalf("images field must not leak into the wire body: %s", string(body))
	}
}

func TestBuildRequestNoImagesKeepsStringContent(t *testing.T) {
	cfg := llm.ModelConfig{Name: "gpt-4o", APIKey: "k", BaseURL: "https://api.openai.com/v1"}
	req := BuildRequest(cfg, "", []llm.Message{{Role: llm.RoleUser, Content: "hi"}}, nil)

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// content must stay a JSON string, not an array, when there are no images.
	var raw struct {
		Messages []struct {
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Messages) != 1 || string(raw.Messages[0].Content) != `"hi"` {
		t.Fatalf("expected string content \"hi\", got %s", string(raw.Messages[0].Content))
	}
}
