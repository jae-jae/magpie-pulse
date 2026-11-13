package app

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"magpie-pulse/internal/magpie"
	"magpie-pulse/internal/model"
	"magpie-pulse/internal/ui/panel"
	"magpie-pulse/internal/ui/settings"

	"github.com/egoist/mygo/ui"
)

// TestEndToEndFlow exercises the full Magpie -> Service -> Rail -> Card -> Settings pipeline
// through ui.NewTester with real rendering.
func TestEndToEndFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name":    "magpie",
				"version": "0.1.3078",
				"models":  30,
				"window":  true,
			})
		case "/v1/magpie/quotas":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"provider": "zcode",
						"name":     "ZCode",
						"plan":     "GLM Coding Pro",
						"user":     "ZCode",
						"windows":  []any{},
						"error":    "API KEY missing — sign in again",
					},
					{
						"provider": "antigravity",
						"name":     "Antigravity",
						"plan":     "Google AI Pro",
						"user":     "alice@example.com",
						"readAt":   time.Now().Add(-3 * time.Minute).Format(time.RFC3339),
						"windows": []map[string]any{
							{
								"name":     "Gemini · 5 hours",
								"used":     60,
								"resetsAt": time.Now().Add(2 * time.Hour).Format(time.RFC3339),
								"pool":     "Gemini",
								"holds": map[string]any{
									"tokens": 9302853,
									"cost":   28.73,
									"priced": true,
									"routed": map[string]any{
										"calls":     363,
										"tokens":    3995364,
										"cacheRead": 62751283,
										"cost":      12.34,
									},
								},
							},
							{
								"name":     "Gemini · 7 days",
								"used":     28,
								"resetsAt": time.Now().Add(5 * 24 * time.Hour).Format(time.RFC3339),
								"pool":     "Gemini",
							},
							{
								"name":     "Claude & GPT · 7 days",
								"used":     12,
								"resetsAt": time.Now().Add(4 * 24 * time.Hour).Format(time.RFC3339),
								"pool":     "Claude & GPT",
							},
						},
					},
				},
			})
		case "/v1/magpie/quotas/history":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"provider": "antigravity",
						"user":     "alice@example.com",
						"lines": []map[string]any{
							{
								"name": "Gemini · 5 hours",
								"points": []map[string]any{
									{"at": "2026-11-12T12:00:00Z", "left": 92},
									{"at": "2026-11-12T18:00:00Z", "left": 74},
									{"at": "2026-11-13T00:00:00Z", "left": 55},
									{"at": "2026-11-13T06:00:00Z", "left": 40},
								},
							},
						},
					},
				},
			})
		case "/v1/magpie/usage":
			period := r.URL.Query().Get("period")
			switch period {
			case "1d":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"period": "1d", "calls": 180, "input": 1200000, "output": 450000, "cost": 14.20,
				})
			case "7d":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"period": "7d", "calls": 1120, "input": 9500000, "output": 2100000, "cost": 88.45,
				})
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{
					"period": "30d", "calls": 4800, "input": 38000000, "output": 8200000, "cost": 312.80,
				})
			}
		case "/v1/magpie/statusline":
			_, _ = w.Write([]byte("antigravity: 40% left"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := model.DefaultConfig()
	cfg.MagpieURL = srv.URL
	cfg.SplitPools = true
	cfg.ShowDetailedCard = true

	client := magpie.NewClient(srv.URL)
	snap := client.FetchSnapshot(context.Background(), false, t.TempDir())
	if !snap.Connected || len(snap.Accounts) != 2 {
		t.Fatalf("unexpected snapshot: connected=%v accounts=%d err=%q", snap.Connected, len(snap.Accounts), snap.ConnectionErr)
	}

	slots := model.BuildRailSlots(snap.Accounts, cfg, time.Now())
	if len(slots) != 3 {
		t.Fatalf("expected 3 slots (zcode + 2 antigravity pools), got %d", len(slots))
	}

	m := model.ComputeMetrics(cfg)
	railW := int(math.Ceil(float64(m.Width)))
	railH := int(math.Ceil(float64(m.RailLength(len(slots), true, false))))

	var hoveredSlot string
	var clickedSlot string
	railState := &panel.RailState{
		Slots:    slots,
		Config:   cfg,
		Expanded: true,
		OnHoverSlot: func(slotID string, rect ui.Rect) {
			hoveredSlot = slotID
		},
		OnClickSlot: func(slotID string) {
			clickedSlot = slotID
		},
	}

	rt := ui.NewTester(func(c *ui.Context) {
		panel.RailView(c, railState)
	}, railW, railH)
	rt.SetScale(2)
	rt.Frame()

	if _, ok := rt.Access("Antigravity · Gemini"); !ok {
		t.Fatalf("missing Antigravity · Gemini slot in accessibility tree: %v", rt.Texts())
	}
	if _, ok := rt.Access("ZCode"); !ok {
		t.Fatalf("missing ZCode slot in accessibility tree: %v", rt.Texts())
	}

	if err := rt.Click("Antigravity · Gemini"); err != nil {
		t.Fatalf("click Antigravity Gemini slot: %v", err)
	}
	if clickedSlot == "" || hoveredSlot == "" {
		t.Fatalf("expected hover (%q) and click (%q) callbacks to fire", hoveredSlot, clickedSlot)
	}

	slot := findSlotByID(slots, clickedSlot)
	if slot == nil {
		t.Fatalf("slot %q not found", clickedSlot)
	}
	hist := snap.Histories["antigravity/alice@example.com"]
	cardState := &panel.CardState{
		Slot:          slot,
		History:       &hist,
		UsageToday:    snap.UsageToday,
		Usage7d:       snap.Usage7d,
		Usage30d:      snap.Usage30d,
		Config:        cfg,
		PointerCenter: 130,
	}
	cardW := int(math.Ceil(float64(m.CardWidth + m.PointerWidth)))
	ct := ui.NewTester(func(c *ui.Context) {
		panel.CardView(c, cardState)
	}, cardW, 380)
	ct.SetScale(2)
	ct.Frame()

	if !ct.HasText("Antigravity Usage") || !ct.HasText("60% Used") || !ct.HasText("Today") || !ct.HasText("30 days") {
		t.Fatalf("card missing expected content: %v", ct.Texts())
	}

	st := &settings.SettingsState{
		ActiveTab: settings.TabGeneral,
		Config:    cfg,
		Snapshot:  snap,
	}
	st.OnTestConnection = func(rawURL string) {
		info, err := client.FetchInfo(context.Background())
		if err != nil {
			st.TestOK = false
			st.TestStatus = err.Error()
			return
		}
		st.TestOK = true
		st.TestStatus = "Connected · v" + info.Version
	}

	sett := ui.NewTester(func(c *ui.Context) {
		settings.SettingsView(c, st)
	}, 680, 600)
	sett.SetScale(2)
	sett.Frame()

	if err := sett.Click("测试连接"); err != nil {
		t.Fatalf("click 测试连接: %v", err)
	}
	if !st.TestOK {
		t.Fatalf("expected TestOK=true, got status=%q", st.TestStatus)
	}
	if err := sett.Click("账号总览"); err != nil {
		t.Fatalf("switch to accounts tab: %v", err)
	}
	if !sett.HasText("Antigravity") || !sett.HasText("ZCode") {
		t.Fatalf("accounts tab missing expected accounts: %v", sett.Texts())
	}
}

// TestCaptureLiveMagpieScreenshots captures high-resolution Retina screenshots when
// MAGPIE_E2E_SCREENSHOT_DIR is provided and the local Magpie service is reachable.
func TestCaptureLiveMagpieScreenshots(t *testing.T) {
	outDir := os.Getenv("MAGPIE_E2E_SCREENSHOT_DIR")
	if outDir == "" {
		t.Skip("MAGPIE_E2E_SCREENSHOT_DIR not set")
	}

	cfg := model.LoadConfig()
	cfg.Normalize()
	client := magpie.NewClient(cfg.MagpieURL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap := client.FetchSnapshot(ctx, cfg.IncludeCachedAccounts, magpie.DefaultMagpieDir())
	if !snap.Connected {
		t.Skipf("live magpie not connected: %s", snap.ConnectionErr)
	}
	captureScreenshotSuite(t, outDir, cfg, snap)
}

// TestCaptureReadmeScreenshots captures privacy-safe Retina screenshots for README.md
// when MAGPIE_README_SCREENSHOT_DIR is set.
func TestCaptureReadmeScreenshots(t *testing.T) {
	outDir := os.Getenv("MAGPIE_README_SCREENSHOT_DIR")
	if outDir == "" {
		t.Skip("MAGPIE_README_SCREENSHOT_DIR not set")
	}

	now := time.Now()
	cfg := model.DefaultConfig()
	cfg.Normalize()

	snap := model.Snapshot{
		Connected:      true,
		MagpieVersion:  "0.1.3078",
		MagpieModelCnt: 30,
		FetchedAt:      now,
		UsageToday:     &model.UsageSummary{Period: "1d", Calls: 420, Input: 18400000, Output: 2650000, Cost: 24.80},
		Usage7d:        &model.UsageSummary{Period: "7d", Calls: 2840, Input: 128000000, Output: 19400000, Cost: 162.40},
		Usage30d:       &model.UsageSummary{Period: "30d", Calls: 11200, Input: 512000000, Output: 74000000, Cost: 648.90},
		Accounts: []model.AccountQuota{
			{
				Provider: "antigravity",
				Name:     "Antigravity",
				Plan:     "Google AI Pro",
				User:     "alex@studio.dev",
				ReadAt:   now.Add(-90 * time.Second).Format(time.RFC3339),
				Windows: []model.QuotaWindow{
					{
						Name:     "Gemini · 5 hours",
						Used:     76,
						ResetsAt: now.Add(24 * time.Minute).Format(time.RFC3339),
						Pool:     "Gemini",
						Holds: &model.WindowHolds{
							Tokens: 7520000,
							Cost:   22.44,
							Priced: true,
							Routed: &model.RoutedUsage{Calls: 644, Tokens: 4120000, Cost: 14.10},
						},
					},
					{
						Name:     "Gemini · 7 days",
						Used:     37,
						ResetsAt: now.Add(6*24*time.Hour + 9*time.Hour).Format(time.RFC3339),
						Pool:     "Gemini",
						Holds: &model.WindowHolds{
							Tokens: 15100000,
							Cost:   38.63,
							Priced: true,
							Routed: &model.RoutedUsage{Calls: 1939, Tokens: 9800000, Cost: 25.10},
						},
					},
					{
						Name:     "Claude & GPT · 7 days",
						Used:     18,
						ResetsAt: now.Add(4*24*time.Hour + 11*time.Hour).Format(time.RFC3339),
						Pool:     "Claude & GPT",
					},
				},
			},
			{
				Provider: "claude",
				Name:     "Claude",
				Plan:     "Max 5x",
				User:     "alex@studio.dev",
				ReadAt:   now.Add(-2 * time.Minute).Format(time.RFC3339),
				Windows: []model.QuotaWindow{
					{Name: "5 hours", Used: 34, ResetsAt: now.Add(3*time.Hour + 12*time.Minute).Format(time.RFC3339)},
					{Name: "7 days", Used: 46, ResetsAt: now.Add(3*24*time.Hour + 8*time.Hour).Format(time.RFC3339)},
				},
			},
			{
				Provider: "codex",
				Name:     "Codex",
				Plan:     "Pro",
				User:     "maya@acme.ai",
				ReadAt:   now.Add(-1 * time.Minute).Format(time.RFC3339),
				Windows: []model.QuotaWindow{
					{Name: "5 hours", Used: 18, ResetsAt: now.Add(4*time.Hour + 5*time.Minute).Format(time.RFC3339)},
					{Name: "7 days", Used: 22, ResetsAt: now.Add(5*24*time.Hour + 2*time.Hour).Format(time.RFC3339)},
				},
			},
			{
				Provider: "gemini-web",
				Name:     "Gemini Web",
				Plan:     "AI Pro",
				User:     "maya@acme.ai",
				ReadAt:   now.Add(-4 * time.Minute).Format(time.RFC3339),
				Windows: []model.QuotaWindow{
					{Name: "24 hours", Used: 8, ResetsAt: now.Add(16 * time.Hour).Format(time.RFC3339)},
				},
			},
			{
				Provider:   "copilot",
				Name:       "GitHub Copilot",
				Plan:       "Pro",
				User:       "team@acme.ai",
				CachedOnly: true,
				Windows: []model.QuotaWindow{
					{Name: "Monthly", Used: 54, ResetsAt: now.Add(12 * 24 * time.Hour).Format(time.RFC3339)},
				},
			},
		},
		Histories: map[string]model.AccountHistory{
			"antigravity/alex@studio.dev": {
				Provider: "antigravity",
				User:     "alex@studio.dev",
				Lines: []model.HistoryLine{
					{
						Name: "Gemini · 5 hours",
						Points: []model.HistoryPoint{
							{At: now.Add(-24 * time.Hour).Format(time.RFC3339), Left: 95},
							{At: now.Add(-20 * time.Hour).Format(time.RFC3339), Left: 84},
							{At: now.Add(-16 * time.Hour).Format(time.RFC3339), Left: 72},
							{At: now.Add(-12 * time.Hour).Format(time.RFC3339), Left: 88},
							{At: now.Add(-8 * time.Hour).Format(time.RFC3339), Left: 64},
							{At: now.Add(-4 * time.Hour).Format(time.RFC3339), Left: 45},
							{At: now.Add(-2 * time.Hour).Format(time.RFC3339), Left: 32},
							{At: now.Format(time.RFC3339), Left: 24},
						},
					},
				},
			},
		},
	}
	captureScreenshotSuite(t, outDir, cfg, snap)
}

func captureScreenshotSuite(t *testing.T, outDir string, cfg model.Config, snap model.Snapshot) {
	t.Helper()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	slots := model.BuildRailSlots(snap.Accounts, cfg, time.Now())
	if len(slots) == 0 {
		t.Skip("no live rail slots available")
	}

	m := model.ComputeMetrics(cfg)
	railW := int(math.Ceil(float64(m.Width)))
	railH := int(math.Ceil(float64(m.RailLength(len(slots), true, false))))

	// 1. Rail + hovered slot + Card composite (mimicking desktop right edge)
	activeSlot := slots[0]
	for _, s := range slots {
		if s.UsedFraction != nil && *s.UsedFraction > 0 {
			activeSlot = s
			break
		}
	}

	railState := &panel.RailState{
		Slots:         slots,
		Config:        cfg,
		Expanded:      true,
		HoveredSlotID: activeSlot.SlotID,
	}
	rt := ui.NewTester(func(c *ui.Context) {
		panel.RailView(c, railState)
	}, railW, railH)
	rt.SetScale(2)
	rt.Frame()
	railImg := rt.Image()
	if err := savePNG(filepath.Join(outDir, "magpie-pulse-rail.png"), railImg); err != nil {
		t.Fatal(err)
	}

	slotRect := railState.SlotBounds[activeSlot.SlotID]
	slotCenterY := slotRect.Y + slotRect.H/2
	if slotCenterY <= 0 {
		slotCenterY = float32(railH) / 2
	}

	var histPtr *model.AccountHistory
	if h, ok := snap.Histories[activeSlot.Account.AccountKey()]; ok {
		cp := h
		histPtr = &cp
	}
	cardState := &panel.CardState{
		Slot:          &activeSlot,
		History:       histPtr,
		UsageToday:    snap.UsageToday,
		Usage7d:       snap.Usage7d,
		Usage30d:      snap.Usage30d,
		Config:        cfg,
		PointerCenter: 160,
	}
	cardW := int(math.Ceil(float64(m.CardWidth + m.PointerWidth)))
	cardH := 420
	ct := ui.NewTester(func(c *ui.Context) {
		panel.CardView(c, cardState)
	}, cardW, cardH)
	ct.SetScale(2)
	ct.Frame()
	cardImg := ct.Image()
	if err := savePNG(filepath.Join(outDir, "magpie-pulse-card.png"), cardImg); err != nil {
		t.Fatal(err)
	}

	// Composite rail + card side-by-side over a clean macOS-style backdrop
	compW := (cardW + railW + 48) * 2
	compH := max(railImg.Bounds().Dy(), cardImg.Bounds().Dy()) + 80
	comp := image.NewRGBA(image.Rect(0, 0, compW, compH))
	drawBackdrop(comp)
	railDst := image.Rect(compW-railImg.Bounds().Dx(), (compH-railImg.Bounds().Dy())/2, compW, (compH+railImg.Bounds().Dy())/2)
	draw.Draw(comp, railDst, railImg, image.Point{}, draw.Over)
	cardY := railDst.Min.Y + int(slotCenterY*2) - int(cardState.PointerCenter*2)
	if cardY+cardImg.Bounds().Dy() > compH-20 {
		cardY = compH - 20 - cardImg.Bounds().Dy()
	}
	if cardY < 20 {
		cardY = 20
	}
	cardDst := image.Rect(railDst.Min.X-8-cardImg.Bounds().Dx(), cardY, railDst.Min.X-8, cardY+cardImg.Bounds().Dy())
	draw.Draw(comp, cardDst, cardImg, image.Point{}, draw.Over)
	if err := savePNG(filepath.Join(outDir, "magpie-pulse-dock-and-card.png"), comp); err != nil {
		t.Fatal(err)
	}

	// 2. Settings Window (General + Accounts)
	st := &settings.SettingsState{
		ActiveTab:        settings.TabGeneral,
		Config:           cfg,
		Snapshot:         snap,
		AppVersion:       "0.1.0",
		AutoCheckUpdates: true,
	}
	sett := ui.NewTester(func(c *ui.Context) {
		settings.SettingsView(c, st)
	}, 680, 600)
	sett.SetScale(2)
	sett.SetDark(true)
	sett.SetVibrancy(false)
	sett.Frame()
	if err := savePNG(filepath.Join(outDir, "magpie-pulse-settings-general.png"), sett.Image()); err != nil {
		t.Fatal(err)
	}

	if err := sett.Click("外观与面板"); err != nil {
		t.Fatalf("click appearance tab: %v", err)
	}
	sett.Frame()
	if err := savePNG(filepath.Join(outDir, "magpie-pulse-settings-appearance.png"), sett.Image()); err != nil {
		t.Fatal(err)
	}

	if err := sett.Click("账号总览"); err != nil {
		t.Fatalf("click accounts tab: %v", err)
	}
	sett.Frame()
	if err := savePNG(filepath.Join(outDir, "magpie-pulse-settings-accounts.png"), sett.Image()); err != nil {
		t.Fatal(err)
	}
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func drawBackdrop(dst *image.RGBA) {
	b := dst.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		fy := float64(y-b.Min.Y) / float64(max(b.Dy()-1, 1))
		for x := b.Min.X; x < b.Max.X; x++ {
			fx := float64(x-b.Min.X) / float64(max(b.Dx()-1, 1))
			r := uint8(18 + 22*fx + 14*(1-fy))
			g := uint8(28 + 38*(1-fy) + 28*fx)
			bl := uint8(78 + 95*(1-fx) + 45*fy)
			dst.SetRGBA(x, y, color.RGBA{R: r, G: g, B: bl, A: 255})
		}
	}
}
