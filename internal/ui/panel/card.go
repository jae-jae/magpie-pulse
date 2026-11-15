package panel

import (
	"fmt"
	"math"
	"strings"
	"time"

	"magpie-pulse/internal/i18n"
	"magpie-pulse/internal/icons"
	"magpie-pulse/internal/model"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// CardState holds the account slot, historical series, Magpie summaries,
// layout config, and callbacks for CardView.
type CardState struct {
	Slot          *model.RailSlot
	History       *model.AccountHistory
	Usage30d      *model.UsageSummary
	Usage7d       *model.UsageSummary
	UsageToday    *model.UsageSummary
	Config        model.Config
	PointerCenter float32
	ShowAllModels bool
	Scroll        ui.ScrollState
	OnCopyText    func(string)
	OnRefresh     func()
}

type cardPalette struct {
	light     bool
	fg        ui.Color
	muted     ui.Color
	divider   ui.Color
	badgeBg   ui.Color
	track     ui.Color
	rowHover  ui.Color
	errorFg   ui.Color
	errorBg   ui.Color
	errorEdge ui.Color
}

func newCardPalette(cfg model.Config) cardPalette {
	if cfg.Surface == model.SurfaceLight {
		return cardPalette{
			light:     true,
			fg:        ui.RGB(20, 22, 26),
			muted:     model.LightMutedText,
			divider:   model.LightCardDivider,
			badgeBg:   model.LightSubtleBadgeBg,
			track:     model.LightTrack,
			rowHover:  ui.RGBA(0, 0, 0, 0.05),
			errorFg:   model.PulseWarningLight,
			errorBg:   ui.RGBA(222, 56, 43, 0.09),
			errorEdge: ui.RGBA(222, 56, 43, 0.30),
		}
	}
	return cardPalette{
		light:     false,
		fg:        ui.RGB(242, 244, 248),
		muted:     model.DarkMutedText,
		divider:   model.DarkCardDivider,
		badgeBg:   model.DarkSubtleBadgeBg,
		track:     model.DarkTrack,
		rowHover:  ui.RGBA(255, 255, 255, 0.06),
		errorFg:   model.PulseWarningDark,
		errorBg:   ui.RGBA(255, 79, 66, 0.14),
		errorEdge: ui.RGBA(255, 79, 66, 0.35),
	}
}

// CardView renders Pulse's floating usage detail bubble for the active slot.
func CardView(c *ui.Context, state *CardState) {
	c.Root().Background(ui.Transparent)
	if state == nil || state.Slot == nil {
		return
	}
	cfg := state.Config
	cfg.Normalize()
	msg := i18n.For(cfg.Language, c.Locale())
	m := model.ComputeMetrics(cfg)
	s := m.Scale
	pal := newCardPalette(cfg)
	slot := *state.Slot
	acc := slot.Account
	now := c.Now()

	pad := m.CardPadding
	padTop, padRight, padBottom, padLeft := pad, pad, pad, pad
	extraW, extraH := float32(0), float32(0)
	switch cfg.DockEdge {
	case model.DockRight:
		padRight += m.PointerWidth
		extraW = m.PointerWidth
	case model.DockLeft:
		padLeft += m.PointerWidth
		extraW = m.PointerWidth
	case model.DockTop:
		padTop += m.PointerWidth
		extraH = m.PointerWidth
	case model.DockBottom:
		padBottom += m.PointerWidth
		extraH = m.PointerWidth
	}

	cardCornerRadius := 20 * s
	bubble := ui.Column(c.Key("card:"+slot.SlotID)).
		Width(m.CardWidth+extraW).
		MinHeight(64*s+extraH).
		Padding(padTop, padRight, padBottom, padLeft).
		Gap(10 * s)

	bubble.Draw(func(p *ui.Painter, r ui.Rect) {
		cardRect := r
		switch cfg.DockEdge {
		case model.DockRight:
			cardRect.W -= m.PointerWidth
		case model.DockLeft:
			cardRect.X += m.PointerWidth
			cardRect.W -= m.PointerWidth
		case model.DockTop:
			cardRect.Y += m.PointerWidth
			cardRect.H -= m.PointerWidth
		case model.DockBottom:
			cardRect.H -= m.PointerWidth
		}
		ptrCenter := state.PointerCenter
		if ptrCenter <= 0 {
			if cfg.DockEdge.IsVertical() {
				ptrCenter = cardRect.X + cardRect.W/2
			} else {
				ptrCenter = cardRect.Y + cardRect.H/2
			}
		}
		paintCardSurface(p, cardRect, cfg, ptrCenter, m.PointerWidth, m.PointerHeight, cardCornerRadius)
	})

	bubble.Children(func() {
		renderCardHeader(c, state, slot, acc, cfg, pal, msg, s, now)

		if strings.TrimSpace(acc.Error) != "" {
			renderCardErrorBanner(c, acc.Error, pal, s)
		}

		numeric, textWins := splitAndDedupeWindows(slot.Windows)
		shownNumeric, hasMoreToggle := selectDisplayedWindows(numeric, state.ShowAllModels)

		if len(shownNumeric) > 0 || len(textWins) > 0 || hasMoreToggle {
			renderCardDivider(c, pal)
			scroll := ui.Scroll(c.Key("windows-scroll")).
				TrackScroll(&state.Scroll).
				MaxHeight(260 * s).
				FillWidth()
			scroll.Children(func() {
				ui.Column(c).Gap(10 * s).FillWidth().Children(func() {
					for i, w := range shownNumeric {
						renderNumericWindow(c, i, w, cfg, pal, msg, s, now)
					}
					if hasMoreToggle {
						renderShowAllToggle(c, state, len(numeric), pal, msg, s)
					}
					for i, w := range textWins {
						renderTextWindow(c, i, w, state, pal, s)
					}
				})
			})
		}

		if cfg.ShowDetailedCard {
			if line := pickHistoryLine(state.History, slot); line != nil {
				renderCardDivider(c, pal)
				renderHistorySparkline(c, *line, cfg, pal, msg, s)
			}
			if state.UsageToday != nil || state.Usage7d != nil || state.Usage30d != nil {
				renderCardDivider(c, pal)
				renderUsageFooter(c, state.UsageToday, state.Usage7d, state.Usage30d, pal, msg, s)
			}
		}
	})
}

func paintCardSurface(p *ui.Painter, cardRect ui.Rect, cfg model.Config, ptrCenter, ptrW, ptrH, cornerRadius float32) {
	path := UsageBubblePath(cardRect, cfg.DockEdge, ptrCenter, ptrW, ptrH, cornerRadius)
	switch cfg.Surface {
	case model.SurfaceLight:
		p.FillPath(path, model.PanelLightFill)
		p.StrokePath(path, 1, model.PanelLightEdge)
	case model.SurfaceGlass:
		alpha := float32(0.56 + 0.32*(1-cfg.GlassTransparency))
		tint := ui.RGBA(18, 20, 24, alpha)
		glass.Paint(p, cardRect, cornerRadius, glass.Glass{
			Style:   glass.Regular,
			Tint:    tint,
			Clarity: float32(cfg.GlassTransparency),
		})
		p.FillPath(path, ui.RGBA(18, 20, 24, alpha*0.42))
		p.StrokePath(path, 1, ui.RGBA(255, 255, 255, 0.16))
	default: // SurfaceDark
		p.FillPath(path, ui.RGB(14, 15, 18))
		p.StrokePath(path, 1, ui.RGBA(255, 255, 255, 0.12))
	}
}

func renderCardHeader(c *ui.Context, state *CardState, slot model.RailSlot, acc model.AccountQuota, cfg model.Config, pal cardPalette, msg *i18n.Messages, s float32, now time.Time) {
	ui.Column(c).Gap(4 * s).FillWidth().Children(func() {
		ui.Row(c).Gap(8 * s).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Icon(c, icons.ForProvider(acc.Provider)).Size(18*s, 18*s).TextColor(pal.fg).Shrink(0)
			ui.Text(c, fmt.Sprintf(msg.CardUsageSuffixFmt, acc.DisplayNameLocale(msg.Locale))).
				FontSize(13.5 * s).
				FontWeight(600).
				TextColor(pal.fg).
				SingleLine().
				Grow(1)
			if strings.TrimSpace(acc.Plan) != "" {
				renderPill(c, acc.Plan, pal.badgeBg, pal.fg, 10*s, s)
			}
			if state.OnRefresh != nil {
				btn := ui.Box(c.Key("refresh-btn")).
					Size(22*s, 22*s).
					Radius(6 * s).
					Center().
					Cursor(ui.CursorPointer).
					Label(msg.CardRefreshLabel)
				if btn.Hovered() {
					btn.Background(pal.rowHover)
				}
				if btn.Clicked() {
					state.OnRefresh()
				}
				btn.Children(func() {
					ui.Icon(c, icons.RefreshIcon).Size(13*s, 13*s).TextColor(pal.muted)
				})
			}
		})

		var parts []string
		if strings.TrimSpace(acc.User) != "" {
			parts = append(parts, acc.User)
		}
		if rt, ok := acc.ReadTime(); ok {
			parts = append(parts, fmt.Sprintf(msg.CardUpdatedFmt, model.FormatRelativeTimeLocale(msg.Locale, now, rt)))
		}
		if slot.Pool != "" && cfg.SplitPools {
			parts = append(parts, slot.Pool)
		}

		if len(parts) > 0 || acc.CachedOnly {
			ui.Row(c).Gap(6 * s).AlignItems(ui.Center).FillWidth().Children(func() {
				if len(parts) > 0 {
					ui.Text(c, strings.Join(parts, " · ")).
						FontSize(11 * s).
						TextColor(pal.muted).
						SingleLine().
						Grow(1)
				}
				if acc.CachedOnly {
					renderPill(c, msg.CardCachedPill, pal.badgeBg, pal.muted, 9.5*s, s)
				}
			})
		}
	})
}

