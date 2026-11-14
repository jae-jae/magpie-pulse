package panel

import (
	"time"

	"magpie-pulse/internal/icons"
	"magpie-pulse/internal/model"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// RailProps is an alias for RailState for callers preferring props naming.
type RailProps = RailState

// RailState holds the data, visual flags, and callbacks for RailView.
type RailState struct {
	Slots          []model.RailSlot
	Config         model.Config
	HoveredSlotID  string
	SelectedSlotID string
	Expanded       bool
	Refreshing     bool
	SlotBounds     map[string]ui.Rect
	OnHoverSlot    func(slotID string, rect ui.Rect)
	OnLeaveRail    func()
	OnClickSlot    func(slotID string)
	OnRightClick   func()
	OnOpenSettings func()
}

// RailView renders Pulse's docked (or floating) berth rail and its usage rings.
func RailView(c *ui.Context, state *RailState) {
	if state == nil {
		return
	}
	cfg := state.Config
	cfg.Normalize()
	if state.SlotBounds == nil {
		state.SlotBounds = map[string]ui.Rect{}
	}

	root := c.Root().Background(ui.Transparent)
	switch cfg.DockEdge {
	case model.DockRight:
		root.AlignItems(ui.End).Justify(ui.Center)
	case model.DockLeft:
		root.AlignItems(ui.Start).Justify(ui.Center)
	case model.DockTop:
		root.Justify(ui.Start).AlignItems(ui.Center)
	case model.DockBottom:
		root.Justify(ui.End).AlignItems(ui.Center)
	default:
		root.Center()
	}

	m := model.ComputeMetrics(cfg)
	isVertical := cfg.DockEdge.IsVertical()
	isDocked := cfg.DockEdge.IsDocked()
	length := m.RailLength(len(state.Slots), isDocked, isVertical)
	railW, railH := m.Width, length
	if isVertical {
		railW, railH = length, m.Width
	}

	rail := ui.Box(c.Key("rail")).Size(railW, railH).DragWindow()
	targetOpen := float32(1.0)
	if cfg.AutoCollapse && isDocked && !state.Expanded {
		targetOpen = 0.0
	}
	openness := rail.Animate("berth-openness", targetOpen, 180*time.Millisecond)

	light := cfg.Surface == model.SurfaceLight
	alertColor := worstAlertColor(state.Slots, cfg, light)

	rail.Draw(func(p *ui.Painter, r ui.Rect) {
		paintRailSurface(p, r, cfg, m, openness, alertColor)
	})

	if state.OnRightClick != nil {
		rail.ObserveInput(func(ev ui.InputEvent) {
			if ev.Kind == ui.InputPointerUp && ev.Button == 1 && !ev.Canceled {
				state.OnRightClick()
			}
		})
	}

	railHovered := rail.Hovered()
	prevHoveredSlot := ui.Local(rail, "prevHoverSlot", func() string { return "" })
	hoveredID := ""
	var hoveredRect ui.Rect

	slotOpacity := float32(1.0)
	if openness < 1 {
		slotOpacity = clamp01((openness - 0.35) / 0.65)
	}
	slotsHidden := openness < 0.05

	rail.Children(func() {
		endPad := m.EndPadding(isDocked)
		step := m.ItemStep(isVertical)

		if len(state.Slots) == 0 {
			btnSize := m.RingDiameter
			var bx, by float32
			if isVertical {
				bx = endPad
				by = (railH - btnSize) / 2
			} else {
				bx = (railW - btnSize) / 2
				by = endPad
			}
			emptyBtn := ui.Box(c.Key("empty-settings")).
				Absolute().Left(bx).Top(by).Size(btnSize, btnSize).
				Center().Cursor(ui.CursorPointer).Label("Settings").
				Opacity(slotOpacity).Hidden(slotsHidden)
			if emptyBtn.Clicked() && state.OnOpenSettings != nil {
				state.OnOpenSettings()
			}
			emptyBtn.Children(func() {
				iconColor := ui.RGBA(255, 255, 255, 0.72)
				if light {
					iconColor = ui.RGBA(0, 0, 0, 0.65)
				}
				ui.Icon(c, icons.SettingsIcon).Size(18*m.Scale, 18*m.Scale).TextColor(iconColor)
			})
			return
		}

		for i, slot := range state.Slots {
			var slotX, slotY, slotW, slotH float32
			if isVertical {
				slotW = m.RingDiameter
				if cfg.ShowPercentages {
					slotW = m.RingDiameter + m.RingToTextSpacing + m.PercentTextHeight
				}
				slotH = m.Width
				slotX = endPad + float32(i)*step
				slotY = 0
			} else {
				slotW = m.Width
				slotH = m.ItemHeight
				slotX = 0
				slotY = endPad + float32(i)*step
			}

			slotBox := ui.Box(c.Key("slot:"+slot.SlotID)).
				Absolute().Left(slotX).Top(slotY).Size(slotW, slotH).
				Cursor(ui.CursorPointer).Label(slot.Title).
				Opacity(slotOpacity).Hidden(slotsHidden)

			bounds := slotBox.Bounds()
			if bounds.W <= 0 || bounds.H <= 0 {
				rb := rail.Bounds()
				bounds = ui.Rect{X: rb.X + slotX, Y: rb.Y + slotY, W: slotW, H: slotH}
			}
			state.SlotBounds[slot.SlotID] = bounds

			slotHovered := slotBox.Hovered()
			if slotHovered {
				hoveredID = slot.SlotID
				hoveredRect = bounds
			}
			if slotBox.Clicked() && state.OnClickSlot != nil {
				state.OnClickSlot(slot.SlotID)
			}

			isActive := slotHovered || state.HoveredSlotID == slot.SlotID || state.SelectedSlotID == slot.SlotID
			slotCopy := slot
			slotBox.Draw(func(p *ui.Painter, r ui.Rect) {
				state.SlotBounds[slotCopy.SlotID] = r
				DrawUsageRing(p, r, slotCopy, cfg, m, isActive, state.Refreshing, light)
			})
		}
	})

	if hoveredID != "" {
		if *prevHoveredSlot != hoveredID || state.HoveredSlotID != hoveredID {
			*prevHoveredSlot = hoveredID
			if state.OnHoverSlot != nil {
				state.OnHoverSlot(hoveredID, hoveredRect)
			}
		}
	} else if !railHovered {
		if *prevHoveredSlot != "" || state.HoveredSlotID != "" {
			*prevHoveredSlot = ""
			if state.OnLeaveRail != nil {
				state.OnLeaveRail()
			}
		}
	}
}

func paintRailSurface(p *ui.Painter, r ui.Rect, cfg model.Config, m model.DockMetrics, openness float32, alertColor *ui.Color) {
	path := DockBerthPath(r, cfg.DockEdge, m, cfg.RoundEnds, openness)
	switch cfg.Surface {
	case model.SurfaceLight:
		fill := model.PanelLightFill
		if alertColor != nil && openness < 0.4 {
			fill = fill.Mix(*alertColor, 0.85*(1-openness))
		}
		p.FillPath(path, fill)
		p.StrokePath(path, 1, model.PanelLightEdge)
	case model.SurfaceGlass:
		alpha := float32(0.58 + 0.32*(1-cfg.GlassTransparency))
		tint := ui.RGBA(18, 20, 24, alpha)
		if alertColor != nil && openness < 0.4 {
			tint = tint.Mix(*alertColor, 0.85*(1-openness))
		}
		if cfg.DockEdge == model.DockFloating && openness > 0.25 {
			glass.Paint(p, r, min(r.W, r.H)/2, glass.Glass{
				Style:   glass.Regular,
				Tint:    tint,
				Clarity: float32(cfg.GlassTransparency),
			})
		} else {
			p.FillPath(path, tint)
			p.StrokePath(path, 1, ui.RGBA(255, 255, 255, 0.14))
		}
	default: // SurfaceDark
		fill := model.PanelDarkFill
		if alertColor != nil && openness < 0.4 {
			fill = fill.Mix(*alertColor, 0.85*(1-openness))
		}
		p.FillPath(path, fill)
		if openness < 0.95 {
			p.StrokePath(path, 1, ui.RGBA(255, 255, 255, 0.14+0.18*(1-openness)))
		}
	}
}

func worstAlertColor(slots []model.RailSlot, cfg model.Config, light bool) *ui.Color {
	thresh := float64(cfg.WarningThreshold) / 100.0
	worstFrac := -1.0
	worstSpent := false
	var worst *ui.Color
	for _, s := range slots {
		if s.UsedFraction == nil && !s.IsSpent {
			continue
		}
		used := 0.0
		if s.UsedFraction != nil {
			used = *s.UsedFraction
		}
		if used < thresh && !s.IsSpent {
			continue
		}
		if (s.IsSpent && !worstSpent) || used > worstFrac {
			worstFrac = used
			worstSpent = s.IsSpent
			c := model.UsageTintColor(used, s.IsSpent, cfg.WarningThreshold, light)
			worst = &c
		}
	}
	return worst
}
