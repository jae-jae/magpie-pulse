package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RoutedUsage struct {
	Calls     int64   `json:"calls"`
	Tokens    int64   `json:"tokens"`
	CacheRead int64   `json:"cacheRead"`
	Cost      float64 `json:"cost"`
}

type WindowHolds struct {
	Tokens int64        `json:"tokens"`
	Cost   float64      `json:"cost"`
	Priced bool         `json:"priced"`
	Used   float64      `json:"used"`
	Since  string       `json:"since"`
	Routed *RoutedUsage `json:"routed,omitempty"`
}

type QuotaWindow struct {
	Name      string       `json:"name"`
	Used      float64      `json:"used"`
	Remaining float64      `json:"remaining"`
	ResetsAt  string       `json:"resetsAt,omitempty"`
	ResetSecs int64        `json:"resetSecs,omitempty"`
	Pool      string       `json:"pool,omitempty"`
	Family    string       `json:"family,omitempty"`
	Display   string       `json:"display,omitempty"`
	Limit     float64      `json:"limit,omitempty"`
	IsText    bool         `json:"text,omitempty"`
	Holds     *WindowHolds `json:"holds,omitempty"`
	HoldsWhy  string       `json:"holdsWhy,omitempty"`
	SpanNanos int64        `json:"-"`
	Aside     bool         `json:"-"`
}

// UsedFraction returns the clamped 0..1 fraction of quota used.
func (w QuotaWindow) UsedFraction() float64 {
	f := w.Used / 100.0
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// RemainingFraction returns the clamped 0..1 fraction remaining.
func (w QuotaWindow) RemainingFraction() float64 {
	return 1.0 - w.UsedFraction()
}

// IsSpent reports whether this window has reached 100%.
func (w QuotaWindow) IsSpent() bool {
	return !w.IsText && w.Used >= 100
}

// ResetTime parses ResetsAt if valid RFC3339.
func (w QuotaWindow) ResetTime() (time.Time, bool) {
	if w.ResetsAt == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, w.ResetsAt)
	if err != nil {
		t, err = time.Parse(time.RFC3339, w.ResetsAt)
		if err != nil {
			return time.Time{}, false
		}
	}
	return t, true
}

// InferSpanDuration estimates the window duration from SpanNanos or its name.
func (w QuotaWindow) InferSpanDuration() time.Duration {
	if w.SpanNanos > 0 {
		return time.Duration(w.SpanNanos)
	}
	return parseWindowDuration(w.Name)
}

func parseWindowDuration(name string) time.Duration {
	low := strings.ToLower(strings.TrimSpace(name))
	if low == "" {
		return 0
	}

	if d := parseNumberedDuration(low); d > 0 {
		return d
	}

	switch {
	case strings.Contains(low, "hourly"):
		return time.Hour
	case strings.Contains(low, "daily"):
		return 24 * time.Hour
	case strings.Contains(low, "weekly"):
		return 7 * 24 * time.Hour
	case strings.Contains(low, "monthly") || strings.Contains(low, "month"):
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}

func parseNumberedDuration(low string) time.Duration {
	for i := 0; i < len(low); i++ {
		if low[i] < '0' || low[i] > '9' {
			continue
		}
		if i > 0 && isAlphaNum(low[i-1]) {
			continue
		}
		j := i
		for j < len(low) && low[j] >= '0' && low[j] <= '9' {
			j++
		}
		n, err := strconv.Atoi(low[i:j])
		if err != nil || n <= 0 {
			i = j
			continue
		}
		rest := strings.TrimLeft(low[j:], " ")
		unitLen := 0
		for unitLen < len(rest) && rest[unitLen] >= 'a' && rest[unitLen] <= 'z' {
			unitLen++
		}
		unit := rest[:unitLen]
		switch unit {
		case "m", "min", "mins", "minute", "minutes":
			return time.Duration(n) * time.Minute
		case "h", "hr", "hrs", "hour", "hours":
			return time.Duration(n) * time.Hour
		case "d", "day", "days":
			return time.Duration(n) * 24 * time.Hour
		case "w", "wk", "wks", "week", "weeks":
			return time.Duration(n) * 7 * 24 * time.Hour
		case "mo", "mon", "month", "months":
			return time.Duration(n) * 30 * 24 * time.Hour
		}
		i = j
	}
	return 0
}

func isAlphaNum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// SortWindowsByCycle returns a copy of wins sorted from shortest cycle to longest cycle.
// Windows with an explicit cycle duration precede windows without a known duration.
func SortWindowsByCycle(wins []QuotaWindow) []QuotaWindow {
	if len(wins) <= 1 {
		out := make([]QuotaWindow, len(wins))
		copy(out, wins)
		return out
	}
	out := make([]QuotaWindow, len(wins))
	copy(out, wins)
	sort.SliceStable(out, func(i, j int) bool {
		di := out[i].InferSpanDuration()
		dj := out[j].InferSpanDuration()
		if di > 0 && dj > 0 {
			if di != dj {
				return di < dj
			}
			if out[i].Aside != out[j].Aside {
				return !out[i].Aside
			}
			return false
		}
		if (di > 0) != (dj > 0) {
			return di > 0
		}
		return false
	})
	return out
}

// ClockFraction returns 0..1 progress through the current reset window if ResetsAt and duration are known.
func (w QuotaWindow) ClockFraction(now time.Time) (float64, bool) {
	resetAt, ok := w.ResetTime()
	if !ok {
		return 0, false
	}
	dur := w.InferSpanDuration()
	if dur <= 0 {
		return 0, false
	}
	rem := resetAt.Sub(now)
	if rem <= 0 {
		return 1, true
	}
	elapsed := 1.0 - float64(rem)/float64(dur)
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > 1 {
		elapsed = 1
	}
	return elapsed, true
}

type AccountQuota struct {
	ID           string        `json:"id,omitempty"`
	Provider     string        `json:"provider"`
	Name         string        `json:"name"`
	Kind         string        `json:"kind,omitempty"`
	Plan         string        `json:"plan,omitempty"`
	User         string        `json:"user,omitempty"`
	Icon         string        `json:"icon,omitempty"`
	Windows      []QuotaWindow `json:"windows"`
	Error        string        `json:"error,omitempty"`
	ReadAt       string        `json:"readAt,omitempty"`
	LastServedAt string        `json:"lastServedAt,omitempty"`
	Last         bool          `json:"last,omitempty"`
	AsOf         string        `json:"asOf,omitempty"`
	Until        string        `json:"until,omitempty"`
	Renew        string        `json:"renew,omitempty"`
	CachedOnly   bool          `json:"cachedOnly,omitempty"`
	Disabled     bool          `json:"disabled,omitempty"`
}

// AccountKey returns a stable identifier like "provider/user".
func (a AccountQuota) AccountKey() string {
	if a.ID != "" {
		return a.ID
	}
	user := strings.TrimSpace(a.User)
	if user == "" {
		user = "default"
	}
	return strings.ToLower(a.Provider) + "/" + strings.ToLower(user)
}

// DisplayName returns the friendly provider title.
func (a AccountQuota) DisplayName() string {
	if strings.TrimSpace(a.Name) != "" {
		return a.Name
	}
	switch strings.ToLower(a.Provider) {
	case "claude":
		return "Claude"
	case "codex":
		return "Codex"
	case "antigravity":
		return "Antigravity"
	case "gemini-web":
		return "Gemini Web"
	case "ai-studio":
		return "Google AI Studio"
	case "zcode":
		return "ZCode"
	case "copilot":
		return "GitHub Copilot"
	case "kiro":
		return "Kiro"
	default:
		if a.Provider == "" {
			return "Unknown"
		}
		return strings.ToUpper(a.Provider[:1]) + a.Provider[1:]
	}
}

// ReadTime parses ReadAt.
func (a AccountQuota) ReadTime() (time.Time, bool) {
	if a.ReadAt == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, a.ReadAt)
	if err != nil {
		t, err = time.Parse(time.RFC3339, a.ReadAt)
		if err != nil {
			return time.Time{}, false
		}
	}
	return t, true
}

