package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	assert.Contains(t, string(body), `"images"`)
	assert.Contains(t, string(body), `"mimeType"`)

	var got Message
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, orig, got)
}

func TestMessageWithoutImagesMarshalsPlain(t *testing.T) {
	body, err := json.Marshal(Message{Role: RoleUser, Content: "hi"})
	require.NoError(t, err)
	assert.NotContains(t, string(body), `"images"`)
}
