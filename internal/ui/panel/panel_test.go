package panel

import (
	"image"
	"testing"
	"time"

	"magpie-pulse/internal/model"

	"github.com/egoist/mygo/ui"
)

func hasNonZeroPixels(img *image.RGBA) bool {
	if img == nil {
		return false
	}
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 0 {
			return true
		}
	}
	return false
}

func TestDockBerthPathAllEdgesAndOpenness(t *testing.T) {
	cfg := model.DefaultConfig()
	m := model.ComputeMetrics(cfg)

	edges := []model.DockEdge{
		model.DockRight,
		model.DockLeft,
		model.DockTop,
		model.DockBottom,
		model.DockFloating,
	}

	for _, edge := range edges {
		for _, roundEnds := range []bool{false, true} {
			rect := ui.Rect{X: 0, Y: 0, W: m.Width, H: 240}
			if edge.IsVertical() {
				rect = ui.Rect{X: 0, Y: 0, W: 240, H: m.Width}
			}
			open := DockBerthPath(rect, edge, m, roundEnds, 1.0)
			if !open.Contains(rect.X+rect.W/2, rect.Y+rect.H/2) {
				t.Fatalf("edge=%s roundEnds=%v: center not inside open path", edge, roundEnds)
			}
			half := DockBerthPath(rect, edge, m, roundEnds, 0.5)
			hb := half.Bounds()
			if hb.W <= 0 || hb.H <= 0 {
				t.Fatalf("edge=%s roundEnds=%v: empty half-open bounds %+v", edge, roundEnds, hb)
			}

			closed := DockBerthPath(rect, edge, m, roundEnds, 0.0)
			cb := closed.Bounds()
			switch edge {
			case model.DockRight:
				if cb.W > m.CollapsedWidth+1 {
					t.Fatalf("DockRight collapsed width = %.2f, want <= %.2f", cb.W, m.CollapsedWidth+1)
				}
				if !closed.Contains(rect.X+rect.W-2, rect.Y+rect.H/2) {
					t.Fatalf("DockRight collapsed sliver missing flush-right center")
				}
			case model.DockLeft:
				if cb.W > m.CollapsedWidth+1 {
					t.Fatalf("DockLeft collapsed width = %.2f, want <= %.2f", cb.W, m.CollapsedWidth+1)
				}
				if !closed.Contains(rect.X+2, rect.Y+rect.H/2) {
					t.Fatalf("DockLeft collapsed sliver missing flush-left center")
				}
			case model.DockTop:
				if cb.H > m.CollapsedWidth+1 {
					t.Fatalf("DockTop collapsed height = %.2f, want <= %.2f", cb.H, m.CollapsedWidth+1)
				}
				if !closed.Contains(rect.X+rect.W/2, rect.Y+2) {
					t.Fatalf("DockTop collapsed sliver missing flush-top center")
				}
			case model.DockBottom:
				if cb.H > m.CollapsedWidth+1 {
					t.Fatalf("DockBottom collapsed height = %.2f, want <= %.2f", cb.H, m.CollapsedWidth+1)
				}
				if !closed.Contains(rect.X+rect.W/2, rect.Y+rect.H-2) {
					t.Fatalf("DockBottom collapsed sliver missing flush-bottom center")
				}
			}
		}
	}
}