type HistoryPoint struct {
	At       string  `json:"at"`
	Left     float64 `json:"left"`
	Start    string  `json:"start,omitempty"`
	ResetsAt string  `json:"resetsAt,omitempty"`
}

type HistoryLine struct {
	Name   string         `json:"name"`
	Points []HistoryPoint `json:"points"`
}

type AccountHistory struct {
	Provider string        `json:"provider"`
	User     string        `json:"user"`
	Lines    []HistoryLine `json:"lines"`
}

type UsageDay struct {
	Label      string  `json:"label"`
	Time       string  `json:"time"`
	Calls      int64   `json:"calls"`
	Errors     int64   `json:"errors"`
	Input      int64   `json:"input"`
	Output     int64   `json:"output"`
	CacheRead  int64   `json:"cache_read"`
	CacheWrite int64   `json:"cache_write"`
	Reasoning  int64   `json:"reasoning"`
	Cost       float64 `json:"cost"`
}

type UsageSummary struct {
	Period     string     `json:"period"`
	Since      string     `json:"since"`
	Calls      int64      `json:"calls"`
	Errors     int64      `json:"errors"`
	Input      int64      `json:"input"`
	Output     int64      `json:"output"`
	CacheRead  int64      `json:"cache_read"`
	CacheWrite int64      `json:"cache_write"`
	Reasoning  int64      `json:"reasoning"`
	Cost       float64    `json:"cost"`
	Series     []UsageDay `json:"series"`
}

// TotalTokens returns input + output + cache_read + cache_write + reasoning.
func (u UsageSummary) TotalTokens() int64 {
	return u.Input + u.Output + u.CacheRead + u.CacheWrite + u.Reasoning
}

// FormatShortTokens formats large token counts like 41.8M or 1.2B.
func FormatShortTokens(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// FormatRelativeTime formats how long ago t occurred.
func FormatRelativeTime(now, t time.Time) string {
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// FormatResetCountdown formats the reset timestamp concisely.
func FormatResetCountdown(now, reset time.Time) string {
	d := reset.Sub(now)
	if d <= 0 {
		return "Resetting soon"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	local := reset.Local()
	switch {
	case days >= 1:
		return fmt.Sprintf("Resets %s (%dd %dh)", local.Format("Jan 2 15:04"), days, hours)
	case hours >= 1:
		return fmt.Sprintf("Resets %s (%dh %dm)", local.Format("15:04"), hours, mins)
	default:
		return fmt.Sprintf("Resets in %dm", max(mins, 1))
	}
}

// Snapshot is the aggregated live state fetched from Magpie.
type Snapshot struct {
	Accounts       []AccountQuota            `json:"accounts"`
	Histories      map[string]AccountHistory `json:"histories"`
	Usage30d       *UsageSummary             `json:"usage30d,omitempty"`
	Usage7d        *UsageSummary             `json:"usage7d,omitempty"`
	UsageToday     *UsageSummary             `json:"usageToday,omitempty"`
	StatusLine     string                    `json:"statusLine,omitempty"`
	FetchedAt      time.Time                 `json:"fetchedAt"`
	Connected      bool                      `json:"connected"`
	ConnectionErr  string                    `json:"connectionErr,omitempty"`
	MagpieVersion  string                    `json:"magpieVersion,omitempty"`
	MagpieModelCnt int                       `json:"magpieModelCnt,omitempty"`
}
