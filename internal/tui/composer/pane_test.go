package composer

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulseaiclub/xui"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/mention"
	"github.com/pulseaiclub/phi/internal/tui/commands"
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

func TestTopRightLabelPairsModelLeftOfThink(t *testing.T) {
	th := components.DefaultTheme()
	c := NewComposerPane(th, "sonnet", "/tmp")
	assert.Equal(t, "sonnet", c.Chat.TopRightLabel.Text)
	assert.Equal(t, th.IdentityOrSuccess(), c.Chat.TopRightLabel.Style)

	c.SetModelLabel("sonnet", "high")
	require.Len(t, c.Chat.TopRightLabel.Spans, 3)
	assert.Equal(t, "sonnet", c.Chat.TopRightLabel.Spans[0].Text)
	assert.Equal(t, th.IdentityOrSuccess(), c.Chat.TopRightLabel.Spans[0].Style)
	assert.Equal(t, "high", c.Chat.TopRightLabel.Spans[2].Text)
	assert.Equal(t, th.IdentityOrSuccess(), c.Chat.TopRightLabel.Spans[2].Style)

	c.SetModelLabel("sonnet", "off")
	assert.Equal(t, "sonnet", c.Chat.TopRightLabel.Text)
	assert.Equal(t, th.IdentityOrSuccess(), c.Chat.TopRightLabel.Style)
	assert.Empty(t, c.Chat.TopRightLabel.Spans)
}

func TestSlashTabCompletesWithoutSubmitting(t *testing.T) {
	c, bus := wiredComposer(t)
	openSlashPicker(t, c, "/cl", "cl")

	ctx := &components.EventContext{}
	c.Handle(ctx, xui.KeyEvent{Code: xui.KeyTab, Press: true})

	assert.Equal(t, "/clear ", c.Chat.Value, "Tab should fill the command in the composer")
	assert.Empty(t, submittedText(t, bus), "Tab must not run the command")
	assert.False(t, c.slash.Open)
}

func TestSlashTabCompletesNeedsArgsCommand(t *testing.T) {
	c, bus := wiredComposer(t)
	openSlashPicker(t, c, "/di", "di")

	ctx := &components.EventContext{}
	c.Handle(ctx, xui.KeyEvent{Code: xui.KeyTab, Press: true})

	assert.Equal(t, "/diff ", c.Chat.Value, "Trailing space stays single")
	assert.Empty(t, submittedText(t, bus))
}

func TestSlashEnterStillSubmits(t *testing.T) {
	c, bus := wiredComposer(t)
	openSlashPicker(t, c, "/cl", "cl")

	ctx := &components.EventContext{}
	c.Handle(ctx, xui.KeyEvent{Code: xui.KeyEnter, Press: true})

	assert.Equal(t, "/clear", c.Chat.Value)
	assert.Equal(t, "/clear", submittedText(t, bus), "Enter keeps running a no-arg command")
}

func TestMentionTabCompletesIntoComposer(t *testing.T) {
	c, bus := wiredComposer(t)
	c.mention.SetResults([]mention.Item{{Path: "go.mod"}}, "")
	c.mention.Show()
	c.Chat.MentionOpen = true
	c.Chat.Value = "@go"
	c.Chat.Cursor = len(c.Chat.Value)

	ctx := &components.EventContext{}
	c.Handle(ctx, xui.KeyEvent{Code: xui.KeyTab, Press: true})

	assert.Equal(t, "@go.mod ", c.Chat.Value)
	assert.Empty(t, submittedText(t, bus))
	assert.False(t, c.mention.Open)
}

// wiredComposer builds a composer wired like the app, with a small command
// registry: `clear` (no args, runs on Enter) and `diff` (needs args).
func wiredComposer(t *testing.T) (*ComposerPane, *controller.Bus) {
	t.Helper()
	c := NewComposerPane(components.DefaultTheme(), "m", t.TempDir())
	bus := controller.NewBus(nil)
	reg := commands.NewCommandRegistry()
	reg.Register(commands.Command{Name: "clear", Slash: true})
	reg.Register(commands.Command{Name: "diff", Slash: true, NeedsArgs: true})
	c.Wire(nil, nil, reg, c.cwd, bus, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	return c, bus
}

// openSlashPicker types value into the composer and opens the slash picker.
func openSlashPicker(t *testing.T, c *ComposerPane, value, query string) {
	t.Helper()
	c.Chat.Value = value
	c.Chat.Cursor = len(value)
	c.onSlashChange(true, query)
	require.True(t, c.slash.Open, "slash picker should be open")
}

// submittedText drains SubmitMsg texts published on the bus.
func submittedText(t *testing.T, bus *controller.Bus) string {
	t.Helper()
	out := ""
	for _, m := range bus.Drain() {
		if sm, ok := m.(controller.SubmitMsg); ok {
			out = sm.Text
		}
	}
	return out
}
