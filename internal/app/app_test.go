package app

import (
	"testing"

	"magpie-pulse/internal/model"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestComputeRailBoundsAllEdges(t *testing.T) {
	wa := mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 875}

	cfg := model.DefaultConfig()
	cfg.RailRatio = 0.5

	cfg.DockEdge = model.DockRight
	right := ComputeRailBounds(wa, cfg, 4)
	if right.X+right.Width != wa.X+wa.Width {
		t.Fatalf("DockRight X+W=%d, want %d", right.X+right.Width, wa.X+wa.Width)
	}
	if right.Width <= 0 || right.Height <= 0 {
		t.Fatalf("invalid DockRight bounds: %+v", right)
	}

	cfg.DockEdge = model.DockLeft
	left := ComputeRailBounds(wa, cfg, 4)
	if left.X != wa.X {
		t.Fatalf("DockLeft X=%d, want %d", left.X, wa.X)
	}

	cfg.DockEdge = model.DockTop
	top := ComputeRailBounds(wa, cfg, 4)
	if top.Y != wa.Y || top.Width <= top.Height {
		t.Fatalf("unexpected DockTop bounds: %+v", top)
	}

	cfg.DockEdge = model.DockBottom
	bot := ComputeRailBounds(wa, cfg, 4)
	if bot.Y+bot.Height != wa.Y+wa.Height || bot.Width <= bot.Height {
		t.Fatalf("unexpected DockBottom bounds: %+v", bot)
	}

	cfg.DockEdge = model.DockFloating
	cfg.FloatX = 9999
	cfg.FloatY = 9999
	flt := ComputeRailBounds(wa, cfg, 3)
	if flt.X+flt.Width > wa.X+wa.Width || flt.Y+flt.Height > wa.Y+wa.Height {
		t.Fatalf("DockFloating not clamped to work area: %+v", flt)
	}
}

func TestComputeSliverInputRegion(t *testing.T) {
	cfg := model.DefaultConfig()
	cfg.AutoCollapse = false
	if got := ComputeSliverInputRegion(64, 300, cfg, false); got != nil {
		t.Fatalf("expected nil input region when AutoCollapse=false, got %+v", got)
	}

	cfg.AutoCollapse = true
	if got := ComputeSliverInputRegion(64, 300, cfg, true); got != nil {
		t.Fatalf("expected nil input region when expanded, got %+v", got)
	}

	cfg.DockEdge = model.DockRight
	reg := ComputeSliverInputRegion(64, 300, cfg, false)
	if len(reg) != 1 || reg[0].X+reg[0].Width != 64 || reg[0].Height != 300 {
		t.Fatalf("unexpected DockRight sliver region: %+v", reg)
	}

	cfg.DockEdge = model.DockLeft
	reg = ComputeSliverInputRegion(64, 300, cfg, false)
	if len(reg) != 1 || reg[0].X != 0 || reg[0].Width <= 0 {
		t.Fatalf("unexpected DockLeft sliver region: %+v", reg)
	}

	cfg.DockEdge = model.DockBottom
	reg = ComputeSliverInputRegion(300, 64, cfg, false)
	if len(reg) != 1 || reg[0].Y+reg[0].Height != 64 || reg[0].Width != 300 {
		t.Fatalf("unexpected DockBottom sliver region: %+v", reg)
	}

	cfg.DockEdge = model.DockFloating
	if got := ComputeSliverInputRegion(64, 300, cfg, false); got != nil {
		t.Fatalf("expected nil input region when DockFloating, got %+v", got)
	}
}

func TestCardPlacementAndPointerCenter(t *testing.T) {
	if got := CardPlacementForEdge(model.DockRight); got != mygo.PlacementLeft {
		t.Fatalf("DockRight placement=%q, want left", got)
	}
	if got := CardPlacementForEdge(model.DockLeft); got != mygo.PlacementRight {
		t.Fatalf("DockLeft placement=%q, want right", got)
	}
	if got := CardPlacementForEdge(model.DockTop); got != mygo.PlacementBottom {
		t.Fatalf("DockTop placement=%q, want bottom", got)
	}
	if got := CardPlacementForEdge(model.DockBottom); got != mygo.PlacementTop {
		t.Fatalf("DockBottom placement=%q, want top", got)
	}

	railBounds := mygo.Rectangle{X: 1376, Y: 200, Width: 64, Height: 300}
	slotRect := ui.Rect{X: 0, Y: 100, W: 64, H: 60}
	cardBounds := mygo.Rectangle{X: 1080, Y: 180, Width: 292, Height: 240}
	ptrY := ComputeCardPointerCenter(railBounds, slotRect, cardBounds, model.DockRight)
	// slot center Y = 200 + 130 = 330; card Y = 180 => ptrY = 150
	if ptrY != 150 {
		t.Fatalf("ptrY=%v, want 150", ptrY)
	}

	topBounds := mygo.Rectangle{X: 500, Y: 25, Width: 300, Height: 64}
	vSlot := ui.Rect{X: 120, Y: 0, W: 40, H: 64}
	vCard := mygo.Rectangle{X: 480, Y: 95, Width: 272, Height: 220}
	ptrX := ComputeCardPointerCenter(topBounds, vSlot, vCard, model.DockTop)
	// slot center X = 500 + 140 = 640; card X = 480 => ptrX = 160
	if ptrX != 160 {
		t.Fatalf("ptrX=%v, want 160", ptrX)
	}
}