func renderPill(c *ui.Context, text string, bg, fg ui.Color, fontSize, s float32) {
	ui.Box(c).
		Padding(2*s, 7*s).
		Radius(999).
		Background(bg).
		Shrink(0).
		Children(func() {
			ui.Text(c, text).FontSize(fontSize).FontWeight(600).TextColor(fg).SingleLine()
		})
}

func renderCardErrorBanner(c *ui.Context, errMsg string, pal cardPalette, s float32) {
	ui.Row(c).
		Gap(7*s).
		Padding(8*s, 10*s).
		Radius(8*s).
		Background(pal.errorBg).
		Border(1, pal.errorEdge).
		AlignItems(ui.Start).
		FillWidth().
		Children(func() {
			ui.Icon(c, icons.AlertIcon).Size(14*s, 14*s).TextColor(pal.errorFg).Shrink(0)
			ui.Text(c, errMsg).FontSize(11.5 * s).TextColor(pal.errorFg).Grow(1)
		})
}

func renderCardDivider(c *ui.Context, pal cardPalette) {
	ui.Box(c).Height(1).FillWidth().Background(pal.divider).Shrink(0)
}

func renderNumericWindow(c *ui.Context, idx int, w model.QuotaWindow, cfg model.Config, pal cardPalette, msg *i18n.Messages, s float32, now time.Time) {
	usedFrac := w.UsedFraction()
	fillFrac := float32(usedFrac)
	if cfg.ShowRemaining {
		fillFrac = float32(w.RemainingFraction())
	}
	barColor := model.UsageTintColor(usedFrac, w.IsSpent(), cfg.WarningThreshold, pal.light)

	ui.Column(c.Key(fmt.Sprintf("win:%d:%s:%s", idx, w.Pool, w.Name))).Gap(4 * s).FillWidth().Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Text(c, w.Name).FontSize(12 * s).FontWeight(500).TextColor(pal.fg).SingleLine().Grow(1)
			if w.Pool != "" && !strings.Contains(strings.ToLower(w.Name), strings.ToLower(w.Pool)) {
				ui.Text(c, w.Pool).FontSize(10 * s).TextColor(pal.muted).SingleLine()
			}
		})

		// UsageBar drawn with Painter (Continuous capsule or Segmented ticks).
		ui.Box(c).Height(7 * s).FillWidth().Draw(func(p *ui.Painter, r ui.Rect) {
			drawUsageBar(p, r, fillFrac, barColor, pal.track, cfg.BarStyle, s)
		})

		// Left percentage & right reset countdown.
		pctText := fmt.Sprintf(msg.CardPctUsedFmt, math.Round(w.Used))
		if cfg.ShowRemaining {
			rem := w.RemainingFraction() * 100
			pctText = fmt.Sprintf(msg.CardPctLeftFmt, math.Round(rem))
		}
		resetText := formatWindowReset(w, msg.Locale, now)

		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Text(c, pctText).FontSize(11 * s).FontFeatures("tnum").TextColor(pal.muted).SingleLine()
			if resetText != "" {
				ui.Text(c, resetText).FontSize(10.5 * s).FontFeatures("tnum").TextColor(pal.muted).SingleLine()
			}
		})

		if holdsLine := formatWindowHolds(w.Holds, msg); holdsLine != "" {
			ui.Text(c, holdsLine).FontSize(10.5 * s).FontFeatures("tnum").TextColor(pal.muted).SingleLine()
		}
	})
}

