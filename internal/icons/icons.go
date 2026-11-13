package icons

import (
	"bytes"
	"embed"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

//go:embed svg/*.svg
var svgFS embed.FS

var (
	providerSVGs = map[string]*ui.SVG{}
	fallbackSVG  *ui.SVG
	SettingsIcon *ui.SVG
	RefreshIcon  *ui.SVG
	GeneralIcon  *ui.SVG
	PaletteIcon  *ui.SVG
	RingsIcon    *ui.SVG
	AccountsIcon *ui.SVG
	AlertIcon    *ui.SVG
)

func init() {
	entries, err := svgFS.ReadDir("svg")
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".svg") {
				continue
			}
			raw, readErr := svgFS.ReadFile("svg/" + entry.Name())
			if readErr != nil {
				continue
			}
			parsed, parseErr := ui.ParseSVG(raw)
			if parseErr != nil {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".svg")
			providerSVGs[name] = parsed
		}
	}

	fallbackSVG = providerSVGs["extension"]
	if fallbackSVG == nil {
		fallbackSVG = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="8"/><circle cx="12" cy="12" r="3" fill="currentColor"/></svg>`))
	}

	SettingsIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9c.26.6.82 1 1.48 1H21a2 2 0 0 1 0 4h-.09c-.66 0-1.22.4-1.51 1z"/></svg>`))
	RefreshIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-2.64-6.36"/><polyline points="21 3 21 9 15 9"/></svg>`))
	GeneralIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="3"/><path d="M8 9h8"/><path d="M8 14h5"/></svg>`))
	PaletteIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="4"/><circle cx="8.5" cy="8.5" r="1.5" fill="currentColor"/><circle cx="15.5" cy="8.5" r="1.5" fill="currentColor"/><path d="M8 15h8"/></svg>`))
	RingsIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><circle cx="12" cy="12" r="8" stroke-opacity="0.35"/><path d="M12 4a8 8 0 0 1 7.5 10.7"/></svg>`))
	AccountsIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="4"/><path d="M5 20a7 7 0 0 1 14 0"/></svg>`))
	AlertIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 8v5"/><circle cx="12" cy="16.5" r="0.8" fill="currentColor"/></svg>`))
}

var providerAlias = map[string]string{
	"antigravity":   "antigravity",
	"claude":        "claude",
	"claude-code":   "claude",
	"codex":         "openai",
	"openai":        "openai",
	"gemini":        "gemini",
	"gemini-web":    "gemini",
	"ai-studio":     "gemini",
	"gemini-cli":    "geminicli",
	"copilot":       "github",
	"github":        "github",
	"zcode":         "zai",
	"z.ai":          "zai",
	"zai":           "zai",
	"kiro":          "kiro",
	"deepseek":      "deepseek",
	"kimi":          "kimi",
	"minimax":       "minimax",
	"qwen":          "qwen",
	"stepfun":       "stepfun",
	"sub2api":       "sub2api",
	"qoder":         "qoder",
	"cursor":        "cursor",
	"windsurf":      "windsurf",
	"grok":          "grok",
	"openclaw":      "openclaw",
	"opencode":      "opencode",
	"remote-magpie": "extension",
}

// ForProvider returns the monochrome/vector SVG mark for a Magpie provider ID.
func ForProvider(provider string) *ui.SVG {
	key := strings.ToLower(strings.TrimSpace(provider))
	if mapped, ok := providerAlias[key]; ok {
		if svg, exists := providerSVGs[mapped]; exists {
			return svg
		}
	}
	if svg, exists := providerSVGs[key]; exists {
		return svg
	}
	return fallbackSVG
}

// TrayIconPNG draws a crisp 36x36 template ring icon for the macOS menu bar.
func TrayIconPNG(alert bool) []byte {
	const size = 36
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size-1)/2.0, float64(size-1)/2.0
	outerR := 13.0
	innerR := 8.8
	dotR := 3.8
	ink := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	if alert {
		ink = color.NRGBA{R: 255, G: 79, B: 66, A: 255}
	}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Hypot(dx, dy)
			angle := math.Atan2(dy, dx) // -pi..pi, 0 at right, -pi/2 at top
			if angle < -math.Pi/2 {
				angle += 2 * math.Pi
			}
			// Draw outer 270-degree pulse arc starting from 12 o'clock (-pi/2)
			arcCover := 0.0
			if dist <= outerR+0.8 && dist >= innerR-0.8 {
				if dist > outerR {
					arcCover = math.Max(0, 1-(dist-outerR)/0.8)
				} else if dist < innerR {
					arcCover = math.Max(0, 1-(innerR-dist)/0.8)
				} else {
					arcCover = 1.0
				}
				// Leave a clean notch in top-left quadrant (-pi/2 - 0.85 .. -pi/2)
				rel := angle - (-math.Pi / 2)
				if rel > 1.48*math.Pi {
					arcCover *= 0.25
				}
			}
			dotCover := 0.0
			if dist <= dotR+0.8 {
				if dist > dotR {
					dotCover = math.Max(0, 1-(dist-dotR)/0.8)
				} else {
					dotCover = 1.0
				}
			}
			cov := math.Max(arcCover, dotCover)
			if cov > 0 {
				c := ink
				c.A = uint8(math.Round(float64(ink.A) * cov))
				img.SetNRGBA(x, y, c)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// AppIconPNG draws a 128x128 dark rounded app icon with a glowing green/emerald pulse ring.
func AppIconPNG() []byte {
	const size = 128
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size-1)/2.0, float64(size-1)/2.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Squircle-like rounded background
			nx := math.Abs(float64(x)-cx) / (size/2.0 - 4)
			ny := math.Abs(float64(y)-cy) / (size/2.0 - 4)
			if math.Pow(nx, 4.5)+math.Pow(ny, 4.5) > 1.0 {
				continue
			}
			bg := color.NRGBA{R: 16, G: 18, B: 22, A: 255}
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Hypot(dx, dy)
			if dist >= 30 && dist <= 40 {
				angle := math.Atan2(dy, dx) + math.Pi/2
				if angle < 0 {
					angle += 2 * math.Pi
				}
				if angle <= 1.55*math.Pi {
					bg = color.NRGBA{R: 0, G: 230, B: 140, A: 255}
				} else {
					bg = color.NRGBA{R: 48, G: 52, B: 60, A: 255}
				}
			} else if dist <= 13 {
				bg = color.NRGBA{R: 245, G: 247, B: 250, A: 255}
			}
			img.SetNRGBA(x, y, bg)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
