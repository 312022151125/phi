package footer

import (
	"fmt"
	"strings"

	"github.com/pulseaiclub/xui"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/layout"
	"github.com/pulseaiclub/phi/internal/components/status"
	"github.com/pulseaiclub/phi/internal/session"
	"github.com/pulseaiclub/phi/internal/tui/controller"
)

type labelComposer interface {
	SetBottomLeftLabel(layout.BorderLabel)
	ClearBottomLeftLabel()
}

// FooterChrome owns activity status, spinner, token label, and footer hints.
type FooterChrome struct {
	theme         components.Theme
	spin          *status.Spinner
	activity      *controller.ActivityHandler
	contextWindow int
	lastUsage     session.TokenUsage
	updateHint    string
	hookStatus    string
	tick          int

	composer     labelComposer
	labelContext func() session.Snapshot
	liveJobs     func() int
}

// NewFooterChrome builds footer chrome with a fresh spinner and activity handler.
func NewFooterChrome(theme components.Theme, contextWindow int) *FooterChrome {
	spin := status.NewSpinner(theme.ToolName)
	return &FooterChrome{
		theme:         theme,
		spin:          spin,
		activity:      controller.NewActivityHandler(spin),
		contextWindow: contextWindow,
	}
}

// Spinner returns the shared spinner (e.g. for TranscriptPane mapper).
func (f *FooterChrome) Spinner() *status.Spinner {
	if f == nil {
		return nil
	}
	return f.spin
}

// Activity returns the activity handler.
func (f *FooterChrome) Activity() *controller.ActivityHandler {
	if f == nil {
		return nil
	}
	return f.activity
}

// BindComposer wires the composer for token display updates.
func (f *FooterChrome) BindComposer(c labelComposer) {
	if f != nil {
		f.composer = c
	}
}

// SetLabelContext supplies snap for activity footer labels.
func (f *FooterChrome) SetLabelContext(fn func() session.Snapshot) {
	if f != nil {
		f.labelContext = fn
	}
}

// SetLiveJobs supplies live sub-agent job count for the footer.
func (f *FooterChrome) SetLiveJobs(fn func() int) {
	if f != nil {
		f.liveJobs = fn
	}
}

// AdvanceTick drives spinner animation during active work.
func (f *FooterChrome) AdvanceTick() {
	if f == nil {
		return
	}
	f.tick++
	if f.activity.ShowSpinner() && f.tick%4 == 0 && f.spin != nil {
		f.spin.Tick()
	}
}

// SyncFromSnap refreshes activity from the session snapshot.
func (f *FooterChrome) SyncFromSnap(snap session.Snapshot) {
	if f != nil && f.activity != nil {
		f.activity.SyncFromSnap(snap)
	}
}

// SetTheme updates footer chrome styling.
func (f *FooterChrome) SetTheme(th components.Theme) {
	if f == nil {
		return
	}
	f.theme = th
	if f.spin != nil {
		f.spin.Style = th.ToolName
	}
	if f.lastUsage.Reported() {
		f.UpdateTokenDisplay(f.lastUsage)
	}
}

// UpdateTokenDisplay refreshes composer token/context labels from usage.
func (f *FooterChrome) UpdateTokenDisplay(usage session.TokenUsage) {
	if f == nil || !usage.Reported() {
		return
	}
	f.lastUsage = usage
	if f.composer == nil {
		return
	}
	combined := joinBorderParts(formatUsageStats(usage), formatContextLabel(usage, f.contextWindow))
	if combined == "" {
		f.composer.ClearBottomLeftLabel()
		return
	}
	f.composer.SetBottomLeftLabel(layout.BorderLabel{
		Text:  combined,
		Style: contextLabelStyle(f.theme, usage, f.contextWindow),
	})
}

// ClearTokenDisplay clears composer token stats (e.g. after /clear).
func (f *FooterChrome) ClearTokenDisplay() {
	if f != nil {
		f.lastUsage = session.TokenUsage{}
		if f.composer != nil {
			f.composer.ClearBottomLeftLabel()
		}
	}
}

// SetExtensionStatus overrides the footer activity label prefix.
func (f *FooterChrome) SetExtensionStatus(status string) {
	if f != nil {
		f.hookStatus = status
	}
}

// Apply handles footer bus messages.
func (f *FooterChrome) Apply(msg controller.FooterMsg) {
	if f == nil {
		return
	}
	switch msg.Kind {
	case controller.FooterSetActivity:
		f.activity.Apply(msg.Activity)
	case controller.FooterClearIfActivity:
		if f.activity.Current == msg.If {
			f.activity.Apply(controller.ActivityIdle)
		}
	case controller.FooterUpdateAvailable:
		latest := strings.TrimPrefix(msg.Latest, "v")
		f.updateHint = latest + " available · phi update"
	}
}

// ApplySessionEffects applies toast/status from session lifecycle extensions.
func (f *FooterChrome) ApplySessionEffects(msg controller.ExtSessionEffectsMsg) {
	if f == nil {
		return
	}
	if msg.StatusSet {
		f.hookStatus = msg.Status
	}
}

// Draw renders the one-row footer surface.
func (f *FooterChrome) Draw(ctx components.DrawContext, width int) components.Surface {
	if f == nil {
		return components.NewSurface(width, 1, nil)
	}
	footer := components.NewSurface(width, 1, nil)
	dim := f.theme.Muted
	var parts []string
	if hs := strings.TrimSpace(f.hookStatus); hs != "" {
		parts = append(parts, hs)
	}
	var snap session.Snapshot
	if f.labelContext != nil {
		snap = f.labelContext()
	}
	if a := f.activity.Label(snap); a != "" {
		parts = append(parts, a)
	}
	if f.liveJobs != nil {
		if n := f.liveJobs(); n > 0 {
			jobBit := fmt.Sprintf("%d job", n)
			if n != 1 {
				jobBit += "s"
			}
			parts = append(parts, jobBit)
		}
	}
	msg := strings.Join(parts, " · ")

	x := 1
	if msg != "" {
		if f.activity.ShowSpinner() && f.spin != nil {
			x += f.spin.PaintScan(&footer, x, 0, f.theme.ToolName, dim, ctx.Method)
			footer.Print(x, 0, " ", dim, ctx.Method)
			x += xui.StringWidth(" ", ctx.Method)
		}
		footer.Print(x, 0, msg, dim, ctx.Method)
		x += xui.StringWidth(msg, ctx.Method)
	}

	hint := strings.TrimSpace(f.updateHint)
	if hint != "" {
		hw := xui.StringWidth(hint, ctx.Method)
		hx := width - hw - 1
		hx = max(hx, x+2)
		if hx+hw <= width {
			st := f.theme.Warning
			st.Bold = false
			footer.Print(hx, 0, hint, st, ctx.Method)
		}
	}
	return footer
}