func drawUsageBar(p *ui.Painter, r ui.Rect, fillFrac float32, fillColor, trackColor ui.Color, style model.BarStyle, s float32) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	fillFrac = clamp01(fillFrac)
	if style == model.BarSegmented {
		const segments = 10
		gap := 2 * s
		segW := (r.W - gap*float32(segments-1)) / float32(segments)
		if segW <= 1 {
			segW = r.W / float32(segments)
			gap = 0
		}
		segRad := min(2*s, r.H/2)
		for i := 0; i < segments; i++ {
			sx := r.X + float32(i)*(segW+gap)
			segBox := ui.Rect{X: sx, Y: r.Y, W: segW, H: r.H}
			p.Fill(segBox, trackColor, segRad)
			lo := float32(i) / float32(segments)
			hi := float32(i+1) / float32(segments)
			if fillFrac <= lo {
				continue
			}
			if fillFrac >= hi {
				p.Fill(segBox, fillColor, segRad)
				continue
			}
			part := (fillFrac - lo) / (hi - lo)
			p.Clip(segBox, segRad, func() {
				p.Fill(ui.Rect{X: sx, Y: r.Y, W: segW * part, H: r.H}, fillColor, segRad)
			})
		}
		return
	}

	rad := r.H / 2
	p.Fill(r, trackColor, rad)
	if fillFrac > 0.002 {
		fw := r.W * fillFrac
		p.Clip(r, rad, func() {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(fw, r.H*0.6), H: r.H}, fillColor, rad)
		})
	}
}

