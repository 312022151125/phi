package block

import (
	"fmt"
	"strings"

	components2 "github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/xui"
)

// MaxBashPreviewLines matches the maximum number of preview lines
// (last N lines when truncated).
const MaxBashPreviewLines = 15

// BashStatus mirrors bash tool status.
type BashStatus int

const (
	BashDone BashStatus = iota
	BashRunning
	BashError
	BashCancelled
)

// BashBlock renders bash tool output:
//
//	$ ls
//	  [... 14 lines truncated ...] Show more
//	  parser.go
//	  ...
type BashBlock struct {
	Command  string
	Output   string
	Status   BashStatus
	ExitCode int
	Expanded bool
	Theme    components2.Theme

	// OnToggle is called when the user expands/collapses (click title / Enter).
	OnToggle func(expanded bool)
	// OnShowMore is called when "Show more" is activated.
	OnShowMore func(fullOutput string)

	showMoreHit hitRange // filled during Draw for mouse
	titleH      int      // title row count; body clicks don't toggle (allow selection)
}

type hitRange struct {
	valid     bool
	x0, x1, y int
}

func (bashBlock *BashBlock) theme() components2.Theme {
	if bashBlock.Theme.Success.Fg.Kind == 0 && bashBlock.Theme.Foreground.Fg.Kind == 0 {
		return components2.DefaultTheme()
	}
	return bashBlock.Theme
}

func (bashBlock *BashBlock) Handle(ctx *components2.EventContext, ev xui.Event) {
	switch e := ev.(type) {
	case xui.KeyEvent:
		if e.Code == xui.KeyEnter || (e.Code == xui.KeyRune && e.Rune == ' ') {
			if bashBlock.hasBody() {
				bashBlock.Expanded = !bashBlock.Expanded
				if bashBlock.OnToggle != nil {
					bashBlock.OnToggle(bashBlock.Expanded)
				}
				ctx.ConsumeAndRedraw()
			}
		}
	case xui.MouseEvent:
		if e.Action != xui.MousePress || e.Button != xui.MouseLeft {
			return
		}
		if bashBlock.showMoreHit.valid && e.Y == bashBlock.showMoreHit.y && e.X >= bashBlock.showMoreHit.x0 && e.X < bashBlock.showMoreHit.x1 {
			if bashBlock.OnShowMore != nil {
				bashBlock.OnShowMore(bashBlock.Output)
			} else {
				bashBlock.Expanded = true
			}
			ctx.ConsumeAndRedraw()
			return
		}
		// Only the title toggles expand; body stays selectable for copy-on-select.
		if bashBlock.hasBody() && e.Y >= 0 && e.Y < bashBlock.titleH {
			bashBlock.Expanded = !bashBlock.Expanded
			if bashBlock.OnToggle != nil {
				bashBlock.OnToggle(bashBlock.Expanded)
			}
			ctx.ConsumeAndRedraw()
		}
	}
}

// CopyText returns "$ command" plus output when present.
func (bashBlock *BashBlock) CopyText() string {
	var sb strings.Builder
	sb.WriteString("$ ")
	sb.WriteString(bashBlock.Command)
	out := strings.TrimRight(bashBlock.Output, "\n")
	if out != "" {
		sb.WriteByte('\n')
		sb.WriteString(out)
	}
	return sb.String()
}

func (bashBlock *BashBlock) hasBody() bool {
	return strings.TrimSpace(bashBlock.Output) != "" || (bashBlock.Status == BashError)
}

func (bashBlock *BashBlock) Draw(ctx components2.DrawContext) components2.Surface {
	th := bashBlock.theme()
	w := ctx.Max.Width
	if w <= 0 {
		w = 40
	}
	bashBlock.showMoreHit = hitRange{}

	prefixStyle := th.Success
	switch bashBlock.Status {
	case BashError:
		prefixStyle = th.Destructive
	case BashRunning:
		prefixStyle = th.ToolName
	case BashCancelled:
		prefixStyle = th.Muted
	}

	cmdStyle := th.Foreground
	if bashBlock.Status == BashCancelled {
		cmdStyle.Strikethrough = true
	}

	title := []components2.Span{
		{Text: "$ ", Style: prefixStyle},
		{Text: bashBlock.Command, Style: cmdStyle},
	}
	if bashBlock.Status == BashDone && bashBlock.ExitCode != 0 {
		title = append(title,
			components2.Span{Text: " (", Style: xui.Style{Italic: true}},
			components2.Span{Text: "exit code: ", Style: xui.Style{Italic: true}},
			components2.Span{Text: fmt.Sprintf("%d", bashBlock.ExitCode), Style: xui.Style{Italic: true, Fg: th.Destructive.Fg}},
			components2.Span{Text: ")", Style: xui.Style{Italic: true}},
		)
	}
	if bashBlock.hasBody() {
		arrow := " ▶"
		if bashBlock.Expanded {
			arrow = " ▼"
		}
		title = append(title, components2.Span{Text: arrow, Style: th.Muted})
	}

	titleWrapped := components2.WrapSpans(title, w, ctx.Method)
	var bodyLines []components2.RichLine
	titleH := len(titleWrapped)
	bashBlock.titleH = titleH
	if bashBlock.Expanded && bashBlock.hasBody() {
		var hit hitRange
		bodyLines = bashBodyLines(bashBlock.Output, true, th, w-2, ctx.Method, &hit)
		if hit.valid {
			hit.y += titleH
			hit.x0 += 2
			hit.x1 += 2
			bashBlock.showMoreHit = hit
		}
	}

	h := titleH + len(bodyLines)
	if h < 1 {
		h = 1
	}
	s := components2.NewSurface(w, h, bashBlock)
	y := 0
	for _, line := range titleWrapped {
		components2.PaintSpans(&s, 0, y, line, ctx.Method)
		y++
	}
	for _, line := range bodyLines {
		components2.PaintSpans(&s, 2, y, line, ctx.Method)
		y++
	}
	return s
}

func bashBodyLines(output string, showMore bool, th components2.Theme, width int, method xui.WidthMethod, hit *hitRange) []components2.RichLine {
	if output == "" {
		return nil
	}
	text := strings.ReplaceAll(output, "\r", "")
	text = strings.TrimRight(text, "\n")
	lines := strings.Split(text, "\n")
	dim := th.Muted
	dim.Dim = true
	fg := th.Foreground
	fg.Dim = true

	var spans []components2.Span
	if len(lines) > MaxBashPreviewLines {
		n := len(lines) - MaxBashPreviewLines
		trunc := fmt.Sprintf("[... %d lines truncated ...] ", n)
		spans = append(spans, components2.Span{Text: trunc, Style: fg})
		if showMore {
			link := "Show more"
			if hit != nil {
				// x positions within the first painted body line (before left pad)
				hit.valid = true
				hit.x0 = xui.StringWidth(trunc, method)
				hit.x1 = hit.x0 + xui.StringWidth(link, method)
				hit.y = 0
			}
			spans = append(spans, components2.Span{Text: link, Style: th.Accent})
		}
		spans = append(spans, components2.Span{Text: "\n", Style: fg})
		spans = append(spans, components2.Span{Text: strings.Join(lines[len(lines)-MaxBashPreviewLines:], "\n") + "\n", Style: fg})
	} else {
		spans = append(spans, components2.Span{Text: strings.Join(lines, "\n") + "\n", Style: fg})
		_ = dim
	}
	if width < 1 {
		width = 1
	}
	return components2.WrapSpans(spans, width, method)
}
