package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildRailSlotsAndPools(t *testing.T) {
	now := time.Date(2026, 11, 13, 14, 0, 0, 0, time.UTC)
	accounts := []AccountQuota{
		{
			Provider: "antigravity",
			Name:     "Antigravity",
			User:     "user@example.com",
			Windows: []QuotaWindow{
				{Name: "Gemini · 7 days", Used: 25, Pool: "Gemini", ResetsAt: "2026-11-20T01:02:33Z"},
				{Name: "Gemini · 5 hours", Used: 35, Pool: "Gemini", ResetsAt: "2026-11-13T16:02:33Z"},
				{Name: "Claude & GPT · 7 days", Used: 0, Pool: "Claude & GPT", ResetsAt: "2026-11-20T14:29:39Z"},
				{Name: "Claude & GPT · 5 hours", Used: 80, Pool: "Claude & GPT", ResetsAt: "2026-11-13T19:29:39Z"},
			},
		},
		{
			Provider: "zcode",
			Name:     "ZCode",
			User:     "ZCode",
			Error:    "API KEY missing",
		},
	}

	cfg := DefaultConfig()
	cfg.SplitPools = true
	slots := BuildRailSlots(accounts, cfg, now)
	if len(slots) != 3 {
		t.Fatalf("expected 3 slots with SplitPools=true, got %d", len(slots))
	}
	if slots[0].Pool != "Gemini" || slots[0].Primary == nil || slots[0].Primary.Used != 35 {
		t.Fatalf("unexpected Gemini slot: %+v", slots[0])
	}
	if slots[0].Second == nil || slots[0].Second.Used != 25 {
		t.Fatalf("expected Gemini second ring 25, got %+v", slots[0].Second)
	}
	if slots[1].Pool != "Claude & GPT" || slots[1].Primary == nil || slots[1].Primary.Used != 80 {
		t.Fatalf("unexpected Claude & GPT slot: %+v", slots[1])
	}
	if !slots[2].HasError || !slots[2].IsSpent {
		t.Fatalf("expected zcode slot to be error+spent: %+v", slots[2])
	}

	cfg.SplitPools = false
	unsplit := BuildRailSlots(accounts, cfg, now)
	if len(unsplit) != 2 {
		t.Fatalf("expected 2 slots with SplitPools=false, got %d", len(unsplit))
	}
	if unsplit[0].Primary == nil || unsplit[0].Primary.Used != 80 {
		t.Fatalf("expected unsplit antigravity primary to pick max 80, got %+v", unsplit[0].Primary)
	}
}

func TestCycleOrderAndRingMapping(t *testing.T) {
	now := time.Date(2026, 11, 13, 14, 0, 0, 0, time.UTC)
	accounts := []AccountQuota{
		{
			Provider: "claude",
			Name:     "Claude",
			User:     "short-vs-long@example.com",
			// Intentionally put 7 days first AND with higher usage than 5 hours.
			Windows: []QuotaWindow{
				{Name: "7 days", Used: 65, ResetsAt: "2026-11-20T01:02:33Z"},
				{Name: "5 hours", Used: 12, ResetsAt: "2026-11-13T16:02:33Z"},
			},
		},
		{
			Provider: "zcode",
			Name:     "ZCode",
			User:     "z@example.com",
			Windows: []QuotaWindow{
				{Name: "MCP · Month", Used: 0, SpanNanos: int64(30 * 24 * time.Hour), Aside: true},
				{Name: "Weekly", Used: 63, SpanNanos: int64(7 * 24 * time.Hour)},
				{Name: "5 hours", Used: 16, SpanNanos: int64(5 * time.Hour)},
			},
		},
		{
			Provider: "custom",
			Name:     "Custom",
			User:     "5h-vs-7h@example.com",
			Windows: []QuotaWindow{
				{Name: "7 hours", Used: 80},
				{Name: "5 hours", Used: 20},
			},
		},
	}

	slots := BuildRailSlots(accounts, DefaultConfig(), now)
	if len(slots) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(slots))
	}

	// Outer ring = 5 hours (12%), inner ring = 7 days (65%).
	if slots[0].Primary == nil || slots[0].Primary.Name != "5 hours" || slots[0].Primary.Used != 12 {
		t.Fatalf("expected Claude outer ring to be 5 hours (12%%), got %+v", slots[0].Primary)
	}
	if slots[0].Second == nil || slots[0].Second.Name != "7 days" || slots[0].Second.Used != 65 {
		t.Fatalf("expected Claude inner ring to be 7 days (65%%), got %+v", slots[0].Second)
	}

	// ZCode: outer ring = 5 hours (16%), inner ring = Weekly (63%, preferring non-aside over 30d aside).
	if slots[1].Primary == nil || slots[1].Primary.Name != "5 hours" {
		t.Fatalf("expected ZCode outer ring to be 5 hours, got %+v", slots[1].Primary)
	}
	if slots[1].Second == nil || slots[1].Second.Name != "Weekly" {
		t.Fatalf("expected ZCode inner ring to be Weekly, got %+v", slots[1].Second)
	}

	// 5 hours vs 7 hours: outer = 5 hours, inner = 7 hours.
	if slots[2].Primary == nil || slots[2].Primary.Name != "5 hours" {
		t.Fatalf("expected 5 hours outer ring, got %+v", slots[2].Primary)
	}
	if slots[2].Second == nil || slots[2].Second.Name != "7 hours" {
		t.Fatalf("expected 7 hours inner ring, got %+v", slots[2].Second)
	}

	sorted := SortWindowsByCycle(accounts[1].Windows)
	if sorted[0].Name != "5 hours" || sorted[1].Name != "Weekly" || sorted[2].Name != "MCP · Month" {
		t.Fatalf("unexpected sorted window order: %+v", sorted)
	}
}