func renderShowAllToggle(c *ui.Context, state *CardState, total int, pal cardPalette, msg *i18n.Messages, s float32) {
	label := fmt.Sprintf(msg.CardShowAllFmt, total)
	if state.ShowAllModels {
		label = msg.CardShowSummary
	}
	btn := ui.Box(c.Key("toggle-all-models")).
		Padding(5*s, 8*s).
		Radius(8 * s).
		Background(pal.badgeBg).
		Center().
		FillWidth().
		Cursor(ui.CursorPointer).
		Label(label)
	if btn.Hovered() {
		btn.Background(pal.rowHover)
	}
	if btn.Clicked() {
		state.ShowAllModels = !state.ShowAllModels
	}
	btn.Children(func() {
		ui.Text(c, label).FontSize(11 * s).FontWeight(500).TextColor(pal.fg).SingleLine()
	})
}

func renderTextWindow(c *ui.Context, idx int, w model.QuotaWindow, state *CardState, pal cardPalette, s float32) {
	row := ui.Row(c.Key(fmt.Sprintf("text-win:%d:%s", idx, w.Name))).
		Justify(ui.SpaceBetween).
		AlignItems(ui.Center).
		Padding(6*s, 8*s).
		Radius(8 * s).
		Background(pal.badgeBg).
		FillWidth().
		Cursor(ui.CursorPointer).
		Label(w.Name)
	if row.Hovered() {
		row.Background(pal.rowHover)
	}
	if row.Clicked() && state.OnCopyText != nil {
		copyVal := w.Display
		if copyVal == "" {
			copyVal = w.Name
		}
		state.OnCopyText(copyVal)
	}
	row.Children(func() {
		ui.Text(c, w.Name).FontSize(11.5 * s).TextColor(pal.muted).SingleLine()
		val := w.Display
		if val == "" {
			val = "--"
		}
		ui.Text(c, val).FontSize(11.5 * s).FontWeight(600).FontFeatures("tnum").TextColor(pal.fg).SingleLine()
	})
}