func TestUsageBubblePathTails(t *testing.T) {
	card := ui.Rect{X: 40, Y: 40, W: 272, H: 220}
	pw, ph, rad := float32(18), float32(38), float32(20)

	for _, side := range []model.DockEdge{
		model.DockRight,
		model.DockLeft,
		model.DockTop,
		model.DockBottom,
		model.DockFloating,
	} {
		center := card.Y + card.H/2
		if side.IsVertical() {
			center = card.X + card.W/2
		}
		p := UsageBubblePath(card, side, center, pw, ph, rad)
		if !p.Contains(card.X+card.W/2, card.Y+card.H/2) {
			t.Fatalf("tail=%s: card center not inside path", side)
		}
		b := p.Bounds()
		switch side {
		case model.DockRight:
			if b.X+b.W < card.X+card.W+pw-1 {
				t.Fatalf("DockRight tail did not extend right: bounds=%+v", b)
			}
			if !p.Contains(card.X+card.W+pw*0.6, center) {
				t.Fatalf("DockRight tail interior missing")
			}
		case model.DockLeft:
			if b.X > card.X-pw+1 {
				t.Fatalf("DockLeft tail did not extend left: bounds=%+v", b)
			}
			if !p.Contains(card.X-pw*0.6, center) {
				t.Fatalf("DockLeft tail interior missing")
			}
		case model.DockTop:
			if b.Y > card.Y-pw+1 {
				t.Fatalf("DockTop tail did not extend up: bounds=%+v", b)
			}
			if !p.Contains(center, card.Y-pw*0.6) {
				t.Fatalf("DockTop tail interior missing")
			}
		case model.DockBottom:
			if b.Y+b.H < card.Y+card.H+pw-1 {
				t.Fatalf("DockBottom tail did not extend down: bounds=%+v", b)
			}
			if !p.Contains(center, card.Y+card.H+pw*0.6) {
				t.Fatalf("DockBottom tail interior missing")
			}
		case model.DockFloating:
			if b.W > card.W+1 || b.H > card.H+1 {
				t.Fatalf("DockFloating should not have a tail: bounds=%+v", b)
			}
		}
	}
}

func TestDrawUsageRingVariants(t *testing.T) {
	now := time.Now()
	used := 0.38
	clock := 0.62
	second := model.QuotaWindow{Name: "7 Days", Used: 65, Remaining: 35}
	slots := []model.RailSlot{
		{
			SlotID:        "antigravity/pro",
			Account:       model.AccountQuota{Provider: "antigravity", Name: "Antigravity", User: "alex@example.com"},
			Title:         "Antigravity",
			HasReading:    true,
			UsedFraction:  &used,
			ClockFraction: &clock,
			Second:        &second,
		},
		{
			SlotID:     "zcode/err",
			Account:    model.AccountQuota{Provider: "zcode", Name: "ZCode", Error: "token expired"},
			Title:      "ZCode",
			HasError:   true,
			IsSpent:    true,
			HasReading: false,
		},
		{
			SlotID:     "codex/empty",
			Account:    model.AccountQuota{Provider: "codex", Name: "Codex"},
			Title:      "Codex",
			HasReading: false,
		},
	}

	cfg := model.DefaultConfig()
	cfg.ShowClockArc = true
	cfg.ShowSecondRing = true
	cfg.RingTints["antigravity/pro"] = "#38BDF8"
	m := model.ComputeMetrics(cfg)

	for _, light := range []bool{false, true} {
		for _, showRem := range []bool{false, true} {
			cfg.ShowRemaining = showRem
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Size(240, 120).Draw(func(p *ui.Painter, r ui.Rect) {
					for i, sl := range slots {
						box := ui.Rect{X: float32(i)*72 + 8, Y: 12, W: 64, H: m.ItemHeight}
						DrawUsageRing(p, box, sl, cfg, m, i == 0, i == 2, light)
					}
				})
			}, 240, 120)
			if !hasNonZeroPixels(tt.Image()) {
				t.Fatalf("DrawUsageRing produced empty image (light=%v showRem=%v)", light, showRem)
			}
			_ = now
		}
	}
}

