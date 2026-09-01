// Package extpane renders non-blocking extension panes over the transcript.
package extpane

import (
	"strings"

	"github.com/pulseaiclub/xui"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/layout"
	"github.com/pulseaiclub/phi/internal/tui/controller"
)

// Pane holds one extension pane instance.
type Pane struct {
	ID      string
	Title   string
	Body    string
	Format  string
	Actions []controller.ExtPaneAction
	Source  string
	scroll  int
	focus   int // action index; -1 = body
}

// Host owns the visible extension panes (topmost is drawn).
type Host struct {
	theme    components.Theme
	panes    map[string]*Pane
	order    []string // z-order, last = top
	onAction func(controller.ExtPaneActionMsg)
}

// New builds an empty pane host.
func New(theme components.Theme, onAction func(controller.ExtPaneActionMsg)) *Host {
	return &Host{
		theme:    theme,
		panes:    make(map[string]*Pane),
		onAction: onAction,
	}
}

// SetTheme updates chrome styling.
func (h *Host) SetTheme(th components.Theme) {
	if h != nil {
		h.theme = th
	}
}

// Active reports whether any pane is open.
func (h *Host) Active() bool {
	return h != nil && len(h.order) > 0
}

// Apply routes ExtPaneMsg.
func (h *Host) Apply(msg controller.ExtPaneMsg) {
	if h == nil {
		return
	}
	id := strings.TrimSpace(msg.ID)
	if id == "" {
		id = "default"
	}
	switch msg.Op {
	case "show":
		p := &Pane{
			ID:      id,
			Title:   msg.Title,
			Body:    msg.Body,
			Format:  msg.Format,
			Actions: append([]controller.ExtPaneAction(nil), msg.Actions...),
			Source:  msg.Source,
			focus:   -1,
		}
		h.panes[id] = p
		h.bringToFront(id)
	case "update":
		if p := h.panes[id]; p != nil {
			p.Body = msg.Body
			if msg.Title != "" {
				p.Title = msg.Title
			}
			h.bringToFront(id)
		}
	case "close":
		delete(h.panes, id)
		h.removeOrder(id)
	}
}

// Top returns the frontmost pane.
func (h *Host) Top() *Pane {
	if h == nil || len(h.order) == 0 {
		return nil
	}
	return h.panes[h.order[len(h.order)-1]]
}

// HandleKey handles keys when a pane is focused (Esc closes, arrows scroll/actions).
func (h *Host) HandleKey(ctx *components.EventContext, e xui.KeyEvent) bool {
	p := h.Top()
	if p == nil || !e.Press {
		return false
	}
	switch e.Code {
	case xui.KeyEscape:
		h.Apply(controller.ExtPaneMsg{Op: "close", ID: p.ID})
		ctx.ConsumeAndRedraw()
		return true
	case xui.KeyUp:
		if p.focus > 0 {
			p.focus--
		} else if p.focus == 0 {
			p.focus = -1
		} else if p.scroll > 0 {
			p.scroll--
		}
		ctx.ConsumeAndRedraw()
		return true
	case xui.KeyDown, xui.KeyTab:
		if len(p.Actions) > 0 && p.focus < len(p.Actions)-1 {
			p.focus++
		} else {
			lines := strings.Count(p.Body, "\n") + 1
			if p.scroll < lines-1 {
				p.scroll++
			}
		}
		ctx.ConsumeAndRedraw()
		return true
	case xui.KeyEnter:
		if p.focus >= 0 && p.focus < len(p.Actions) {
			act := p.Actions[p.focus]
			if h.onAction != nil {
				h.onAction(controller.ExtPaneActionMsg{
					PaneID: p.ID, ActionID: act.ID, Source: p.Source,
				})
			}
		}
		ctx.ConsumeAndRedraw()
		return true
	case xui.KeyRune:
		if e.Mods.Has(xui.ModCtrl) || e.Mods.Has(xui.ModAlt) {
			return false
		}
		if e.Rune >= '1' && e.Rune <= '9' {
			idx := int(e.Rune - '1')
			if idx < len(p.Actions) {
				act := p.Actions[idx]
				if h.onAction != nil {
					h.onAction(controller.ExtPaneActionMsg{
						PaneID: p.ID, ActionID: act.ID, Source: p.Source,
					})
				}
				ctx.ConsumeAndRedraw()
				return true
			}
		}
	}
	return false
}