func renderHistorySparkline(c *ui.Context, line model.HistoryLine, cfg model.Config, pal cardPalette, msg *i18n.Messages, s float32) {
	pts := line.Points
	last := pts[len(pts)-1]
	tint := model.UsageTintColor((100-last.Left)/100, last.Left <= 0, cfg.WarningThreshold, pal.light)
	title := msg.CardQuotaHistory
	if strings.TrimSpace(line.Name) != "" {
		title = fmt.Sprintf(msg.CardNamedHistoryFmt, line.Name)
	}

	ui.Column(c).Gap(5 * s).FillWidth().Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Text(c, title).FontSize(10.5 * s).TextColor(pal.muted).SingleLine()
			ui.Text(c, fmt.Sprintf(msg.CardSparklineLeftFmt, math.Round(last.Left))).
				FontSize(10.5 * s).FontFeatures("tnum").TextColor(pal.muted).SingleLine()
		})
		ui.Box(c).Height(34 * s).FillWidth().Radius(8 * s).Background(pal.badgeBg).Draw(func(p *ui.Painter, r ui.Rect) {
			pad := 5 * s
			w := r.W - 2*pad
			h := r.H - 2*pad
			if w <= 2 || h <= 2 {
				return
			}
			coords := make([][2]float32, 0, len(pts))
			denom := float32(max(len(pts)-1, 1))
			for i, pt := range pts {
				frac := clamp01(float32(pt.Left / 100.0))
				x := r.X + pad + (float32(i)/denom)*w
				y := r.Y + pad + (1-frac)*h
				coords = append(coords, [2]float32{x, y})
			}
			p.StrokePolyline(coords, 1.75*s, tint)
		})
	})
}

func renderUsageFooter(c *ui.Context, today, week, month *model.UsageSummary, pal cardPalette, msg *i18n.Messages, s float32) {
	cols := []struct {
		label string
		data  *model.UsageSummary
	}{
		{msg.CardToday, today},
		{msg.Card7Days, week},
		{msg.Card30Days, month},
	}
	ui.Row(c).Gap(6 * s).FillWidth().Children(func() {
		for _, col := range cols {
			ui.Column(c).Gap(2 * s).Grow(1).Children(func() {
				ui.Text(c, col.label).FontSize(10 * s).TextColor(pal.muted).SingleLine()
				if col.data == nil {
					ui.Text(c, "--").FontSize(11.5 * s).FontWeight(600).TextColor(pal.muted).SingleLine()
					return
				}
				ui.Text(c, model.FormatShortTokens(col.data.TotalTokens())).
					FontSize(11.5 * s).
					FontWeight(600).
					FontFeatures("tnum").
					TextColor(pal.fg).
					SingleLine()
				ui.Text(c, fmt.Sprintf("$%.2f", col.data.Cost)).
					FontSize(10 * s).
					FontFeatures("tnum").
					TextColor(pal.muted).
					SingleLine()
			})
		}
	})
}

