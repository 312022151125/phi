package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMessageImageRoundTrip(t *testing.T) {
	orig := Message{
		Role:    RoleUser,
		Content: "what is this?",
		Images: []Image{
			{Data: "QUJD", MimeType: "image/png"},
		},
	}
	body, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"images"`) || !strings.Contains(string(body), `"mimeType"`) {
		t.Fatalf("expected images in persisted JSON: %s", body)
	}

	var got Message
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Images) != 1 || got.Images[0].Data != "QUJD" || got.Images[0].MimeType != "image/png" {
		t.Fatalf("unexpected round-trip: %+v", got.Images)
	}
	if got.Content != orig.Content || got.Role != orig.Role {
		t.Fatalf("text fields changed in round-trip: %+v", got)
	}
}

func TestMessageWithoutImagesMarshalsPlain(t *testing.T) {
	body, err := json.Marshal(Message{Role: RoleUser, Content: "hi"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(body), `"images"`) {
		t.Fatalf("expected no images key: %s", body)
	}
}
