package app

import (
	"fmt"
	"math"

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
	if !cfg.AutoCollapse || expanded || railW <= 0 || railH <= 0 {
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

// WorstAlertSummary returns whether any visible slot has reached the warning or spent threshold,
// plus a short status tooltip for the tray icon.
func WorstAlertSummary(slots []model.RailSlot, cfg model.Config, snap model.Snapshot) (bool, string) {
	if !snap.Connected {
		if snap.ConnectionErr != "" {
			return false, "Magpie Pulse · Offline"
		}
		return false, "Magpie Pulse"
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
		return alert, fmt.Sprintf("Magpie Pulse · %s %.0f%%", maxName, maxPct)
	}
	return alert, fmt.Sprintf("Magpie Pulse · %d accounts", len(snap.Accounts))
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