func TestUpdateDockedRatioAndAlertSummary(t *testing.T) {
	wa := mygo.Rectangle{X: 0, Y: 25, Width: 1000, Height: 800}
	win := mygo.Rectangle{X: 936, Y: 225, Width: 64, Height: 400}
	ratio := UpdateDockedRatio(wa, win, model.DockRight)
	if ratio != 0.5 {
		t.Fatalf("ratio=%v, want 0.5", ratio)
	}

	cfg := model.DefaultConfig()
	cfg.WarningThreshold = 75
	used80 := 0.80
	slots := []model.RailSlot{
		{SlotID: "claude/a", Title: "Claude", UsedFraction: &used80},
	}
	snap := model.Snapshot{Connected: true, Accounts: make([]model.AccountQuota, 1)}
	alert, tip := WorstAlertSummary(slots, cfg, snap)
	if !alert {
		t.Fatalf("expected alert=true at 80%%")
	}
	if tip != "Magpie Pulse · Claude 80%" {
		t.Fatalf("unexpected tip: %q", tip)
	}
}

func TestDetectSnapEdgeAndFloatingDetach(t *testing.T) {
	wa := mygo.Rectangle{X: 0, Y: 38, Width: 1512, Height: 944}

	// Floating window in center stays floating.
	mid := mygo.Rectangle{X: 700, Y: 400, Width: 64, Height: 240}
	if edge, snapped := DetectSnapEdge(wa, mid, mygo.Point{X: 730, Y: 420}, model.DockFloating); snapped || edge != model.DockFloating {
		t.Fatalf("center window got (%q, %v), want (floating, false)", edge, snapped)
	}

	// Floating window dragged near right edge snaps to DockRight.
	nearRight := mygo.Rectangle{X: 1512 - 64 - 18, Y: 360, Width: 64, Height: 240}
	if edge, snapped := DetectSnapEdge(wa, nearRight, mygo.Point{X: 1470, Y: 380}, model.DockFloating); !snapped || edge != model.DockRight {
		t.Fatalf("near-right window got (%q, %v), want (right, true)", edge, snapped)
	}

	// Floating window dragged near left, top, bottom edges snaps accordingly.
	nearLeft := mygo.Rectangle{X: 12, Y: 360, Width: 64, Height: 240}
	if edge, snapped := DetectSnapEdge(wa, nearLeft, mygo.Point{X: 30, Y: 380}, model.DockFloating); !snapped || edge != model.DockLeft {
		t.Fatalf("near-left window got (%q, %v), want (left, true)", edge, snapped)
	}
	nearTop := mygo.Rectangle{X: 600, Y: 50, Width: 64, Height: 240}
	if edge, snapped := DetectSnapEdge(wa, nearTop, mygo.Point{X: 630, Y: 55}, model.DockFloating); !snapped || edge != model.DockTop {
		t.Fatalf("near-top window got (%q, %v), want (top, true)", edge, snapped)
	}
	nearBottom := mygo.Rectangle{X: 600, Y: 38 + 944 - 240 - 15, Width: 64, Height: 240}
	if edge, snapped := DetectSnapEdge(wa, nearBottom, mygo.Point{X: 630, Y: 950}, model.DockFloating); !snapped || edge != model.DockBottom {
		t.Fatalf("near-bottom window got (%q, %v), want (bottom, true)", edge, snapped)
	}

	// DockRight slid all the way to the top-right corner stays DockRight.
	topRight := mygo.Rectangle{X: 1512 - 64, Y: 38, Width: 64, Height: 280}
	if edge, snapped := DetectSnapEdge(wa, topRight, mygo.Point{X: 1480, Y: 45}, model.DockRight); !snapped || edge != model.DockRight {
		t.Fatalf("top-right slide got (%q, %v), want (right, true)", edge, snapped)
	}

	// DockRight dragged away from edge (> 32px) detaches into DockFloating.
	detached := mygo.Rectangle{X: 1512 - 64 - 45, Y: 320, Width: 64, Height: 280}
	if edge, snapped := DetectSnapEdge(wa, detached, mygo.Point{X: 1430, Y: 340}, model.DockRight); snapped || edge != model.DockFloating {
		t.Fatalf("detached right rail got (%q, %v), want (floating, false)", edge, snapped)
	}

	// DockRight dragged straight across to Left edge snaps to DockLeft.
	crossedLeft := mygo.Rectangle{X: 4, Y: 320, Width: 64, Height: 280}
	if edge, snapped := DetectSnapEdge(wa, crossedLeft, mygo.Point{X: 20, Y: 340}, model.DockRight); !snapped || edge != model.DockLeft {
		t.Fatalf("crossed-left rail got (%q, %v), want (left, true)", edge, snapped)
	}
}