func TestUsageTintAndMetrics(t *testing.T) {
	if UsageTintColor(0.2, false, 75, false) != PulseGoodDark {
		t.Fatal("expected PulseGoodDark at 20%")
	}
	if UsageTintColor(0.6, false, 75, false) != PulseCautionDark {
		t.Fatal("expected PulseCautionDark at 60%")
	}
	if UsageTintColor(0.8, false, 75, false) != PulseWarningDark {
		t.Fatal("expected PulseWarningDark at 80%")
	}
	if UsageTintColor(1.0, true, 75, false) != PulseExhaustedDark {
		t.Fatal("expected PulseExhaustedDark when spent")
	}
	if UsageTintColor(0.2, false, 75, true) != PulseGoodLight {
		t.Fatal("expected PulseGoodLight in light mode")
	}

	cfg := DefaultConfig()
	m := ComputeMetrics(cfg)
	if m.CornerRadius+m.FlareWidth != m.Width {
		t.Fatalf("expected corner+flare == width, got %v + %v != %v", m.CornerRadius, m.FlareWidth, m.Width)
	}
	cfg.RoundEnds = true
	mr := ComputeMetrics(cfg)
	if mr.CornerRadius+mr.FlareWidth != mr.Width {
		t.Fatalf("roundEnds: expected corner+flare == width, got %v + %v != %v", mr.CornerRadius, mr.FlareWidth, mr.Width)
	}
}

func TestConfigLanguageDefaultsAndFormatting(t *testing.T) {
	def := DefaultConfig()
	if def.Language != LangAuto {
		t.Fatalf("DefaultConfig().Language = %q, want %q", def.Language, LangAuto)
	}

	cases := map[Language]Language{
		"":        LangAuto,
		"auto":    LangAuto,
		"AUTO":    LangAuto,
		"zh":      LangZhHans,
		"zh-CN":   LangZhHans,
		"zh_CN":   LangZhHans,
		"zh-Hans": LangZhHans,
		"en":      LangEn,
		"en-US":   LangEn,
		"fr-FR":   LangAuto,
	}
	for in, want := range cases {
		if got := NormalizeLanguage(in); got != want {
			t.Errorf("NormalizeLanguage(%q) = %q, want %q", in, got, want)
		}
	}

	cfg := DefaultConfig()
	cfg.Language = "zh-CN"
	cfg.Normalize()
	if cfg.Language != LangZhHans {
		t.Fatalf("Config.Normalize() Language = %q, want %q", cfg.Language, LangZhHans)
	}

	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	if got := FormatRelativeTimeLocale("zh-Hans", now, now.Add(-20*time.Second)); got != "刚刚" {
		t.Fatalf("zh relative just now = %q", got)
	}
	if got := FormatRelativeTimeLocale("zh-Hans", now, now.Add(-5*time.Minute)); got != "5 分钟前" {
		t.Fatalf("zh relative 5m = %q", got)
	}
	if got := FormatRelativeTimeLocale("en", now, now.Add(-5*time.Minute)); got != "5 min ago" {
		t.Fatalf("en relative 5m = %q", got)
	}
	if got := FormatResetCountdownLocale("zh-Hans", now, now.Add(-time.Second)); got != "即将重置" {
		t.Fatalf("zh reset soon = %q", got)
	}
	if got := FormatResetCountdownLocale("zh-Hans", now, now.Add(25*time.Minute)); got != "25 分钟后重置" {
		t.Fatalf("zh reset 25m = %q", got)
	}
	if got := FormatResetCountdownLocale("en", now, now.Add(25*time.Minute)); got != "Resets in 25m" {
		t.Fatalf("en reset 25m = %q", got)
	}
	if got := (AccountQuota{}).DisplayNameLocale("zh-Hans"); got != "未知" {
		t.Fatalf("zh unknown display name = %q", got)
	}
	if got := (AccountQuota{}).DisplayNameLocale("en"); got != "Unknown" {
		t.Fatalf("en unknown display name = %q", got)
	}
}

func TestOpenAtLoginConfigDefaultsAndPersistence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if !DefaultConfig().OpenAtLogin {
		t.Fatal("expected DefaultConfig().OpenAtLogin to be true")
	}
	if got := LoadConfig(); !got.OpenAtLogin {
		t.Fatal("expected LoadConfig() with no file to default OpenAtLogin to true")
	}

	// Existing config files that predate OpenAtLogin should also default it to true.
	cfgPath := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(`{"magpieUrl":"http://127.0.0.1:3425"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := LoadConfig(); !got.OpenAtLogin {
		t.Fatal("expected LoadConfig() with legacy config missing openAtLogin to default to true")
	}

	cfg := LoadConfig()
	cfg.OpenAtLogin = false
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	reloaded := LoadConfig()
	if reloaded.OpenAtLogin {
		t.Fatal("expected OpenAtLogin=false to persist across SaveConfig/LoadConfig")
	}
}