// PreferredHeight estimates rows for the pane overlay.
func (h *Host) PreferredHeight(termH int) int {
	p := h.Top()
	if p == nil {
		return 0
	}
	lines := 4 + strings.Count(p.Body, "\n") + 1
	if len(p.Actions) > 0 {
		lines += len(p.Actions) + 1
	}
	maxH := termH / 2
	maxH = max(maxH, 8)
	if lines > maxH {
		return maxH
	}
	if lines < 6 {
		return 6
	}
	return lines
}

// Draw renders the top pane into a surface.
func (h *Host) Draw(ctx components.DrawContext, width, height int) components.Surface {
	p := h.Top()
	if p == nil {
		return components.NewSurface(width, height, nil)
	}
	th := h.theme
	innerW := width - 4
	if innerW < 10 {
		innerW = width
	}

	var body []components.RichLine
	add := func(spans ...components.Span) {
		body = append(body, components.WrapSpans(spans, innerW, ctx.Method)...)
	}

	title := p.Title
	if title == "" {
		title = "Extension"
	}
	add(components.Span{Text: title, Style: xui.Style{Bold: true, Fg: th.Command.Fg}})
	body = append(body, components.RichLine{})

	lines := strings.Split(p.Body, "\n")
	start := p.scroll
	start = max(start, 0)
	start = min(start, len(lines))
	visible := height - 5 - len(p.Actions)
	visible = max(visible, 3)
	end := start + visible
	end = min(end, len(lines))
	for _, line := range lines[start:end] {
		add(components.Span{Text: line, Style: th.Foreground})
	}
	if end < len(lines) {
		add(components.Span{Text: "…", Style: th.Muted})
	}

	if len(p.Actions) > 0 {
		body = append(body, components.RichLine{})
		for i, act := range p.Actions {
			label := act.Label
			if label == "" {
				label = act.ID
			}
			sel := i == p.focus
			arrow := " "
			style := th.Foreground
			if sel {
				arrow = "▸"
				style = xui.Style{Bold: true, Fg: th.Success.Fg}
				if act.Kind == "danger" {
					style = xui.Style{Bold: true, Fg: th.Destructive.Fg}
				}
			} else if act.Kind == "danger" {
				style = th.Destructive
			} else if act.Kind == "primary" {
				style = th.Success
			}
			hint := ""
			if i < 9 {
				hint = "  [" + string(rune('1'+i)) + "]"
			}
			body = append(body, components.WrapSpans([]components.Span{
				{Text: arrow + " " + label, Style: style},
				{Text: hint, Style: th.Muted},
			}, innerW, ctx.Method)...)
		}
	}
	body = append(body, components.WrapSpans([]components.Span{
		{Text: "Esc close · ↑↓ scroll/actions · Enter activate", Style: th.Muted},
	}, innerW, ctx.Method)...)

	panel := components.NewSurface(width, height, nil)
	layout.DrawRoundedBorder(&panel, layout.BorderRounded, th.Border, nil, nil, nil, nil, ctx.Method)
	y := 1
	for _, line := range body {
		if y >= height-1 {
			break
		}
		components.PaintSpans(&panel, 2, y, line, ctx.Method)
		y++
	}
	return panel
}

func (h *Host) bringToFront(id string) {
	h.removeOrder(id)
	h.order = append(h.order, id)
}

func (h *Host) removeOrder(id string) {
	out := h.order[:0]
	for _, x := range h.order {
		if x != id {
			out = append(out, x)
		}
	}
	h.order = out
}