func makeSampleRailSlots(now time.Time) []model.RailSlot {
	accounts := []model.AccountQuota{
		{
			Provider: "antigravity",
			Name:     "Antigravity",
			Plan:     "Google AI Pro",
			User:     "dev@example.com",
			ReadAt:   now.Add(-2 * time.Minute).Format(time.RFC3339),
			Windows: []model.QuotaWindow{
				{Name: "Gemini", Pool: "Gemini", Used: 14, Remaining: 86, ResetsAt: now.Add(3 * time.Hour).Format(time.RFC3339)},
				{Name: "Claude & GPT", Pool: "Claude & GPT", Used: 42, Remaining: 58, ResetsAt: now.Add(4 * time.Hour).Format(time.RFC3339)},
			},
		},
		{
			Provider: "claude",
			Name:     "Claude",
			Plan:     "Max 5x",
			User:     "team@example.com",
			ReadAt:   now.Add(-5 * time.Minute).Format(time.RFC3339),
			Windows: []model.QuotaWindow{
				{Name: "5 Hours", Used: 82, Remaining: 18, ResetsAt: now.Add(90 * time.Minute).Format(time.RFC3339)},
				{Name: "7 Days", Used: 35, Remaining: 65, ResetsAt: now.Add(72 * time.Hour).Format(time.RFC3339)},
			},
		},
		{
			Provider: "zcode",
			Name:     "ZCode",
			Plan:     "GLM Coding Pro",
			User:     "glm@example.com",
			Error:    "API key expired",
		},
	}
	cfg := model.DefaultConfig()
	cfg.SplitPools = false
	return model.BuildRailSlots(accounts, cfg, now)
}

func TestRailViewInteractionsAndSurfaces(t *testing.T) {
	now := time.Now()
	slots := makeSampleRailSlots(now)

	var hoveredID string
	var hoveredRect ui.Rect
	leftRail := false
	clickedID := ""
	rightClicks := 0

	state := &RailState{
		Slots:      slots,
		Config:     model.DefaultConfig(),
		Expanded:   true,
		SlotBounds: map[string]ui.Rect{},
		OnHoverSlot: func(slotID string, rect ui.Rect) {
			hoveredID = slotID
			hoveredRect = rect
		},
		OnLeaveRail: func() {
			leftRail = true
		},
		OnClickSlot: func(slotID string) {
			clickedID = slotID
		},
		OnRightClick: func() {
			rightClicks++
		},
	}
	state.OnHoverSlot = func(slotID string, rect ui.Rect) {
		hoveredID = slotID
		hoveredRect = rect
		state.HoveredSlotID = slotID
	}
	state.OnLeaveRail = func() {
		leftRail = true
		state.HoveredSlotID = ""
	}

	tt := ui.NewTester(func(c *ui.Context) {
		RailView(c, state)
	}, 120, 420)

	if len(state.SlotBounds) != len(slots) {
		t.Fatalf("SlotBounds count = %d, want %d", len(state.SlotBounds), len(slots))
	}
	first := slots[0]
	fb := state.SlotBounds[first.SlotID]
	if fb.W <= 0 || fb.H <= 0 {
		t.Fatalf("invalid first slot bounds: %+v", fb)
	}

	// Hover first slot.
	tt.Move(fb.X+fb.W/2, fb.Y+fb.H/2)
	if hoveredID != first.SlotID {
		t.Fatalf("OnHoverSlot got %q, want %q", hoveredID, first.SlotID)
	}
	if hoveredRect.W <= 0 {
		t.Fatalf("OnHoverSlot rect empty: %+v", hoveredRect)
	}

	// Click second slot.
	second := slots[1]
	sb := state.SlotBounds[second.SlotID]
	tt.ClickAt(sb.X+sb.W/2, sb.Y+sb.H/2)
	if clickedID != second.SlotID {
		t.Fatalf("OnClickSlot got %q, want %q", clickedID, second.SlotID)
	}

	// Right click on rail empty area (top flare area).
	tt.RightClickAt(fb.X+fb.W/2, fb.Y-20)
	if rightClicks != 1 {
		t.Fatalf("expected OnRightClick count=1 on rail empty area, got %d", rightClicks)
	}

	// Right click on a slot should also fire OnRightClick exactly once.
	tt.RightClickAt(sb.X+sb.W/2, sb.Y+sb.H/2)
	if rightClicks != 2 {
		t.Fatalf("expected OnRightClick count=2 after slot right click, got %d", rightClicks)
	}

	// Leave rail.
	tt.Move(5, 5)
	if !leftRail {
		t.Fatalf("expected OnLeaveRail callback when moving off rail")
	}

	// Verify Dark, Light, Glass, RoundEnds, and AutoCollapse render cleanly.
	surfaces := []model.SurfaceMode{model.SurfaceDark, model.SurfaceLight, model.SurfaceGlass}
	for _, surf := range surfaces {
		for _, roundEnds := range []bool{false, true} {
			cfg := model.DefaultConfig()
			cfg.Surface = surf
			cfg.RoundEnds = roundEnds
			st := &RailState{Slots: slots, Config: cfg, Expanded: true}
			surfTest := ui.NewTester(func(c *ui.Context) {
				RailView(c, st)
			}, 120, 420)
			if !hasNonZeroPixels(surfTest.Image()) {
				t.Fatalf("RailView produced empty image for surface=%s roundEnds=%v", surf, roundEnds)
			}
		}
	}

	// AutoCollapse collapsed sliver with alert slot.
	collapsedCfg := model.DefaultConfig()
	collapsedCfg.AutoCollapse = true
	collapsedState := &RailState{Slots: slots, Config: collapsedCfg, Expanded: false}
	collapsedTest := ui.NewTester(func(c *ui.Context) {
		RailView(c, collapsedState)
	}, 120, 420)
	if !hasNonZeroPixels(collapsedTest.Image()) {
		t.Fatalf("AutoCollapse sliver produced empty image")
	}

	// Horizontal rail on DockTop and DockBottom.
	for _, edge := range []model.DockEdge{model.DockTop, model.DockBottom} {
		hCfg := model.DefaultConfig()
		hCfg.DockEdge = edge
		hState := &RailState{Slots: slots, Config: hCfg, Expanded: true}
		hTest := ui.NewTester(func(c *ui.Context) {
			RailView(c, hState)
		}, 420, 120)
		if !hasNonZeroPixels(hTest.Image()) {
			t.Fatalf("RailView on %s produced empty image", edge)
		}
		if len(hState.SlotBounds) != len(slots) {
			t.Fatalf("RailView on %s SlotBounds = %d, want %d", edge, len(hState.SlotBounds), len(slots))
		}
	}

	// Empty slots -> settings button.
	settingsOpened := false
	emptyState := &RailState{
		Config: model.DefaultConfig(),
		OnOpenSettings: func() {
			settingsOpened = true
		},
	}
	emptyTest := ui.NewTester(func(c *ui.Context) {
		RailView(c, emptyState)
	}, 120, 260)
	if err := emptyTest.Click("Settings"); err != nil {
		t.Fatalf("Click Settings in empty rail failed: %v", err)
	}
	if !settingsOpened {
		t.Fatalf("expected OnOpenSettings callback when clicking empty rail settings button")
	}
}

