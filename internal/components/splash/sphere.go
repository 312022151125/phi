package splash

import (
	"math"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/xui"
)

// Classic density charset (dim → bright).
const sphereCharset = " .:-=+*#%@"

// Default palette: soft graphite → near-white (logo-like on dark terminals).
var (
	spherePrimary   = rgb{48, 52, 60}
	sphereSecondary = rgb{236, 240, 248}
)

type rgb struct{ r, g, b uint8 }

func lerpRGB(a, b rgb, t float64) rgb {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return rgb{
		r: uint8(math.Round(float64(a.r) + float64(b.r-a.r)*t)),
		g: uint8(math.Round(float64(a.g) + float64(b.g-a.g)*t)),
		b: uint8(math.Round(float64(a.b) + float64(b.b-a.b)*t)),
	}
}

// Sphere draws an animated φ mark: thick ring + vertical stroke (top→bottom).
// Drive animation by advancing Time (seconds) each frame with App.Anim.
type Sphere struct {
	Width  int // default 40
	Height int // default 40
	Time   float64
	// Fast speeds the downward glow flow (1.8x when true, else 1x).
	Fast bool

	noise   glowNoise
	palette []xui.Color
}

func (o *Sphere) ensure() {
	if o.Width <= 0 {
		o.Width = 40
	}
	if o.Height <= 0 {
		o.Height = 40
	}
	if o.noise.seed == 0 {
		o.noise = newGlowNoise(2654435761)
	}
	if len(o.palette) == 0 {
		const n = 64
		o.palette = make([]xui.Color, n)
		for i := 0; i < n; i++ {
			c := lerpRGB(spherePrimary, sphereSecondary, float64(i)/float64(n-1))
			o.palette[i] = xui.RGBColor(c.r, c.g, c.b)
		}
	}
}

func (o *Sphere) Handle(_ *components.EventContext, _ xui.Event) {}

func (o *Sphere) Draw(ctx components.DrawContext) components.Surface {
	o.ensure()
	w, h := o.Width, o.Height
	if max := ctx.Max.Width; max > 0 && w > max {
		w = max
	}
	if max := ctx.Max.Height; max > 0 && h > max {
		h = max
	}
	if w < 3 {
		w = 3
	}
	if h < 3 {
		h = 3
	}

	s := components.NewSurface(w, h, o)

	// Aspect correction: terminal cells are roughly twice as tall as wide.
	const kx = 0.5
	cx := float64(w) / 2
	cy := float64(h) / 2
	// Fit ring + protruding stem tips inside the canvas.
	rx := math.Max(1, float64(w)/2-1)
	ry := math.Max(1, float64(h)/(2*kx)-1)
	radius := math.Min(rx, ry) * 0.72 // leave room for stem tips
	invKx := 1 / kx

	innerR := radius * 0.58 // thick annulus like the logo
	stemHalf := radius * 0.085
	stemLen := radius * 1.32 // extends past the ring
	gapHalf := stemHalf * 2.1
	centerGap := radius * 0.07

	speed := 1.0
	if o.Fast {
		speed = 1.8
	}
	chars := []rune(sphereCharset)
	nChars := len(chars)
	nPal := len(o.palette)

	for row := 0; row < h; row++ {
		py := (float64(row) - cy) * invKx
		for col := 0; col < w; col++ {
			px := float64(col) - cx
			g := phiCoverage(px, py, radius, innerR, stemHalf, stemLen, gapHalf, centerGap)
			if g <= 0 {
				continue
			}
			// Downward flow: a bright band travels top → bottom along the mark.
			flow := 0.55 + 0.45*math.Sin(py*0.22-o.Time*speed*2.6)
			noise := o.noise.sample(float64(col), float64(row), o.Time, speed*0.35)
			g *= 0.62 + 0.28*flow + 0.10*noise
			if g <= 0.02 {
				continue
			}
			if g > 1 {
				g = 1
			}
			gi := int(math.Min(float64(nChars-1), math.Floor(g*float64(nChars))))
			pi := int(math.Min(float64(nPal-1), math.Floor(g*float64(nPal))))
			s.SetCell(col, row, xui.Cell{
				Char:  string(chars[gi]),
				Width: 1,
				Style: xui.Style{Fg: o.palette[pi]},
			})
		}
	}
	return s
}

// phiCoverage returns [0,1] coverage for the φ mark at aspect-corrected (px, py).
// Ring is split where the stroke crosses; stroke tapers to points and has a
// small center break. The stem is tilted slightly (/) like the brand mark.
func phiCoverage(px, py, outerR, innerR, stemHalf, stemLen, gapHalf, centerGap float64) float64 {
	d := math.Hypot(px, py)

	// ~10° from vertical toward / (bottom-left → top-right).
	const tilt = 0.18
	ct, st := math.Cos(tilt), math.Sin(tilt)
	lx := px*ct + py*st // distance from tilted stem axis
	ly := -px*st + py*ct
	ax := math.Abs(lx)
	ay := math.Abs(ly)

	var cover float64

	// Annulus, with clearance gaps where the stem cuts through.
	if d <= outerR && d >= innerR {
		ringSoft := softBand(d, innerR, outerR, (outerR-innerR)*0.18)
		if ax >= gapHalf {
			cover = math.Max(cover, ringSoft)
		} else {
			edge := smoothstep(gapHalf*0.35, gapHalf, ax)
			cover = math.Max(cover, ringSoft*edge)
		}
	}

	// Tilted stroke: widest near the ring band, tapering to needle tips.
	if ay <= stemLen {
		t := ay / stemLen // 0 center → 1 tip
		taper := 1 - t*t
		ringCross := 0.0
		midR := (innerR + outerR) * 0.5
		band := (outerR - innerR) * 0.55
		if d >= innerR*0.8 && d <= outerR*1.08 {
			ringCross = 1 - math.Abs(d-midR)/(band+1e-6)
			if ringCross < 0 {
				ringCross = 0
			}
		}
		halfW := stemHalf * (0.75 + 0.35*taper + 0.65*ringCross)
		if ax <= halfW*1.7 {
			stem := 1 - smoothstep(halfW*0.45, halfW*1.55, ax)
			stem *= 0.55 + 0.45*taper
			if ay < centerGap {
				stem *= smoothstep(centerGap*0.2, centerGap, ay)
			}
			stem *= 1 - smoothstep(stemLen*0.78, stemLen, ay)
			cover = math.Max(cover, stem)
		}
	}

	return cover
}

func softBand(d, inner, outer, feather float64) float64 {
	if feather < 1e-6 {
		if d >= inner && d <= outer {
			return 1
		}
		return 0
	}
	return smoothstep(inner-feather, inner+feather, d) * (1 - smoothstep(outer-feather, outer+feather, d))
}

func smoothstep(edge0, edge1, x float64) float64 {
	if edge0 == edge1 {
		if x < edge0 {
			return 0
		}
		return 1
	}
	t := (x - edge0) / (edge1 - edge0)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}
