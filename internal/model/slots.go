package model

import (
	"sort"
	"strings"
	"time"
)

// RailSlot represents one ring on the Pulse dock rail.
type RailSlot struct {
	SlotID        string
	Account       AccountQuota
	Pool          string
	Title         string
	Windows       []QuotaWindow
	Primary       *QuotaWindow
	Second        *QuotaWindow
	HasReading    bool
	HasError      bool
	IsSpent       bool
	UsedFraction  *float64
	ClockFraction *float64
}

// BuildRailSlots converts Magpie account quotas into ordered dock rail slots.
func BuildRailSlots(accounts []AccountQuota, cfg Config, now time.Time) []RailSlot {
	var slots []RailSlot
	for _, acc := range accounts {
		key := acc.AccountKey()
		if cfg.HiddenAccounts[key] {
			continue
		}
		pools := extractPools(acc)
		if cfg.SplitPools && len(pools) > 1 {
			for _, pool := range pools {
				var poolWins []QuotaWindow
				for _, w := range acc.Windows {
					if w.Pool == pool {
						poolWins = append(poolWins, w)
					}
				}
				slotID := key + "@" + pool
				title := acc.DisplayName() + " · " + pool
				slots = append(slots, makeSlot(slotID, acc, pool, title, poolWins, now))
			}
			continue
		}
		slots = append(slots, makeSlot(key, acc, "", acc.DisplayName(), acc.Windows, now))
	}
	return slots
}

func extractPools(acc AccountQuota) []string {
	if len(acc.Windows) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var pools []string
	for _, w := range acc.Windows {
		if w.IsText || strings.TrimSpace(w.Pool) == "" {
			return nil
		}
		if !seen[w.Pool] {
			seen[w.Pool] = true
			pools = append(pools, w.Pool)
		}
	}
	// Keep preferred order if Antigravity-like pools exist
	if len(pools) > 1 {
		order := map[string]int{"Gemini": 0, "Claude & GPT": 1, "Gemini 4 Argon": 2}
		sort.SliceStable(pools, func(i, j int) bool {
			oi, oki := order[pools[i]]
			oj, okj := order[pools[j]]
			if oki && okj {
				return oi < oj
			}
			if oki {
				return true
			}
			if okj {
				return false
			}
			return pools[i] < pools[j]
		})
	}
	return pools
}

func makeSlot(slotID string, acc AccountQuota, pool, title string, wins []QuotaWindow, now time.Time) RailSlot {
	slot := RailSlot{
		SlotID:   slotID,
		Account:  acc,
		Pool:     pool,
		Title:    title,
		Windows:  wins,
		HasError: strings.TrimSpace(acc.Error) != "",
	}
	var numeric []QuotaWindow
	for _, w := range wins {
		if !w.IsText {
			numeric = append(numeric, w)
		}
	}
	if len(numeric) == 0 {
		if len(wins) > 0 && !slot.HasError {
			slot.HasReading = true
		}
		if slot.HasError {
			slot.IsSpent = true
		}
		return slot
	}

	slot.HasReading = true
	sorted := SortWindowsByCycle(numeric)

	primaryIdx, secondIdx := selectRingWindows(sorted)
	p := sorted[primaryIdx]
	slot.Primary = &p
	f := p.UsedFraction()
	slot.UsedFraction = &f
	slot.IsSpent = p.IsSpent()
	if cf, ok := p.ClockFraction(now); ok {
		slot.ClockFraction = &cf
	}

	if secondIdx >= 0 && secondIdx < len(sorted) {
		s := sorted[secondIdx]
		slot.Second = &s
	}
	return slot
}

// selectRingWindows maps shortest-cycle windows to the outer (Primary) ring and
// longer-cycle windows to the inner (Second) ring. When all windows share the
// same duration or have unknown durations, it falls back to highest usage.
func selectRingWindows(sorted []QuotaWindow) (primaryIdx int, secondIdx int) {
	if len(sorted) == 0 {
		return -1, -1
	}
	if len(sorted) == 1 {
		return 0, -1
	}

	minDur := sorted[0].InferSpanDuration()
	if minDur <= 0 {
		return selectByMaxUsed(sorted)
	}

	hasLonger := false
	for i := 1; i < len(sorted); i++ {
		if d := sorted[i].InferSpanDuration(); d > minDur {
			hasLonger = true
			break
		}
	}
	if !hasLonger {
		return selectByMaxUsed(sorted)
	}

	// Primary (outer ring): highest used among shortest-cycle windows.
	primaryIdx = 0
	for i := 1; i < len(sorted); i++ {
		if sorted[i].InferSpanDuration() != minDur {
			break
		}
		if sorted[i].Used > sorted[primaryIdx].Used {
			primaryIdx = i
		}
	}

	// Second (inner ring): longest non-aside cycle > minDur (falling back to aside),
	// breaking ties within that cycle by highest used.
	var targetDur time.Duration
	for i := len(sorted) - 1; i >= 0; i-- {
		d := sorted[i].InferSpanDuration()
		if d > minDur && !sorted[i].Aside {
			targetDur = d
			break
		}
	}
	if targetDur == 0 {
		for i := len(sorted) - 1; i >= 0; i-- {
			d := sorted[i].InferSpanDuration()
			if d > minDur {
				targetDur = d
				break
			}
		}
	}

	secondIdx = -1
	for i := range sorted {
		if i == primaryIdx || sorted[i].InferSpanDuration() != targetDur {
			continue
		}
		if secondIdx == -1 || sorted[i].Used > sorted[secondIdx].Used {
			secondIdx = i
		}
	}
	return primaryIdx, secondIdx
}

func selectByMaxUsed(wins []QuotaWindow) (primaryIdx int, secondIdx int) {
	primaryIdx = 0
	for i := 1; i < len(wins); i++ {
		if wins[i].Used > wins[primaryIdx].Used {
			primaryIdx = i
		}
	}
	secondIdx = -1
	for i := range wins {
		if i == primaryIdx {
			continue
		}
		if secondIdx == -1 || wins[i].Used > wins[secondIdx].Used {
			secondIdx = i
		}
	}
	return primaryIdx, secondIdx
}