func TestCardViewAntigravityShowAllAndDetailed(t *testing.T) {
	now := time.Now()
	acc := model.AccountQuota{
		Provider:   "antigravity",
		Name:       "Antigravity",
		Plan:       "Google AI Pro",
		User:       "alice@example.com",
		ReadAt:     now.Add(-3 * time.Minute).Format(time.RFC3339),
		CachedOnly: true,
		Windows: []model.QuotaWindow{
			{
				Name:      "Gemini",
				Pool:      "Gemini",
				Used:      14,
				Remaining: 86,
				ResetsAt:  now.Add(4 * time.Hour).Format(time.RFC3339),
				Holds: &model.WindowHolds{
					Routed: &model.RoutedUsage{Calls: 1603, Tokens: 41_800_000, Cost: 101.50},
				},
			},
			{Name: "Claude & GPT", Pool: "Claude & GPT", Used: 45, Remaining: 55, ResetsAt: now.Add(4 * time.Hour).Format(time.RFC3339)},
			{Name: "gemini-2.5-pro", Pool: "Gemini", Family: "gemini", Used: 14, Remaining: 86},
			{Name: "gemini-2.5-flash", Pool: "Gemini", Family: "gemini", Used: 8, Remaining: 92},
			{Name: "claude-sonnet-4", Pool: "Claude & GPT", Family: "claude", Used: 45, Remaining: 55},
			{Name: "claude-opus-4", Pool: "Claude & GPT", Family: "claude", Used: 30, Remaining: 70},
			{Name: "gpt-4o", Pool: "Claude & GPT", Family: "gpt", Used: 20, Remaining: 80},
			{Name: "o3-mini", Pool: "Claude & GPT", Family: "gpt", Used: 10, Remaining: 90},
			// Duplicate to verify dedupe
			{Name: "o3-mini", Pool: "Claude & GPT", Family: "gpt", Used: 10, Remaining: 90},
		},
	}
	cfg := model.DefaultConfig()
	cfg.SplitPools = false
	cfg.BarStyle = model.BarSegmented
	cfg.ShowDetailedCard = true
	slots := model.BuildRailSlots([]model.AccountQuota{acc}, cfg, now)
	if len(slots) != 1 {
		t.Fatalf("expected 1 merged slot, got %d", len(slots))
	}

	refreshed := false
	state := &CardState{
		Slot:   &slots[0],
		Config: cfg,
		History: &model.AccountHistory{
			Provider: "antigravity",
			User:     "alice@example.com",
			Lines: []model.HistoryLine{
				{
					Name: "Claude & GPT",
					Points: []model.HistoryPoint{
						{At: now.Add(-2 * time.Hour).Format(time.RFC3339), Left: 80},
						{At: now.Add(-1 * time.Hour).Format(time.RFC3339), Left: 68},
						{At: now.Format(time.RFC3339), Left: 55},
					},
				},
			},
		},
		UsageToday: &model.UsageSummary{Input: 1_200_000, Output: 300_000, Cost: 4.25},
		Usage7d:    &model.UsageSummary{Input: 12_000_000, Output: 3_500_000, Cost: 38.10},
		Usage30d:   &model.UsageSummary{Input: 40_000_000, Output: 11_000_000, Cost: 125.90},
		OnRefresh: func() {
			refreshed = true
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		CardView(c, state)
	}, 340, 560)

	if !tt.HasText("Antigravity Usage") {
		t.Fatalf("missing header title, got texts=%v", tt.Texts())
	}
	if !tt.HasText("Google AI Pro") {
		t.Fatalf("missing plan badge, got texts=%v", tt.Texts())
	}
	if !tt.HasText("Cached") {
		t.Fatalf("missing Cached pill")
	}
	if !tt.HasText("41.8M tok · $101.50 · 1,603 calls") {
		t.Fatalf("missing routed holds line, got texts=%v", tt.Texts())
	}
	if !tt.HasText("Show all 8 model limits") {
		t.Fatalf("missing Show all toggle, got texts=%v", tt.Texts())
	}
	if tt.HasText("o3-mini") {
		t.Fatalf("o3-mini should be collapsed before toggling ShowAllModels")
	}
	if !tt.HasText("Today") || !tt.HasText("$4.25") || !tt.HasText("$125.90") {
		t.Fatalf("missing detailed Magpie footer strip, got texts=%v", tt.Texts())
	}
	if !tt.HasText("Claude & GPT History") {
		t.Fatalf("missing history sparkline label, got texts=%v", tt.Texts())
	}

	if err := tt.Click("Show all 8 model limits"); err != nil {
		t.Fatalf("click Show all toggle failed: %v", err)
	}
	if !state.ShowAllModels {
		t.Fatalf("expected ShowAllModels to be true after clicking toggle")
	}
	if !tt.HasText("Show summary only") || !tt.HasText("o3-mini") {
		t.Fatalf("expected expanded model list after toggle, got texts=%v", tt.Texts())
	}

	if err := tt.Click("Refresh"); err != nil {
		t.Fatalf("click Refresh failed: %v", err)
	}
	if !refreshed {
		t.Fatalf("expected OnRefresh callback")
	}
}

