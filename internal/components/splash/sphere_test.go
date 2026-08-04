package splash

import (
	"testing"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/xui"
)

func TestSphereDrawPhiMark(t *testing.T) {
	sphere := &Sphere{Width: 20, Height: 20, Time: 0.5}
	surf := sphere.Draw(components.DrawContext{
		Max:    components.Size{Width: 20, Height: 20},
		Method: xui.WidthUnicode,
	})
	if surf.Size.Width != 20 || surf.Size.Height != 20 {
		t.Fatalf("size = %dx%d", surf.Size.Width, surf.Size.Height)
	}
	nonEmpty := 0
	for _, c := range surf.Buffer {
		if c.Char != "" && c.Char != " " {
			nonEmpty++
		}
	}
	if nonEmpty < 30 {
		t.Fatalf("expected phi mark cells, got %d non-empty", nonEmpty)
	}
	// Center column should carry the vertical stroke.
	mid := 10
	stemHits := 0
	for y := 0; y < 20; y++ {
		c := surf.Buffer[y*20+mid]
		if c.Char != "" && c.Char != " " {
			stemHits++
		}
	}
	// Stem still crosses the center; allow a slightly lower hit count once tilted.
	if stemHits < 4 {
		t.Fatalf("expected stem near center column, got %d hits", stemHits)
	}
}
