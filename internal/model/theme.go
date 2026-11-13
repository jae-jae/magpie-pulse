package model

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

const CautionThreshold = 0.50

// Pulse usage palette (dark & light twins from Pulse's UsageTint.swift).
var (
	PulseGoodDark       = ui.RGB(0, 230, 140)
	PulseCautionDark    = ui.RGB(255, 194, 38)
	PulseWarningDark    = ui.RGB(255, 79, 66)
	PulseExhaustedDark  = ui.RGB(217, 23, 33)
	PulseGoodLight      = ui.RGB(0, 148, 89)
	PulseCautionLight   = ui.RGB(194, 125, 0)
	PulseWarningLight   = ui.RGB(222, 56, 43)
	PulseExhaustedLight = ui.RGB(173, 15, 26)

	PanelDarkFill      = ui.RGB(0, 0, 0)
	PanelLightFill     = ui.RGB(247, 247, 247)
	PanelLightEdge     = ui.RGBA(0, 0, 0, 0.12)
	DarkTrack          = ui.RGBA(255, 255, 255, 0.14)
	LightTrack         = ui.RGBA(0, 0, 0, 0.11)
	DarkCardDivider    = ui.RGBA(255, 255, 255, 0.11)
	LightCardDivider   = ui.RGBA(0, 0, 0, 0.10)
	DarkMutedText      = ui.RGBA(255, 255, 255, 0.59)
	LightMutedText     = ui.RGBA(0, 0, 0, 0.57)
	DarkSubtleBadgeBg  = ui.RGBA(255, 255, 255, 0.09)
	LightSubtleBadgeBg = ui.RGBA(0, 0, 0, 0.06)
)

// UsageTintColor returns the Pulse status color for a usage fraction (0..1).
func UsageTintColor(usedFrac float64, isSpent bool, warningAtPct int, light bool) ui.Color {
	warn := float64(warningAtPct) / 100.0
	if warn <= CautionThreshold {
		warn = 0.75
	}
	if isSpent || usedFrac >= 1.0 {
		if light {
			return PulseExhaustedLight
		}
		return PulseExhaustedDark
	}
	if usedFrac < CautionThreshold {
		if light {
			return PulseGoodLight
		}
		return PulseGoodDark
	}
	if usedFrac < warn {
		if light {
			return PulseCautionLight
		}
		return PulseCautionDark
	}
	if light {
		return PulseWarningLight
	}
	return PulseWarningDark
}

// ParseHexColor parses "#RRGGBB" into ui.Color.
func ParseHexColor(hex string) (ui.Color, bool) {
	s := strings.TrimSpace(hex)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return ui.Color{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return ui.Color{}, false
	}
	return ui.RGB(uint8((v>>16)&0xFF), uint8((v>>8)&0xFF), uint8(v&0xFF)), true
}

// DockMetrics mirrors Pulse's DockLayout and DetailCardLayout geometry budgets.
type DockMetrics struct {
	Scale               float32
	Width               float32
	VerticalPadding     float32
	HorizontalPadding   float32
	RingDiameter        float32
	RingLineWidth       float32
	SecondRingDiameter  float32
	SecondRingLineWidth float32
	ClockReach          float32
	RingToTextSpacing   float32
	PercentFontSize     float32
	PercentTextHeight   float32
	PercentTextWidth    float32
	ItemSpacing         float32
	CornerRadius        float32
	FlareHeight         float32
	FlareWidth          float32
	CollapsedWidth      float32
	ItemHeight          float32
	CardWidth           float32
	CardPadding         float32
	CardCornerRadius    float32
	PointerWidth        float32
	PointerHeight       float32
	CardGap             float32
}

func ComputeMetrics(cfg Config) DockMetrics {
	s := cfg.Scale.Factor()
	width := 64 * s
	ringDiam := 36 * s
	ringLW := 4 * s
	clockReach := float32(0)
	if cfg.ShowClockArc {
		clockReach = 5 * s
	}
	cornerRadius := 26 * s
	flareHeight := 24 * s
	flareWidth := 38 * s
	vertPad := 46 * s
	if cfg.RoundEnds {
		cornerRadius = width / 2
		flareHeight = cornerRadius
		flareWidth = width - cornerRadius
		vertPad = flareHeight + cornerRadius - ringDiam/2 + 8*s
	}
	ringToText := 6*s + clockReach
	pctH := 16 * s
	itemH := ringDiam
	if cfg.ShowPercentages {
		itemH = ringDiam + ringToText + pctH
	}
	return DockMetrics{
		Scale:               s,
		Width:               width,
		VerticalPadding:     vertPad,
		HorizontalPadding:   10 * s,
		RingDiameter:        ringDiam,
		RingLineWidth:       ringLW,
		SecondRingDiameter:  26 * s,
		SecondRingLineWidth: 2.5 * s,
		ClockReach:          clockReach,
		RingToTextSpacing:   ringToText,
		PercentFontSize:     13 * s,
		PercentTextHeight:   pctH,
		PercentTextWidth:    38 * s,
		ItemSpacing:         28 * s,
		CornerRadius:        cornerRadius,
		FlareHeight:         flareHeight,
		FlareWidth:          flareWidth,
		CollapsedWidth:      6 * s,
		ItemHeight:          itemH,
		CardWidth:           272 * s,
		CardPadding:         16 * s,
		CardCornerRadius:    (ringDiam + ringLW) / 2,
		PointerWidth:        18 * s,
		PointerHeight:       38 * s,
		CardGap:             4 * s,
	}
}

// RailLength returns the full length of the rail along its main axis for n slots.
func (m DockMetrics) RailLength(slots int, docked bool, vertical bool) float32 {
	if slots <= 0 {
		slots = 1
	}
	endPad := m.EndPadding(docked)
	step := m.ItemStep(vertical)
	itemDim := m.ItemHeight
	if vertical && m.ShowPercentagesEstimate() {
		itemDim = m.RingDiameter + m.RingToTextSpacing + m.PercentTextHeight
	}
	return endPad*2 + float32(slots)*itemDim + float32(slots-1)*(step-itemDim)
}

func (m DockMetrics) ShowPercentagesEstimate() bool {
	return m.ItemHeight > m.RingDiameter
}

func (m DockMetrics) EndPadding(docked bool) float32 {
	if docked {
		return m.VerticalPadding
	}
	if m.VerticalPadding > m.FlareHeight {
		return m.VerticalPadding - m.FlareHeight
	}
	return 16 * m.Scale
}

func (m DockMetrics) ItemStep(vertical bool) float32 {
	if vertical {
		return m.RingDiameter + 24*m.Scale
	}
	return m.ItemHeight + m.ItemSpacing
}