func TestCardViewZCodeErrorAndTextWindow(t *testing.T) {
	now := time.Now()
	acc := model.AccountQuota{
		Provider: "zcode",
		Name:     "ZCode",
		Plan:     "GLM Coding Pro",
		User:     "glm-user",
		Error:    "Session expired: please re-authenticate",
		Windows: []model.QuotaWindow{
			{Name: "5 Hours", Used: 25, Remaining: 75, ResetSecs: 3600},
			{Name: "API Endpoint", IsText: true, Display: "https://open.bigmodel.cn/api"},
		},
	}
	cfg := model.DefaultConfig()
	cfg.Surface = model.SurfaceLight
	cfg.ShowRemaining = true
	slots := model.BuildRailSlots([]model.AccountQuota{acc}, cfg, now)

	copied := ""
	state := &CardState{
		Slot:   &slots[0],
		Config: cfg,
		OnCopyText: func(v string) {
			copied = v
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		CardView(c, state)
	}, 340, 420)

	if !tt.HasText("ZCode Usage") {
		t.Fatalf("missing ZCode header")
	}
	if !tt.HasText("GLM Coding Pro") {
		t.Fatalf("missing ZCode plan badge")
	}
	if !tt.HasText("Session expired: please re-authenticate") {
		t.Fatalf("missing error banner text, got %v", tt.Texts())
	}
	if !tt.HasText("75% Left") {
		t.Fatalf("expected ShowRemaining percentage text '75%% Left', got %v", tt.Texts())
	}
	if !tt.HasText("https://open.bigmodel.cn/api") {
		t.Fatalf("missing text window display value")
	}

	if err := tt.Click("API Endpoint"); err != nil {
		t.Fatalf("clicking text window failed: %v", err)
	}
	if copied != "https://open.bigmodel.cn/api" {
		t.Fatalf("OnCopyText got %q, want %q", copied, "https://open.bigmodel.cn/api")
	}

	// Verify glass card renders non-empty pixels as well.
	cfg.Surface = model.SurfaceGlass
	state.Config = cfg
	glassTT := ui.NewTester(func(c *ui.Context) {
		CardView(c, state)
	}, 340, 420)
	if !hasNonZeroPixels(glassTT.Image()) {
		t.Fatalf("Glass CardView produced empty image")
	}
}

