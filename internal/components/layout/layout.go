package layout

import (
	"strings"

	"github.com/pulseaiclub/xui"

	"github.com/pulseaiclub/phi/internal/components"
)

// BorderStyle selects box-drawing characters.
type BorderStyle int

// Border styles for drawable boxes.
const (
	BorderRounded BorderStyle = iota
	BorderSquare
)

type borderChars struct {
	tl, tr, bl, br, h, v string
}

func borderGlyphs(s BorderStyle) borderChars {
	if s == BorderSquare {
		return borderChars{tl: "┌", tr: "┐", bl: "└", br: "┘", h: "─", v: "│"}
	}
	return borderChars{tl: "╭", tr: "╮", bl: "╰", br: "╯", h: "─", v: "│"}
}

// BorderLabel is text embedded into a border edge.
type BorderLabel struct {
	Text  string
	Style xui.Style
}

// DrawRoundedBorder paints a rounded (or square) box onto s and embeds labels
// into the top/bottom edges. Labels on the right are right-aligned with a 1-cell
// gap from the corner; left labels leave a 1-cell gap from the left corner.
func DrawRoundedBorder(
	s *components.Surface,
	style BorderStyle,
	borderStyle xui.Style,
	topLeft, topRight, bottomLeft, bottomRight *BorderLabel,
	method xui.WidthMethod,
) {
	w, h := s.Size.Width, s.Size.Height
	if w < 2 || h < 2 {
		return
	}
	g := borderGlyphs(style)
	bs := borderStyle

	put := func(x, y int, ch string, st xui.Style) {
		s.SetCell(x, y, xui.Cell{Char: ch, Width: 1, Style: st})
	}

	// Corners + edges
	put(0, 0, g.tl, bs)
	put(w-1, 0, g.tr, bs)
	put(0, h-1, g.bl, bs)
	put(w-1, h-1, g.br, bs)
	for x := 1; x < w-1; x++ {
		put(x, 0, g.h, bs)
		put(x, h-1, g.h, bs)
	}
	for y := 1; y < h-1; y++ {
		put(0, y, g.v, bs)
		put(w-1, y, g.v, bs)
	}

	embed := func(y int, left, right *BorderLabel) {
		avail := w - 2 // between corners
		if avail < 1 {
			return
		}
		leftW, rightW := 0, 0
		if left != nil && left.Text != "" {
			leftW = xui.StringWidth(left.Text, method)
		}
		if right != nil && right.Text != "" {
			rightW = xui.StringWidth(right.Text, method)
		}
		// Prefer right label if they collide.
		if leftW+rightW > avail {
			if rightW >= avail {
				rightW = avail
				leftW = 0
			} else {
				leftW = avail - rightW
			}
		}
		if left != nil && leftW > 0 {
			text := TruncateToWidth(left.Text, leftW, method)
			s.Print(1, y, text, left.Style, method)
		}
		if right != nil && rightW > 0 {
			text := TruncateToWidth(right.Text, rightW, method)
			tw := xui.StringWidth(text, method)
			x := w - 1 - tw
			x = max(x, 1)
			s.Print(x, y, text, right.Style, method)
		}
	}
	embed(0, topLeft, topRight)
	embed(h-1, bottomLeft, bottomRight)
}

// TruncateToWidth returns the longest prefix of s that fits within max columns.
func TruncateToWidth(s string, max int, method xui.WidthMethod) string {
	if max <= 0 {
		return ""
	}
	if xui.StringWidth(s, method) <= max {
		return s
	}
	var b strings.Builder
	w := 0
	rest := s
	for rest != "" {
		cluster, cw, next := xui.FirstGrapheme(rest, method)
		rest = next
		if cw < 1 {
			cw = 1
		}
		if w+cw > max {
			break
		}
		b.WriteString(cluster)
		w += cw
	}
	return b.String()
}