func TestComputeFloatingPositionAndTransitionRatio(t *testing.T) {
	wa := mygo.Rectangle{X: 0, Y: 38, Width: 1512, Height: 944}
	cfg := model.DefaultConfig()

	// Horizontal DockTop dragged into middle -> floating centered on dragged rect.
	draggedTop := mygo.Rectangle{X: 500, Y: 300, Width: 280, Height: 64}
	fx, fy := ComputeFloatingPosition(wa, draggedTop, cfg, 3)
	if fx < 580 || fx > 640 || fy < 180 || fy > 280 {
		t.Fatalf("unexpected centered floating origin (%d, %d)", fx, fy)
	}

	// Vertical floating dragged to center-top -> DockTop ratio ~ 0.5.
	fltCenter := mygo.Rectangle{X: 724, Y: 42, Width: 64, Height: 240}
	ratio := UpdateDockedRatioForTransition(wa, fltCenter, model.DockTop, cfg, 3)
	if ratio < 0.45 || ratio > 0.55 {
		t.Fatalf("transition ratio=%v, want ~0.5", ratio)
	}
}

func TestIsCursorOnDockEdge(t *testing.T) {
	display := mygo.Rectangle{X: 0, Y: 0, Width: 1512, Height: 982}
	wa := mygo.Rectangle{X: 0, Y: 38, Width: 1512, Height: 910} // Dock at bottom (34px) + menu bar (38px)

	if !IsCursorOnDockEdge(wa, display, mygo.Point{X: 1511, Y: 200}, model.DockRight) {
		t.Fatalf("expected right edge hit outside rail Y bounds")
	}
	if IsCursorOnDockEdge(wa, display, mygo.Point{X: 1511, Y: 5}, model.DockRight) {
		t.Fatalf("expected top-right corner guard to ignore cursor")
	}
	if IsCursorOnDockEdge(wa, display, mygo.Point{X: 1480, Y: 200}, model.DockRight) {
		t.Fatalf("expected cursor 32px from right edge not to trigger")
	}
	if !IsCursorOnDockEdge(wa, display, mygo.Point{X: 2, Y: 500}, model.DockLeft) {
		t.Fatalf("expected left edge hit")
	}
	if !IsCursorOnDockEdge(wa, display, mygo.Point{X: 700, Y: 1}, model.DockTop) {
		t.Fatalf("expected top screen edge hit through menu bar")
	}
	if !IsCursorOnDockEdge(wa, display, mygo.Point{X: 700, Y: 980}, model.DockBottom) {
		t.Fatalf("expected bottom screen edge hit even when macOS Dock is at bottom")
	}
	if IsCursorOnDockEdge(wa, display, mygo.Point{X: 1511, Y: 200}, model.DockFloating) {
		t.Fatalf("expected DockFloating not to match edge")
	}
}

func TestWorstAlertSummaryLocale(t *testing.T) {
	cfg := model.DefaultConfig()
	offlineSnap := model.Snapshot{Connected: false, ConnectionErr: "dial failed"}
	if _, tip := WorstAlertSummaryLocale("en", nil, cfg, offlineSnap); tip != "Magpie Pulse · Offline" {
		t.Fatalf("en offline tip = %q", tip)
	}
	if _, tip := WorstAlertSummaryLocale("zh-Hans", nil, cfg, offlineSnap); tip != "Magpie Pulse · 离线" {
		t.Fatalf("zh offline tip = %q", tip)
	}

	accSnap := model.Snapshot{Connected: true, Accounts: make([]model.AccountQuota, 2)}
	if _, tip := WorstAlertSummaryLocale("en", nil, cfg, accSnap); tip != "Magpie Pulse · 2 accounts" {
		t.Fatalf("en accounts tip = %q", tip)
	}
	if _, tip := WorstAlertSummaryLocale("zh-Hans", nil, cfg, accSnap); tip != "Magpie Pulse · 2 个账号" {
		t.Fatalf("zh accounts tip = %q", tip)
	}
}
