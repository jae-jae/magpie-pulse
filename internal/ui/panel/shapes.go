package panel

import (
	"magpie-pulse/internal/model"

	"github.com/egoist/mygo/ui"
)

const (
	circleBezierK      float32 = 0.5522847498
	superellipseBezier float32 = 0.63
)

// DockBerthPath builds Pulse's signature berth silhouette for the given rect,
// dock edge, metrics, end style, and openness (0..1).
func DockBerthPath(rect ui.Rect, edge model.DockEdge, metrics model.DockMetrics, roundEnds bool, openness float32) *ui.Path {
	var p ui.Path
	if rect.W <= 0 || rect.H <= 0 {
		return &p
	}
	openness = clamp01(openness)

	if edge == model.DockFloating {
		w := rect.W
		h := rect.H
		if openness < 1 {
			minDim := min(metrics.CollapsedWidth, min(rect.W, rect.H))
			w = minDim + (rect.W-minDim)*openness
			h = minDim + (rect.H-minDim)*openness
		}
		cx := rect.X + rect.W/2
		cy := rect.Y + rect.H/2
		r := ui.Rect{X: cx - w/2, Y: cy - h/2, W: w, H: h}
		rad := min(r.W, r.H) / 2
		addRoundedRect(&p, r, rad, circleBezierK)
		return &p
	}

	vertical := edge.IsVertical()
	fullThick := rect.W
	length := rect.H
	if vertical {
		fullThick = rect.H
		length = rect.W
	}

	collapsed := min(metrics.CollapsedWidth, fullThick)
	thick := collapsed + (fullThick-collapsed)*openness
	ratio := thick / max(fullThick, 1)

	cr := min(metrics.CornerRadius*ratio, thick)
	if roundEnds {
		cr = thick / 2
	}
	fw := max(thick-cr, 0)
	fh := metrics.FlareHeight * (0.3 + 0.7*openness)
	if roundEnds && openness >= 0.999 {
		fh = min(metrics.FlareHeight, length/2-cr)
	}
	if fh+cr > length/2 && (fh+cr) > 0 {
		scale := (length / 2) / (fh + cr)
		fh *= scale
		cr *= scale
		fw = max(thick-cr, 0)
	}

	kc := superellipseBezier
	kf := float32(0.58)
	if roundEnds {
		kc = circleBezierK
		kf = circleBezierK
	}

	pt := func(d, t float32) (float32, float32) {
		switch edge {
		case model.DockLeft:
			return rect.X + d, rect.Y + t
		case model.DockTop:
			return rect.X + t, rect.Y + d
		case model.DockBottom:
			return rect.X + t, rect.Y + rect.H - d
		default: // DockRight
			return rect.X + rect.W - d, rect.Y + t
		}
	}
	move := func(d, t float32) {
		x, y := pt(d, t)
		p.MoveTo(x, y)
	}
	line := func(d, t float32) {
		x, y := pt(d, t)
		p.LineTo(x, y)
	}
	cube := func(d1, t1, d2, t2, d3, t3 float32) {
		x1, y1 := pt(d1, t1)
		x2, y2 := pt(d2, t2)
		x3, y3 := pt(d3, t3)
		p.CubeTo(x1, y1, x2, y2, x3, y3)
	}

	move(0, 0)
	// Start concave flare from the flush screen edge.
	cube(0, fh*kf, fw*(1-kf), fh, fw, fh)
	// Leading outer corner.
	cube(fw+cr*kc, fh, thick, fh+cr*(1-kc), thick, fh+cr)
	// Straight outer edge.
	if length-2*(fh+cr) > 0.01 {
		line(thick, length-fh-cr)
	}
	// Trailing outer corner.
	cube(thick, length-fh-cr*(1-kc), fw+cr*kc, length-fh, fw, length-fh)
	// End concave flare returning flush to the screen edge.
	cube(fw*(1-kf), length-fh, 0, length-fh*kf, 0, length)
	p.Close()
	return &p
}

