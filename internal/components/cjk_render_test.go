package components

import (
	"strings"
	"testing"

	"github.com/pulseaiclub/xui"
)

func TestCJKTrailNotPaintedColumnByColumn(t *testing.T) {
	s := NewSurface(10, 1, nil)
	s.Print(0, 0, "笔记", xui.Style{}, xui.WidthUnicode)
	if !s.Buffer[1].Trail {
		t.Fatalf("expected trail at col 1, got %+v", s.Buffer[1])
	}
	screen := xui.NewScreen(10, 1)
	win := xui.NewWindow(screen)
	win.Clear()
	// Anti-pattern: paint every non-default cell. Trail must be skipped.
	for x := 0; x < s.Size.Width; x++ {
		c := s.Buffer[x]
		if !c.Default && !c.Trail {
			win.SetCell(x, 0, c)
		}
	}
	if got := screen.GetCell(0, 0); got.Char != "笔" || got.Width != 2 {
		t.Fatalf("primary = %+v", got)
	}
	if got := screen.GetCell(1, 0); !got.Trail {
		t.Fatalf("screen trail = %+v", got)
	}
	r := xui.NewRenderer()
	screen.MarkRefresh()
	var buf strings.Builder
	if _, err := r.RenderDiff(&buf, screen.Diff(), -1, -1, false, 0); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "笔\x1b[1;2H") {
		t.Fatal("ANSI writes into trail column after 笔")
	}
}
