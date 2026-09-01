package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/tui/composer"
	"github.com/pulseaiclub/phi/internal/tui/controller"
)

func TestEditorAppliesBranchLabel(t *testing.T) {
	e := &Editor{composer: composer.NewComposerPane(components.DefaultTheme(), "m", "/tmp")}
	e.composer.Wire(nil, nil, nil, "", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	e.composer.Chat.BottomRightLabel.Text = "~ (old)"
	e.Update(controller.BranchLabelMsg{Text: "~ (new)"})
	assert.Equal(t, "~ (new)", e.composer.Chat.BottomRightLabel.Text)
}