func splitAndDedupeWindows(wins []model.QuotaWindow) (numeric []model.QuotaWindow, textWins []model.QuotaWindow) {
	type winKey struct {
		pool     string
		name     string
		isText   bool
		display  string
		resetsAt string
		used     float64
	}
	seen := make(map[winKey]bool, len(wins))
	for _, w := range wins {
		k := winKey{
			pool:     w.Pool,
			name:     w.Name,
			isText:   w.IsText,
			display:  w.Display,
			resetsAt: w.ResetsAt,
			used:     w.Used,
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		if w.IsText {
			textWins = append(textWins, w)
		} else {
			numeric = append(numeric, w)
		}
	}
	return model.SortWindowsByCycle(numeric), textWins
}

func selectDisplayedWindows(numeric []model.QuotaWindow, showAll bool) ([]model.QuotaWindow, bool) {
	if len(numeric) <= 6 {
		return numeric, false
	}
	if showAll {
		return numeric, true
	}

	var summaries []model.QuotaWindow
	for _, w := range numeric {
		low := strings.ToLower(w.Name)
		isPoolSummary := w.Family == "" ||
			(w.Pool != "" && strings.EqualFold(w.Name, w.Pool)) ||
			strings.Contains(low, "pool") ||
			strings.Contains(low, "overall")
		if isPoolSummary {
			summaries = append(summaries, w)
		}
	}
	if len(summaries) == 0 || len(summaries) >= len(numeric) {
		summaries = summaries[:0]
		seenPool := map[string]bool{}
		for _, w := range numeric {
			if w.Pool != "" && !seenPool[w.Pool] {
				seenPool[w.Pool] = true
				summaries = append(summaries, w)
			}
		}
	}
	if len(summaries) == 0 || len(summaries) >= len(numeric) {
		limit := min(4, len(numeric))
		return numeric[:limit], true
	}
	if len(summaries) > 6 {
		summaries = summaries[:6]
	}
	return summaries, true
}

func pickHistoryLine(hist *model.AccountHistory, slot model.RailSlot) *model.HistoryLine {
	if hist == nil || len(hist.Lines) == 0 {
		return nil
	}
	if slot.Primary != nil {
		for i := range hist.Lines {
			if len(hist.Lines[i].Points) >= 2 && strings.EqualFold(hist.Lines[i].Name, slot.Primary.Name) {
				return &hist.Lines[i]
			}
		}
	}
	for i := range hist.Lines {
		if len(hist.Lines[i].Points) >= 2 {
			return &hist.Lines[i]
		}
	}
	return nil
}

func formatWindowReset(w model.QuotaWindow, locale string, now time.Time) string {
	if rt, ok := w.ResetTime(); ok {
		return model.FormatResetCountdownLocale(locale, now, rt)
	}
	if w.ResetSecs > 0 {
		return model.FormatResetCountdownLocale(locale, now, now.Add(time.Duration(w.ResetSecs)*time.Second))
	}
	return ""
}

func formatWindowHolds(h *model.WindowHolds, msg *i18n.Messages) string {
	if h == nil {
		return ""
	}
	if h.Routed != nil {
		r := h.Routed
		parts := []string{
			model.FormatShortTokens(r.Tokens) + " " + msg.CardTokenUnit,
			fmt.Sprintf("$%.2f", r.Cost),
			fmt.Sprintf(msg.CardCallsFmt, formatIntCommas(r.Calls)),
		}
		return strings.Join(parts, " · ")
	}
	if h.Tokens > 0 || h.Cost > 0 {
		parts := []string{model.FormatShortTokens(h.Tokens) + " " + msg.CardTokenUnit}
		if h.Priced || h.Cost > 0 {
			parts = append(parts, fmt.Sprintf("$%.2f", h.Cost))
		}
		return strings.Join(parts, " · ")
	}
	return ""
}

func formatIntCommas(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	raw := fmt.Sprintf("%d", n)
	if len(raw) <= 3 {
		if neg {
			return "-" + raw
		}
		return raw
	}
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	rem := len(raw) % 3
	if rem > 0 {
		b.WriteString(raw[:rem])
		if len(raw) > rem {
			b.WriteByte(',')
		}
	}
	for i := rem; i < len(raw); i += 3 {
		b.WriteString(raw[i : i+3])
		if i+3 < len(raw) {
			b.WriteByte(',')
		}
	}
	return b.String()
}
