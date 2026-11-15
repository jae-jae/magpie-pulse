package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Language string

const (
	LangAuto   Language = "auto"
	LangZhHans Language = "zh-Hans"
	LangEn     Language = "en"
)

// NormalizeLanguage maps a configured or BCP 47 language string into one of
// LangAuto, LangZhHans, or LangEn.
func NormalizeLanguage(lang Language) Language {
	low := strings.ToLower(strings.TrimSpace(string(lang)))
	low = strings.ReplaceAll(low, "_", "-")
	switch {
	case low == "", low == "auto":
		return LangAuto
	case low == "en", strings.HasPrefix(low, "en-"):
		return LangEn
	case low == "zh", strings.HasPrefix(low, "zh-hans"), strings.HasPrefix(low, "zh-cn"), strings.HasPrefix(low, "zh-sg"):
		return LangZhHans
	default:
		return LangAuto
	}
}

type SurfaceMode string

const (
	SurfaceDark  SurfaceMode = "dark"
	SurfaceLight SurfaceMode = "light"
	SurfaceGlass SurfaceMode = "glass"
)

type DockEdge string

const (
	DockRight    DockEdge = "right"
	DockLeft     DockEdge = "left"
	DockTop      DockEdge = "top"
	DockBottom   DockEdge = "bottom"
	DockFloating DockEdge = "floating"
)

func (e DockEdge) IsVertical() bool {
	return e == DockTop || e == DockBottom
}

func (e DockEdge) IsDocked() bool {
	return e != DockFloating
}

type BarStyle string

const (
	BarContinuous BarStyle = "continuous"
	BarSegmented  BarStyle = "segmented"
)

type PanelScale string

const (
	ScaleSmall    PanelScale = "small"
	ScaleStandard PanelScale = "standard"
	ScaleLarge    PanelScale = "large"
)

func (s PanelScale) Factor() float32 {
	switch s {
	case ScaleSmall:
		return 0.85
	case ScaleLarge:
		return 1.15
	default:
		return 1.0
	}
}

type Config struct {
	MagpieURL             string            `json:"magpieUrl"`
	RefreshIntervalSecs   int               `json:"refreshIntervalSecs"`
	IncludeCachedAccounts bool              `json:"includeCachedAccounts"`
	Language              Language          `json:"language"`
	Surface               SurfaceMode       `json:"surface"`
	GlassTransparency     float64           `json:"glassTransparency"`
	DockEdge              DockEdge          `json:"dockEdge"`
	BarStyle              BarStyle          `json:"barStyle"`
	Scale                 PanelScale        `json:"scale"`
	WarningThreshold      int               `json:"warningThreshold"`
	ShowRemaining         bool              `json:"showRemaining"`
	ShowSecondRing        bool              `json:"showSecondRing"`
	ShowClockArc          bool              `json:"showClockArc"`
	SplitPools            bool              `json:"splitPools"`
	RoundEnds             bool              `json:"roundEnds"`
	AutoCollapse          bool              `json:"autoCollapse"`
	ShowPercentages       bool              `json:"showPercentages"`
	ShowDetailedCard      bool              `json:"showDetailedCard"`
	ShowInDock            bool              `json:"showInDock"`
	NotifyOnThreshold     bool              `json:"notifyOnThreshold"`
	HiddenAccounts        map[string]bool   `json:"hiddenAccounts"`
	RingTints             map[string]string `json:"ringTints"`
	FloatX                int               `json:"floatX"`
	FloatY                int               `json:"floatY"`
	RailRatio             float64           `json:"railRatio"`
}

func DefaultConfig() Config {
	return Config{
		MagpieURL:             "http://127.0.0.1:3425",
		RefreshIntervalSecs:   30,
		IncludeCachedAccounts: false,
		Language:              LangAuto,
		Surface:               SurfaceDark,
		GlassTransparency:     0.5,
		DockEdge:              DockRight,
		BarStyle:              BarContinuous,
		Scale:                 ScaleStandard,
		WarningThreshold:      75,
		ShowRemaining:         false,
		ShowSecondRing:        true,
		ShowClockArc:          false,
		SplitPools:            true,
		RoundEnds:             false,
		AutoCollapse:          false,
		ShowPercentages:       true,
		ShowDetailedCard:      true,
		ShowInDock:            false,
		NotifyOnThreshold:     true,
		HiddenAccounts:        map[string]bool{},
		RingTints:             map[string]string{},
		RailRatio:             0.35,
	}
}

func (c *Config) Normalize() {
	def := DefaultConfig()
	if c.MagpieURL == "" {
		c.MagpieURL = def.MagpieURL
	}
	if c.RefreshIntervalSecs < 5 {
		c.RefreshIntervalSecs = def.RefreshIntervalSecs
	}
	c.Language = NormalizeLanguage(c.Language)
	if c.Surface != SurfaceDark && c.Surface != SurfaceLight && c.Surface != SurfaceGlass {
		c.Surface = def.Surface
	}
	if c.GlassTransparency < 0 || c.GlassTransparency > 1 {
		c.GlassTransparency = def.GlassTransparency
	}
	switch c.DockEdge {
	case DockRight, DockLeft, DockTop, DockBottom, DockFloating:
	default:
		c.DockEdge = def.DockEdge
	}
	if c.BarStyle != BarContinuous && c.BarStyle != BarSegmented {
		c.BarStyle = def.BarStyle
	}
	switch c.Scale {
	case ScaleSmall, ScaleStandard, ScaleLarge:
	default:
		c.Scale = def.Scale
	}
	if c.WarningThreshold < 55 || c.WarningThreshold > 95 {
		c.WarningThreshold = def.WarningThreshold
	}
	if c.HiddenAccounts == nil {
		c.HiddenAccounts = map[string]bool{}
	}
	if c.RingTints == nil {
		c.RingTints = map[string]string{}
	}
	if c.RailRatio < 0 || c.RailRatio > 1 {
		c.RailRatio = def.RailRatio
	}
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "magpie-pulse-config.json"
	}
	return filepath.Join(home, ".config", "magpie-pulse", "config.json")
}

func LoadConfig() Config {
	cfg := DefaultConfig()
	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return DefaultConfig()
	}
	cfg.Normalize()
	return cfg
}

func SaveConfig(cfg Config) error {
	cfg.Normalize()
	p := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