func TestCardViewOrdersShortCycleAboveLongCycle(t *testing.T) {
	now := time.Now()
	acc := model.AccountQuota{
		Provider: "antigravity",
		Name:     "Antigravity",
		User:     "cycle@example.com",
		Windows: []model.QuotaWindow{
			{Name: "Gemini · 7 days", Pool: "Gemini", Used: 36, Remaining: 64, ResetsAt: now.Add(6 * 24 * time.Hour).Format(time.RFC3339)},
			{Name: "Gemini · 5 hours", Pool: "Gemini", Used: 72, Remaining: 28, ResetsAt: now.Add(2 * time.Hour).Format(time.RFC3339)},
		},
	}
	cfg := model.DefaultConfig()
	cfg.SplitPools = true
	slots := model.BuildRailSlots([]model.AccountQuota{acc}, cfg, now)
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}

	tt := ui.NewTester(func(c *ui.Context) {
		CardView(c, &CardState{Slot: &slots[0], Config: cfg})
	}, 340, 320)

	shortBox, ok := tt.Access("Gemini · 5 hours")
	if !ok {
		t.Fatalf("missing Gemini · 5 hours in texts: %v", tt.Texts())
	}
	longBox, ok := tt.Access("Gemini · 7 days")
	if !ok {
		t.Fatalf("missing Gemini · 7 days in texts: %v", tt.Texts())
	}
	if shortBox.Bounds.Y >= longBox.Bounds.Y {
		t.Fatalf("expected short-cycle (Y=%v) above long-cycle (Y=%v)", shortBox.Bounds.Y, longBox.Bounds.Y)
	}
}
