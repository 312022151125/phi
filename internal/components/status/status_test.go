package status

import (
	"testing"

	"github.com/pulseaiclub/xui"
)

func TestSpinnerGlyphs(t *testing.T) {
	sp := NewSpinner(xui.Style{})
	glyphs := map[string]struct{}{}
	scans := map[string]struct{}{}
	for i := range 20 {
		g := sp.Glyph()
		if g == "" || xui.StringWidth(g, xui.WidthUnicode) != 1 {
			t.Fatalf("frame %d glyph %q width %d", i, g, xui.StringWidth(g, xui.WidthUnicode))
		}
		sc := sp.Scan()
		if xui.StringWidth(sc, xui.WidthUnicode) != scanW {
			t.Fatalf("frame %d scan %q width %d want %d", i, sc, xui.StringWidth(sc, xui.WidthUnicode), scanW)
		}
		glyphs[g] = struct{}{}
		scans[sc] = struct{}{}
		sp.Tick()
	}
	if len(glyphs) != 10 {
		t.Fatalf("want 10 unique braille frames, got %d", len(glyphs))
	}
	if len(scans) < 6 {
		t.Fatalf("scan bar did not bounce, got %d frames", len(scans))
	}
}
