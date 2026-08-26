package status

import (
	"strings"
	"time"

	"github.com/pulseaiclub/xui"

	"github.com/pulseaiclub/phi/internal/components"
)

// Spinner — animated activity indicator. Advance with Tick().
//
// Tool/thinking rows use a 1-cell braille glyph. The footer uses a
// Knight-Rider scan bar (■ trail on ⬝) so the two sites
// don't compete with the same mark.
type Spinner struct {
	Frame    int
	Style    xui.Style
	frames   []string
	scan     []string
	Interval time.Duration
}

const (
	scanOn    = '■'
	scanOff   = '⬝'
	scanW     = 6
	scanTrail = 2
)

// NewSpinner returns a spinner with a 1-cell braille glyph and a footer scan bar.
func NewSpinner(style xui.Style) *Spinner {
	return &Spinner{
		Style:    style,
		Interval: 80 * time.Millisecond,
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		scan:     knightRider(scanW, scanTrail),
	}
}

func knightRider(width, trail int) []string {
	if width < 2 {
		width = 2
	}
	if trail < 1 {
		trail = 1
	}
	type step struct{ head, dir int }
	steps := make([]step, 0, width*2)
	for i := 0; i < width; i++ {
		steps = append(steps, step{i, 1})
	}
	for i := width - 2; i >= 1; i-- {
		steps = append(steps, step{i, -1})
	}
	out := make([]string, 0, len(steps))
	var b strings.Builder
	for _, st := range steps {
		b.Reset()
		for c := 0; c < width; c++ {
			behind := (st.head - c) * st.dir
			if behind >= 0 && behind < trail {
				b.WriteRune(scanOn)
			} else {
				b.WriteRune(scanOff)
			}
		}
		out = append(out, b.String())
	}
	return out
}

// Tick advances the spinner to the next frame.
func (s *Spinner) Tick() {
	n := len(s.frames)
	if ns := len(s.scan); ns > n {
		n = ns
	}
	if n == 0 {
		return
	}
	s.Frame = (s.Frame + 1) % n
}

// Handle is a no-op; spinners do not take input.
func (*Spinner) Handle(_ *components.EventContext, _ xui.Event) {}

// Draw renders the current spinner frame as a single cell.
func (s *Spinner) Draw(_ components.DrawContext) components.Surface {
	ch := "⋯"
	if len(s.frames) > 0 {
		ch = s.frames[s.Frame%len(s.frames)]
	}
	st := s.Style
	if st == (xui.Style{}) {
		st = components.DefaultTheme().ToolName
	}
	surf := components.NewSurface(1, 1, s)
	surf.SetCell(0, 0, xui.Cell{Char: ch, Width: 1, Style: st})
	return surf
}

// Glyph returns the current 1-cell spinner character (tool / thinking rows).
func (s *Spinner) Glyph() string {
	if s == nil || len(s.frames) == 0 {
		return "⋯"
	}
	return s.frames[s.Frame%len(s.frames)]
}

// Scan returns the current footer scanner string (Knight-Rider bar).
func (s *Spinner) Scan() string {
	if s == nil || len(s.scan) == 0 {
		return s.Glyph()
	}
	return s.scan[s.Frame%len(s.scan)]
}

// PaintScan draws Scan() with the head/trail in `on` and the rest in `off`.
func (s *Spinner) PaintScan(dst *components.Surface, x, y int, on, off xui.Style, method xui.WidthMethod) int {
	if s == nil || dst == nil {
		return 0
	}
	start := x
	for _, r := range s.Scan() {
		st := off
		if r == scanOn {
			st = on
		}
		ch := string(r)
		dst.Print(x, y, ch, st, method)
		x += xui.StringWidth(ch, method)
	}
	return x - start
}

// ToolStatus mirrors tool status icons in transcript blocks.
type ToolStatus int

// Tool status values for tool rows.
const (
	ToolDone ToolStatus = iota
	ToolRunning
	ToolError
	ToolCancelled
	ToolQueued
	ToolRejected
)
