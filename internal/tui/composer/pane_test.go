package composer

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/mention"
	"github.com/pulseaiclub/phi/internal/tui/controller"
)

// png1x1Base64 is a 1x1 transparent PNG (same fixture as util/image tests).
const png1x1Base64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func TestTryAttachClipboardImageBlockedWithoutModelSupport(t *testing.T) {
	c := NewComposerPane(components.DefaultTheme(), "m", "/tmp")
	bus := controller.NewBus(nil)
	c.bus = bus
	c.imageEnabled = func() bool { return false }

	ctx := &components.EventContext{}
	require.True(t, c.tryAttachClipboardImage(ctx))
	assert.True(t, ctx.Consume)
	assert.Contains(t, drainToast(t, bus), "does not support images")
}

func TestTryAttachClipboardImageFallsThroughWhenSupported(t *testing.T) {
	c := NewComposerPane(components.DefaultTheme(), "m", "/tmp")
	c.imageEnabled = func() bool { return true }

	ctx := &components.EventContext{}
	// No clipboard tooling in CI: the gate passes and the read reports
	// ErrUnavailable, so the key is not consumed.
	require.False(t, c.tryAttachClipboardImage(ctx))
}

func TestTryAttachClipboardImageAllowedWithoutModelInfo(t *testing.T) {
	c := NewComposerPane(components.DefaultTheme(), "m", "/tmp")

	ctx := &components.EventContext{}
	require.False(t, c.tryAttachClipboardImage(ctx))
}

func TestAcceptMentionImageBlockedWithoutModelSupport(t *testing.T) {
	dir := t.TempDir()
	png, err := base64.StdEncoding.DecodeString(png1x1Base64)
	require.NoError(t, err)
	path := filepath.Join(dir, "pixel.png")
	require.NoError(t, os.WriteFile(path, png, 0o644))

	c := NewComposerPane(components.DefaultTheme(), "m", dir)
	bus := controller.NewBus(nil)
	c.bus = bus
	c.imageEnabled = func() bool { return false }
	c.Chat.Value = "@pixel.png"
	c.Chat.Cursor = len(c.Chat.Value)

	c.acceptMention(mention.Item{Path: "pixel.png"})

	assert.Contains(t, drainToast(t, bus), "does not support images")
	assert.Empty(t, c.Chat.PendingImages)
	assert.Equal(t, "@pixel.png ", c.Chat.Value)
}

// drainToast returns the message of the last queued ToastMsg and empties the bus.
func drainToast(t *testing.T, bus *controller.Bus) string {
	t.Helper()
	var msg string
	for _, m := range bus.Drain() {
		if tm, ok := m.(controller.ToastMsg); ok {
			msg = tm.Message
		}
	}
	return msg
}

func TestAcceptMentionImageAttachesWhenSupported(t *testing.T) {
	dir := t.TempDir()
	png, err := base64.StdEncoding.DecodeString(png1x1Base64)
	require.NoError(t, err)
	path := filepath.Join(dir, "pixel.png")
	require.NoError(t, os.WriteFile(path, png, 0o644))

	c := NewComposerPane(components.DefaultTheme(), "m", dir)
	c.imageEnabled = func() bool { return true }
	c.Chat.Value = "@pixel.png"
	c.Chat.Cursor = len(c.Chat.Value)

	c.acceptMention(mention.Item{Path: "pixel.png"})

	require.Len(t, c.Chat.PendingImages, 1)
	assert.Equal(t, "pixel.png", c.Chat.PendingImages[0].Label)
	assert.Empty(t, c.Chat.Value)
}
