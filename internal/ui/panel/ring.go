package panel

import (
	"fmt"
	"math"

	"magpie-pulse/internal/icons"
	"magpie-pulse/internal/model"

	"github.com/egoist/mygo/ui"
)

// DrawUsageRing paints a Pulse usage ring inside box, including optional
// reset-clock arc, hover halo, primary & second rings, centered provider icon,
// and percentage label.
func DrawUsageRing(p *ui.Painter, box ui.Rect, slot model.RailSlot, cfg model.Config, m model.DockMetrics, hovered bool, refreshing bool, light bool) {
	if box.W <= 0 || box.H <= 0 {
		return
	}
	s := m.Scale
	if s <= 0 {
		s = 1
	}

	ringR := m.RingDiameter / 2
	beside := cfg.DockEdge.IsVertical() && cfg.ShowPercentages

	var cx, cy float32
	if beside {
		cx = box.X + ringR + m.ClockReach + m.RingLineWidth/2
		cy = box.Y + box.H/2
	} else {
		cx = box.X + box.W/2
		if cfg.ShowPercentages {
			cy = box.Y + ringR
		} else {
			cy = box.Y + box.H/2
		}
	}

	trackColor := model.DarkTrack
	if light {
		trackColor = model.LightTrack
	}

	usedFrac := 0.0
	if slot.UsedFraction != nil {
		usedFrac = *slot.UsedFraction
	}
	tint := model.UsageTintColor(usedFrac, slot.IsSpent, cfg.WarningThreshold, light)
	if !slot.IsSpent {
		if hex, ok := cfg.RingTints[slot.Account.AccountKey()]; ok {
			if custom, ok2 := model.ParseHexColor(hex); ok2 {
				tint = custom
			}
		}
	}

	// Subtle hover halo behind the ring.
	if hovered {
		haloR := ringR + m.RingLineWidth/2 + 3.5*s
		haloBox := ui.Rect{X: cx - haloR, Y: cy - haloR, W: haloR * 2, H: haloR * 2}
		p.Fill(haloBox, tint.Alpha(0.18), haloR)
		p.Stroke(haloBox, tint.Alpha(0.32), haloR, 1*s)
	}

	// Outer reset-clock thin hairline arc.
	if cfg.ShowClockArc && slot.ClockFraction != nil {
		clockR := ringR + m.RingLineWidth/2 + 2.5*s
		clockLW := 1.25 * s
		clockTrack := arcPath(cx, cy, clockR, -math.Pi/2, 2*math.Pi)
		clockTrack.Cap(ui.RoundCap)
		p.StrokePath(clockTrack, clockLW, trackColor.Alpha(0.45))
		cf := clamp01(float32(*slot.ClockFraction))
		if cf > 0.005 {
			clockArc := arcPath(cx, cy, clockR, -math.Pi/2, float64(cf)*2*math.Pi)
			clockArc.Cap(ui.RoundCap)
			p.StrokePath(clockArc, clockLW, tint.Alpha(0.65))
		}
	}

	// Primary track ring.
	fullTrack := arcPath(cx, cy, ringR, -math.Pi/2, 2*math.Pi)
	fullTrack.Cap(ui.RoundCap)
	p.StrokePath(fullTrack, m.RingLineWidth, trackColor)

	// Primary progress arc or refreshing sweep.
	if refreshing {
		p.AnimationFrame()
		phase := float64(p.Now().UnixMilli()%1000) / 1000.0
		startAngle := -math.Pi/2 + phase*2*math.Pi
		sweep := 0.32 * 2 * math.Pi
		spinArc := arcPath(cx, cy, ringR, startAngle, sweep)
		spinArc.Cap(ui.RoundCap)
		p.StrokePath(spinArc, m.RingLineWidth, tint)
	} else if slot.HasReading && slot.UsedFraction != nil {
		displayFrac := *slot.UsedFraction
		if cfg.ShowRemaining {
			displayFrac = 1.0 - displayFrac
		}
		displayFrac = max(0, min(1, displayFrac))
		if displayFrac > 0.004 {
			progArc := arcPath(cx, cy, ringR, -math.Pi/2, displayFrac*2*math.Pi)
			progArc.Cap(ui.RoundCap)
			p.StrokePath(progArc, m.RingLineWidth, tint)
		}
	}

	// Optional inner second ring.
	hasSecond := cfg.ShowSecondRing && slot.Second != nil
	if hasSecond {
		secR := m.SecondRingDiameter / 2
		secTrack := arcPath(cx, cy, secR, -math.Pi/2, 2*math.Pi)
		secTrack.Cap(ui.RoundCap)
		p.StrokePath(secTrack, m.SecondRingLineWidth, trackColor.Alpha(0.75))

		secUsed := slot.Second.UsedFraction()
		secTint := model.UsageTintColor(secUsed, slot.Second.IsSpent(), cfg.WarningThreshold, light)
		secDisplay := secUsed
		if cfg.ShowRemaining {
			secDisplay = 1.0 - secUsed
		}
		secDisplay = max(0, min(1, secDisplay))
		if secDisplay > 0.004 {
			secArc := arcPath(cx, cy, secR, -math.Pi/2, secDisplay*2*math.Pi)
			secArc.Cap(ui.RoundCap)
			p.StrokePath(secArc, m.SecondRingLineWidth, secTint)
		}
	}

	// Centered inner disc + provider SVG icon.
	innerR := ringR - m.RingLineWidth/2 - 1.5*s
	if hasSecond {
		innerR = m.SecondRingDiameter/2 - m.SecondRingLineWidth/2 - 1.25*s
	}
	if innerR < 6*s {
		innerR = 6 * s
	}
	discColor := ui.RGB(18, 20, 24)
	iconColor := ui.RGB(244, 246, 249)
	if light {
		discColor = ui.RGB(236, 237, 240)
		iconColor = ui.RGB(24, 26, 30)
	}
	if !slot.HasReading {
		iconColor = iconColor.Alpha(0.42)
	}
	discBox := ui.Rect{X: cx - innerR, Y: cy - innerR, W: innerR * 2, H: innerR * 2}
	p.Fill(discBox, discColor, innerR)

	iconSize := innerR * 1.32
	iconBox := ui.Rect{X: cx - iconSize/2, Y: cy - iconSize/2, W: iconSize, H: iconSize}
	p.Icon(icons.ForProvider(slot.Account.Provider), iconBox, iconColor)

	// Percentage figure below (or beside on DockTop/DockBottom).
	if cfg.ShowPercentages {
		label, labelColor := ringPercentLabel(slot, cfg, tint, light)
		span := ui.Span{
			Text:     label,
			Size:     m.PercentFontSize,
			Weight:   600,
			Color:    labelColor,
			Features: "tnum",
		}
		tw, th := p.MeasureText(0, span)
		var tx, ty float32
		if beside {
			tx = cx + ringR + m.ClockReach + m.RingLineWidth/2 + 4*s
			ty = cy - th/2
		} else {
			tx = cx - tw/2
			ty = cy + ringR + m.RingToTextSpacing
		}
		p.RichText(tx, ty, 0, span)
	}
}

