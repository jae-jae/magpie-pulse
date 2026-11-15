package app

import (
	"fmt"
	"math"

	"magpie-pulse/internal/i18n"
	"magpie-pulse/internal/model"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// ComputeRailBounds calculates the screen-space bounds for the rail window
// within the given display work area.
func ComputeRailBounds(workArea mygo.Rectangle, cfg model.Config, slotCount int) mygo.Rectangle {
	cfg.Normalize()
	m := model.ComputeMetrics(cfg)
	isVertical := cfg.DockEdge.IsVertical()
	isDocked := cfg.DockEdge.IsDocked()
	length := int(math.Ceil(float64(m.RailLength(slotCount, isDocked, isVertical))))
	thick := int(math.Ceil(float64(m.Width)))

	w, h := thick, length
	if isVertical {
		w, h = length, thick
	}
	if workArea.Width > 0 && w > workArea.Width {
		w = workArea.Width
	}
	if workArea.Height > 0 && h > workArea.Height {
		h = workArea.Height
	}

	ratio := clampFloat64(cfg.RailRatio, 0, 1)
	var x, y int
	switch cfg.DockEdge {
	case model.DockLeft:
		x = workArea.X
		y = workArea.Y + int(math.Round(float64(max(workArea.Height-h, 0))*ratio))
	case model.DockTop:
		x = workArea.X + int(math.Round(float64(max(workArea.Width-w, 0))*ratio))
		y = workArea.Y
	case model.DockBottom:
		x = workArea.X + int(math.Round(float64(max(workArea.Width-w, 0))*ratio))
		y = workArea.Y + workArea.Height - h
	case model.DockFloating:
		if cfg.FloatX == 0 && cfg.FloatY == 0 {
			x = workArea.X + max(workArea.Width-w-24, 0)
			y = workArea.Y + max(workArea.Height-h, 0)/2
		} else {
			x = cfg.FloatX
			y = cfg.FloatY
		}
	default: // DockRight
		x = workArea.X + workArea.Width - w
		y = workArea.Y + int(math.Round(float64(max(workArea.Height-h, 0))*ratio))
	}

	maxX := workArea.X + max(workArea.Width-w, 0)
	maxY := workArea.Y + max(workArea.Height-h, 0)
	x = clampInt(x, workArea.X, maxX)
	y = clampInt(y, workArea.Y, maxY)

	return mygo.Rectangle{X: x, Y: y, Width: w, Height: h}
}

// ComputeSliverInputRegion returns the clickable hit region (in rail window coordinates)
// when AutoCollapse is active and the rail is collapsed. Returns nil when the full
// window should accept input.
func ComputeSliverInputRegion(railW, railH int, cfg model.Config, expanded bool) []mygo.Rectangle {
	if !cfg.AutoCollapse || !cfg.DockEdge.IsDocked() || expanded || railW <= 0 || railH <= 0 {
		return nil
	}
	m := model.ComputeMetrics(cfg)
	sliver := max(int(math.Ceil(float64(m.CollapsedWidth)))+6, 12)
	switch cfg.DockEdge {
	case model.DockRight:
		sw := min(sliver, railW)
		return []mygo.Rectangle{{X: railW - sw, Y: 0, Width: sw, Height: railH}}
	case model.DockLeft:
		sw := min(sliver, railW)
		return []mygo.Rectangle{{X: 0, Y: 0, Width: sw, Height: railH}}
	case model.DockTop:
		sh := min(sliver, railH)
		return []mygo.Rectangle{{X: 0, Y: 0, Width: railW, Height: sh}}
	case model.DockBottom:
		sh := min(sliver, railH)
		return []mygo.Rectangle{{X: 0, Y: railH - sh, Width: railW, Height: sh}}
	default: // DockFloating
		sw := min(max(sliver, 20), railW)
		sh := min(max(sliver, 20), railH)
		return []mygo.Rectangle{{
			X:      (railW - sw) / 2,
			Y:      (railH - sh) / 2,
			Width:  sw,
			Height: sh,
		}}
	}
}

// CardPlacementForEdge maps a dock edge to the flyout placement side.
func CardPlacementForEdge(edge model.DockEdge) mygo.Placement {
	switch edge {
	case model.DockLeft:
		return mygo.PlacementRight
	case model.DockTop:
		return mygo.PlacementBottom
	case model.DockBottom:
		return mygo.PlacementTop
	default:
		return mygo.PlacementLeft
	}
}

// ComputeCardPointerCenter returns the pointer tail offset in CardView local coordinates
// so the bubble tail aligns with the hovered slot center.
func ComputeCardPointerCenter(railBounds mygo.Rectangle, slotRect ui.Rect, cardBounds mygo.Rectangle, edge model.DockEdge) float32 {
	if edge.IsVertical() {
		slotCenterX := float32(railBounds.X) + slotRect.X + slotRect.W/2
		return slotCenterX - float32(cardBounds.X)
	}
	slotCenterY := float32(railBounds.Y) + slotRect.Y + slotRect.H/2
	return slotCenterY - float32(cardBounds.Y)
}

// SlotAnchorRect converts a slot's ui.Rect into a valid mygo.Rectangle anchor
// relative to the rail window's content area.
func SlotAnchorRect(slotRect ui.Rect, railW, railH int) mygo.Rectangle {
	x := int(math.Floor(float64(slotRect.X)))
	y := int(math.Floor(float64(slotRect.Y)))
	w := int(math.Ceil(float64(slotRect.W)))
	h := int(math.Ceil(float64(slotRect.H)))
	if w <= 0 {
		w = max(railW, 1)
	}
	if h <= 0 {
		h = max(railH, 1)
	}
	return mygo.Rectangle{X: x, Y: y, Width: w, Height: h}
}

// UpdateDockedRatio updates cfg.RailRatio from the rail window's moved position
// along its docked axis.
func UpdateDockedRatio(workArea, winBounds mygo.Rectangle, edge model.DockEdge) float64 {
	switch edge {
	case model.DockTop, model.DockBottom:
		avail := workArea.Width - winBounds.Width
		if avail <= 0 {
			return 0.5
		}
		return clampFloat64(float64(winBounds.X-workArea.X)/float64(avail), 0, 1)
	default:
		avail := workArea.Height - winBounds.Height
		if avail <= 0 {
			return 0.5
		}
		return clampFloat64(float64(winBounds.Y-workArea.Y)/float64(avail), 0, 1)
	}
}

// UpdateDockedRatioForTransition computes cfg.RailRatio from winBounds centered
// onto the target docked window size so orientation or padding changes stay aligned.
func UpdateDockedRatioForTransition(workArea, winBounds mygo.Rectangle, targetEdge model.DockEdge, cfg model.Config, slotCount int) float64 {
	targetCfg := cfg
	targetCfg.DockEdge = targetEdge
	targetBounds := ComputeRailBounds(workArea, targetCfg, slotCount)
	switch targetEdge {
	case model.DockTop, model.DockBottom:
		avail := workArea.Width - targetBounds.Width
		if avail <= 0 {
			return 0.5
		}
		centerX := float64(winBounds.X) + float64(winBounds.Width)/2.0
		targetX := centerX - float64(targetBounds.Width)/2.0
		return clampFloat64((targetX-float64(workArea.X))/float64(avail), 0, 1)
	default:
		avail := workArea.Height - targetBounds.Height
		if avail <= 0 {
			return 0.5
		}
		centerY := float64(winBounds.Y) + float64(winBounds.Height)/2.0
		targetY := centerY - float64(targetBounds.Height)/2.0
		return clampFloat64((targetY-float64(workArea.Y))/float64(avail), 0, 1)
	}
}

// ComputeFloatingPosition returns the clamped (FloatX, FloatY) origin for DockFloating
// centered on winBounds so transitioning from a docked edge does not jump.
func ComputeFloatingPosition(workArea, winBounds mygo.Rectangle, cfg model.Config, slotCount int) (int, int) {
	floatCfg := cfg
	floatCfg.DockEdge = model.DockFloating
	floatCfg.FloatX = 1
	floatCfg.FloatY = 1
	targetBounds := ComputeRailBounds(workArea, floatCfg, slotCount)
	cx := winBounds.X + winBounds.Width/2
	cy := winBounds.Y + winBounds.Height/2
	x := cx - targetBounds.Width/2
	y := cy - targetBounds.Height/2
	maxX := workArea.X + max(workArea.Width-targetBounds.Width, 0)
	maxY := workArea.Y + max(workArea.Height-targetBounds.Height, 0)
	x = clampInt(x, workArea.X, maxX)
	y = clampInt(y, workArea.Y, maxY)
	if x == 0 && y == 0 {
		y = 1
	}
	return x, y
}

// DetectSnapEdge decides whether a dragged rail window should snap to a screen edge
// or become DockFloating. When already docked, a slightly larger detach threshold
// avoids accidental unsnapping while sliding along the same edge.
func DetectSnapEdge(workArea, winBounds mygo.Rectangle, cursor mygo.Point, currentEdge model.DockEdge) (model.DockEdge, bool) {
	const (
		snapDist       = 24
		detachDist     = 32
		cursorSnapDist = 10
	)
	if workArea.Width <= 0 || workArea.Height <= 0 {
		return currentEdge, currentEdge.IsDocked()
	}

	leftGap := max(winBounds.X-workArea.X, 0)
	rightGap := max((workArea.X+workArea.Width)-(winBounds.X+winBounds.Width), 0)
	topGap := max(winBounds.Y-workArea.Y, 0)
	bottomGap := max((workArea.Y+workArea.Height)-(winBounds.Y+winBounds.Height), 0)

	gapFor := func(e model.DockEdge) int {
		switch e {
		case model.DockLeft:
			return leftGap
		case model.DockRight:
			return rightGap
		case model.DockTop:
			return topGap
		case model.DockBottom:
			return bottomGap
		default:
			return math.MaxInt32
		}
	}

	if currentEdge.IsDocked() && gapFor(currentEdge) <= detachDist {
		return currentEdge, true
	}

	cursorNear := func(e model.DockEdge) bool {
		switch e {
		case model.DockLeft:
			return cursor.X <= workArea.X+cursorSnapDist
		case model.DockRight:
			return cursor.X >= workArea.X+workArea.Width-cursorSnapDist
		case model.DockTop:
			return cursor.Y <= workArea.Y+cursorSnapDist
		case model.DockBottom:
			return cursor.Y >= workArea.Y+workArea.Height-cursorSnapDist
		default:
			return false
		}
	}

	candidates := []model.DockEdge{model.DockRight, model.DockLeft, model.DockTop, model.DockBottom}
	var best model.DockEdge
	bestScore := math.MaxInt32
	found := false
	for _, e := range candidates {
		g := gapFor(e)
		near := cursorNear(e)
		if g > snapDist && !near {
			continue
		}
		score := g
		if near && score > 0 {
			score = max(score-snapDist, 0)
		}
		if !found || score < bestScore {
			best = e
			bestScore = score
			found = true
		}
	}
	if found {
		return best, true
	}
	return model.DockFloating, false
}

// IsCursorOnDockEdge reports whether cursor is touching the screen/workArea edge
// corresponding to edge, excluding a small corner guard to avoid hot-corner conflicts.
func IsCursorOnDockEdge(workArea, displayBounds mygo.Rectangle, cursor mygo.Point, edge model.DockEdge) bool {
	const (
		edgeBand    = 6
		cornerGuard = 16
	)
	if !edge.IsDocked() {
		return false
	}
	if displayBounds.Width <= 0 || displayBounds.Height <= 0 {
		displayBounds = workArea
	}
	if workArea.Width <= 0 || workArea.Height <= 0 {
		workArea = displayBounds
	}
	switch edge {
	case model.DockRight:
		yOK := cursor.Y >= displayBounds.Y+cornerGuard && cursor.Y <= displayBounds.Y+displayBounds.Height-cornerGuard
		innerEdge := min(workArea.X+workArea.Width, displayBounds.X+displayBounds.Width)
		outerEdge := max(workArea.X+workArea.Width, displayBounds.X+displayBounds.Width)
		return yOK && cursor.X >= innerEdge-edgeBand && cursor.X <= outerEdge+2
	case model.DockLeft:
		yOK := cursor.Y >= displayBounds.Y+cornerGuard && cursor.Y <= displayBounds.Y+displayBounds.Height-cornerGuard
		outerEdge := min(workArea.X, displayBounds.X)
		innerEdge := max(workArea.X, displayBounds.X)
		return yOK && cursor.X >= outerEdge-2 && cursor.X <= innerEdge+edgeBand
	case model.DockTop:
		xOK := cursor.X >= displayBounds.X+cornerGuard && cursor.X <= displayBounds.X+displayBounds.Width-cornerGuard
		outerEdge := min(workArea.Y, displayBounds.Y)
		innerEdge := max(workArea.Y, displayBounds.Y)
		return xOK && cursor.Y >= outerEdge-2 && cursor.Y <= innerEdge+edgeBand
	case model.DockBottom:
		xOK := cursor.X >= displayBounds.X+cornerGuard && cursor.X <= displayBounds.X+displayBounds.Width-cornerGuard
		innerEdge := min(workArea.Y+workArea.Height, displayBounds.Y+displayBounds.Height)
		outerEdge := max(workArea.Y+workArea.Height, displayBounds.Y+displayBounds.Height)
		return xOK && cursor.Y >= innerEdge-edgeBand && cursor.Y <= outerEdge+2
	default:
		return false
	}
}

// WorstAlertSummary returns whether any visible slot has reached the warning or spent threshold,
// plus a short status tooltip for the tray icon.
func WorstAlertSummary(slots []model.RailSlot, cfg model.Config, snap model.Snapshot) (bool, string) {
	return WorstAlertSummaryLocale(i18n.LocaleEn, slots, cfg, snap)
}

// WorstAlertSummaryLocale returns whether any visible slot has reached the warning or spent threshold,
// plus a localized status tooltip for the tray icon.
func WorstAlertSummaryLocale(locale string, slots []model.RailSlot, cfg model.Config, snap model.Snapshot) (bool, string) {
	msg := i18n.For(cfg.Language, locale)
	if !snap.Connected {
		if snap.ConnectionErr != "" {
			return false, msg.TooltipOffline
		}
		return false, msg.AppName
	}
	thresh := float64(cfg.WarningThreshold) / 100.0
	alert := false
	maxPct := 0.0
	maxName := ""
	for _, s := range slots {
		if s.IsSpent {
			alert = true
			maxPct = 100
			maxName = s.Title
			break
		}
		if s.UsedFraction != nil && *s.UsedFraction >= thresh {
			alert = true
		}
		if s.UsedFraction != nil && *s.UsedFraction*100 > maxPct {
			maxPct = *s.UsedFraction * 100
			maxName = s.Title
		}
	}
	if maxName != "" {
		return alert, fmt.Sprintf("%s · %s %.0f%%", msg.AppName, maxName, maxPct)
	}
	return alert, msg.FormatTooltipAccounts(len(snap.Accounts))
}

// PointInRect reports whether p falls within r expanded by pad DIPs on all sides.
func PointInRect(p mygo.Point, r mygo.Rectangle, pad int) bool {
	if r.Width <= 0 || r.Height <= 0 {
		return false
	}
	return p.X >= r.X-pad && p.X < r.X+r.Width+pad && p.Y >= r.Y-pad && p.Y < r.Y+r.Height+pad
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func findSlotByID(slots []model.RailSlot, slotID string) *model.RailSlot {
	for i := range slots {
		if slots[i].SlotID == slotID {
			cp := slots[i]
			return &cp
		}
	}
	return nil
}
