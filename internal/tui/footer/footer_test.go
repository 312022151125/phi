package footer

import (
	"strings"
	"testing"

	"github.com/pulseaiclub/xui"
	"github.com/stretchr/testify/assert"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/tui/controller"
)

func TestDrawCombinesParts(t *testing.T) {
	f := NewFooterChrome(components.DefaultTheme(), 0)
	f.SetHookStatus(" review ")
	f.SetLiveJobs(func() int { return 2 })
	f.Activity().Apply(controller.ActivityCancelled)

	row := components.SurfaceText(f.Draw(components.DrawContext{Method: xui.WidthUnicode}, 40))
	assert.Contains(t, row, "review · Stopped · 2 jobs")
}

func TestDrawSingleJobSingular(t *testing.T) {
	f := NewFooterChrome(components.DefaultTheme(), 0)
	f.SetLiveJobs(func() int { return 1 })

	row := components.SurfaceText(f.Draw(components.DrawContext{Method: xui.WidthUnicode}, 40))
	assert.Contains(t, row, "1 job")
}

func TestDrawEmptyFooter(t *testing.T) {
	f := NewFooterChrome(components.DefaultTheme(), 0)

	row := components.SurfaceText(f.Draw(components.DrawContext{Method: xui.WidthUnicode}, 40))
	assert.Empty(t, strings.TrimSpace(row))
}
