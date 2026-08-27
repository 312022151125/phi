package submit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/status"
	"github.com/pulseaiclub/phi/internal/session"
	"github.com/pulseaiclub/phi/internal/tui/commands"
	"github.com/pulseaiclub/phi/internal/tui/controller"
	"github.com/pulseaiclub/phi/internal/tui/transcript"
	imgutil "github.com/pulseaiclub/phi/internal/util/image"
)

type stubComposer struct {
	skills []string
	images []imgutil.Attachment
}

func (stubComposer) HideCompleters()                       {}
func (stubComposer) ClearInput()                           {}
func (s stubComposer) PendingSkills() []string             { return s.skills }
func (s stubComposer) PendingImages() []imgutil.Attachment { return s.images }
func (stubComposer) ClearPendingSkills()                   {}
func (stubComposer) ClearPendingImages()                   {}
func (stubComposer) SyncBashBorder(string)                 {}
func (stubComposer) CloseMentionSlash()                    {}
func (stubComposer) SetBashBorderActive(bool)              {}

func TestSubmitter_IsBusy(t *testing.T) {
	th := components.DefaultTheme()
	spin := status.NewSpinner(th.ToolName)
	transcript := transcript.NewTranscriptPane(th, spin, "Phi test")

	sub := NewSubmitter(nil, nil, transcript, nil, stubComposer{}, nil, nil, nil, nil, nil, nil)

	if sub.IsBusy() {
		t.Fatal("expected idle submitter")
	}
}

func TestSubmitter_StreamActive_activity(t *testing.T) {
	th := components.DefaultTheme()
	spin := status.NewSpinner(th.ToolName)
	activity := controller.NewActivityHandler(spin)
	sub := NewSubmitter(
		nil,
		nil,
		transcript.NewTranscriptPane(th, spin, "Phi test"),
		activity,
		stubComposer{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	activity.Apply(controller.ActivityWaiting)
	if !sub.StreamActive() {
		t.Fatal("expected stream active while waiting")
	}
}

func TestSubmitter_Submit_unknownSlashFallsThroughToAgent(t *testing.T) {
	th := components.DefaultTheme()
	spin := status.NewSpinner(th.ToolName)
	tp := transcript.NewTranscriptPane(th, spin, "Phi test")
	sub := NewSubmitter(
		nil,
		commands.NewBuiltinRegistry(),
		tp,
		nil,
		stubComposer{},
		func() commands.CommandContext { return commands.CommandContext{} },
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	sub.Submit("/not-a-real-command")
	require.Len(t, tp.Snapshot().Messages, 1)
	assert.Equal(t, "/not-a-real-command", tp.Snapshot().Messages[0].Text)
	assert.Equal(t, session.RoleUser, tp.Snapshot().Messages[0].Role)
}

func TestSubmitter_Submit_bareBangFallsThroughToAgent(t *testing.T) {
	th := components.DefaultTheme()
	spin := status.NewSpinner(th.ToolName)
	tp := transcript.NewTranscriptPane(th, spin, "Phi test")
	sub := NewSubmitter(
		nil,
		nil,
		tp,
		nil,
		stubComposer{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	sub.Submit("!")
	require.Len(t, tp.Snapshot().Messages, 1)
	assert.Equal(t, "!", tp.Snapshot().Messages[0].Text)
}

func TestSubmitter_Submit_withImagesOnly(t *testing.T) {
	th := components.DefaultTheme()
	spin := status.NewSpinner(th.ToolName)
	tp := transcript.NewTranscriptPane(th, spin, "Phi test")
	images := []imgutil.Attachment{
		{Label: "a.png", Result: imgutil.Result{Data: []byte("abc"), MimeType: "image/png"}},
	}
	sub := NewSubmitter(
		nil,
		nil,
		tp,
		nil,
		stubComposer{images: images},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	sub.Submit("")
	require.Len(t, tp.Snapshot().Messages, 1)
	msg := tp.Snapshot().Messages[0]
	assert.Equal(t, session.RoleUser, msg.Role)
	assert.Equal(t, "Images: a.png", msg.Text)
	require.Len(t, msg.Images, 1)
	assert.Equal(t, "image/png", msg.Images[0].MimeType)
	assert.NotEmpty(t, msg.Images[0].Data)
}

func TestBuildUserDisplay(t *testing.T) {
	got := buildUserDisplay("hello", []string{"foo"}, []imgutil.Attachment{
		{Label: "a.png", Result: imgutil.Result{MimeType: "image/png"}},
	})
	assert.Equal(t, "Skills: foo\nImages: a.png\nhello", got)

	assert.Equal(t, "Images: clip", buildUserDisplay("", nil, []imgutil.Attachment{
		{Label: "clip", Result: imgutil.Result{Data: []byte("x"), MimeType: "image/png"}},
	}))
}
