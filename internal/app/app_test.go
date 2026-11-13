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