// UsageBubblePath builds Pulse's rounded detail card with a smooth Bezier
// pointer tail pointing toward pointerCenter on tailSide.
func UsageBubblePath(cardRect ui.Rect, tailSide model.DockEdge, pointerCenter float32, pointerW, pointerH, cornerRadius float32) *ui.Path {
	var p ui.Path
	if cardRect.W <= 0 || cardRect.H <= 0 {
		return &p
	}
	r := min(cornerRadius, min(cardRect.W, cardRect.H)/2)
	if r < 0 {
		r = 0
	}
	if tailSide == model.DockFloating || pointerW <= 0 || pointerH <= 0 {
		addRoundedRect(&p, cardRect, r, superellipseBezier)
		return &p
	}

	k := superellipseBezier
	x0, y0 := cardRect.X, cardRect.Y
	x1, y1 := cardRect.X+cardRect.W, cardRect.Y+cardRect.H

	clampSpan := func(center, lo, hi float32) (float32, float32, float32) {
		avail := max(hi-lo, 0)
		half := min(pointerH/2, avail/2)
		c := clampRange(center, lo+half, hi-half)
		return c, c - half, c + half
	}

	// Top-left corner start at (x0 + r, y0), going clockwise.
	p.MoveTo(x0+r, y0)

	// Top edge (possibly with DockTop tail)
	if tailSide == model.DockTop && x1-x0 > 2*r+2 {
		c, a, b := clampSpan(pointerCenter, x0+r, x1-r)
		p.LineTo(a, y0)
		tipY := y0 - pointerW
		p.CubeTo(a+(c-a)*0.45, y0, c-(c-a)*0.22, tipY, c, tipY)
		p.CubeTo(c+(b-c)*0.22, tipY, b-(b-c)*0.45, y0, b, y0)
	}
	p.LineTo(x1-r, y0)
	// Top-right corner
	p.CubeTo(x1-r*(1-k), y0, x1, y0+r*(1-k), x1, y0+r)

	// Right edge (possibly with DockRight tail)
	if tailSide == model.DockRight && y1-y0 > 2*r+2 {
		c, a, b := clampSpan(pointerCenter, y0+r, y1-r)
		p.LineTo(x1, a)
		tipX := x1 + pointerW
		p.CubeTo(x1, a+(c-a)*0.45, tipX, c-(c-a)*0.22, tipX, c)
		p.CubeTo(tipX, c+(b-c)*0.22, x1, b-(b-c)*0.45, x1, b)
	}
	p.LineTo(x1, y1-r)
	// Bottom-right corner
	p.CubeTo(x1, y1-r*(1-k), x1-r*(1-k), y1, x1-r, y1)

	// Bottom edge (possibly with DockBottom tail)
	if tailSide == model.DockBottom && x1-x0 > 2*r+2 {
		c, a, b := clampSpan(pointerCenter, x0+r, x1-r)
		p.LineTo(b, y1)
		tipY := y1 + pointerW
		p.CubeTo(b-(b-c)*0.45, y1, c+(b-c)*0.22, tipY, c, tipY)
		p.CubeTo(c-(c-a)*0.22, tipY, a+(c-a)*0.45, y1, a, y1)
	}
	p.LineTo(x0+r, y1)
	// Bottom-left corner
	p.CubeTo(x0+r*(1-k), y1, x0, y1-r*(1-k), x0, y1-r)

	// Left edge (possibly with DockLeft tail)
	if tailSide == model.DockLeft && y1-y0 > 2*r+2 {
		c, a, b := clampSpan(pointerCenter, y0+r, y1-r)
		p.LineTo(x0, b)
		tipX := x0 - pointerW
		p.CubeTo(x0, b-(b-c)*0.45, tipX, c+(b-c)*0.22, tipX, c)
		p.CubeTo(tipX, c-(c-a)*0.22, x0, a+(c-a)*0.45, x0, a)
	}
	p.LineTo(x0, y0+r)
	// Top-left corner
	p.CubeTo(x0, y0+r*(1-k), x0+r*(1-k), y0, x0+r, y0)
	p.Close()
	return &p
}

func addRoundedRect(p *ui.Path, r ui.Rect, rad, k float32) {
	rad = min(rad, min(r.W, r.H)/2)
	if rad < 0 {
		rad = 0
	}
	x0, y0 := r.X, r.Y
	x1, y1 := r.X+r.W, r.Y+r.H
	p.MoveTo(x0+rad, y0)
	p.LineTo(x1-rad, y0)
	p.CubeTo(x1-rad*(1-k), y0, x1, y0+rad*(1-k), x1, y0+rad)
	p.LineTo(x1, y1-rad)
	p.CubeTo(x1, y1-rad*(1-k), x1-rad*(1-k), y1, x1-rad, y1)
	p.LineTo(x0+rad, y1)
	p.CubeTo(x0+rad*(1-k), y1, x0, y1-rad*(1-k), x0, y1-rad)
	p.LineTo(x0, y0+rad)
	p.CubeTo(x0, y0+rad*(1-k), x0+rad*(1-k), y0, x0+rad, y0)
	p.Close()
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampRange(v, lo, hi float32) float32 {
	if hi < lo {
		return (lo + hi) / 2
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