func ringPercentLabel(slot model.RailSlot, cfg model.Config, tint ui.Color, light bool) (string, ui.Color) {
	fg := ui.RGB(238, 240, 244)
	muted := model.DarkMutedText
	if light {
		fg = ui.RGB(24, 26, 30)
		muted = model.LightMutedText
	}
	if slot.HasError {
		return "!", tint
	}
	if !slot.HasReading || slot.UsedFraction == nil {
		return "--", muted
	}
	pct := *slot.UsedFraction * 100
	if cfg.ShowRemaining {
		pct = (1.0 - *slot.UsedFraction) * 100
	}
	if slot.IsSpent {
		fg = tint
	}
	return fmt.Sprintf("%.0f%%", math.Round(pct)), fg
}

// arcPath constructs an open (or closed if >= 2*Pi) circular arc path from
// startRad clockwise by sweepRad.
func arcPath(cx, cy, r float32, startRad, sweepRad float64) *ui.Path {
	var path ui.Path
	if r <= 0 || sweepRad <= 0 {
		return &path
	}
	full := false
	if sweepRad >= 2*math.Pi-1e-4 {
		sweepRad = 2 * math.Pi
		full = true
	}
	segs := int(math.Ceil(sweepRad / (math.Pi / 2)))
	if segs < 1 {
		segs = 1
	}
	step := sweepRad / float64(segs)
	a0 := startRad
	x0 := cx + r*float32(math.Cos(a0))
	y0 := cy + r*float32(math.Sin(a0))
	path.MoveTo(x0, y0)
	for i := 0; i < segs; i++ {
		a1 := a0 + step
		k := float32((4.0 / 3.0) * math.Tan(step/4.0))
		cos0, sin0 := float32(math.Cos(a0)), float32(math.Sin(a0))
		cos1, sin1 := float32(math.Cos(a1)), float32(math.Sin(a1))
		px0, py0 := cx+r*cos0, cy+r*sin0
		px3, py3 := cx+r*cos1, cy+r*sin1
		c1x, c1y := px0-k*r*sin0, py0+k*r*cos0
		c2x, c2y := px3+k*r*sin1, py3-k*r*cos1
		path.CubeTo(c1x, c1y, c2x, c2y, px3, py3)
		a0 = a1
	}
	if full {
		path.Close()
	}
	return &path
}
