package settings

import (
	"testing"
	"time"

	"magpie-pulse/internal/model"

	"github.com/egoist/mygo/ui"
)

func newTestState() (*SettingsState, *[]model.Config) {
	var configs []model.Config
	cfg := model.DefaultConfig()
	st := &SettingsState{
		ActiveTab: TabGeneral,
		Config:    cfg,
		Snapshot: model.Snapshot{
			Connected:      true,
			MagpieVersion:  "0.5.0",
			MagpieModelCnt: 12,
			Accounts: []model.AccountQuota{
				{
					Provider: "claude",
					Name:     "Claude",
					Plan:     "Max",
					User:     "alice@example.com",
					Windows: []model.QuotaWindow{
						{Name: "5 hours", Used: 42, Remaining: 58},
						{Name: "7 days", Used: 18, Remaining: 82},
					},
				},
				{
					Provider:   "codex",
					Name:       "Codex",
					Plan:       "Pro",
					User:       "bob@example.com",
					CachedOnly: true,
					Windows: []model.QuotaWindow{
						{Name: "5 hours", Used: 85, Remaining: 15},
					},
				},
				{
					Provider: "antigravity",
					Name:     "Antigravity",
					User:     "carol@example.com",
					Error:    "token expired",
				},
			},
		},
		OnConfigChange: func(c model.Config) {
			configs = append(configs, c)
		},
	}
	return st, &configs
}

func TestSettingsTabsAndControls(t *testing.T) {
	st, configs := newTestState()

	var testedURL string
	var refreshCalls int
	var showPanelCalls int
	var checkUpdateCalls int
	var autoUpdateValues []bool
	st.AppVersion = "0.1.0"
	st.AutoCheckUpdates = true
	st.OnTestConnection = func(url string) {
		testedURL = url
	}
	st.OnRefreshNow = func() {
		refreshCalls++
	}
	st.OnShowPanel = func() {
		showPanelCalls++
	}
	st.OnCheckUpdates = func() {
		checkUpdateCalls++
	}
	st.OnToggleAutoUpdates = func(on bool) {
		autoUpdateValues = append(autoUpdateValues, on)
	}

	tt := ui.NewTester(func(c *ui.Context) {
		SettingsView(c, st)
	}, 860, 640)

	if !tt.HasText("Magpie API 地址") {
		t.Fatalf("expected general tab content, got texts %v", tt.Texts())
	}
	if !tt.HasText("Connected · v0.5.0 · 12 models") {
		t.Fatalf("expected connection status text, got texts %v", tt.Texts())
	}

	if err := tt.Click("测试连接"); err != nil {
		t.Fatalf("click 测试连接 failed: %v", err)
	}
	if testedURL != st.Config.MagpieURL {
		t.Fatalf("expected testedURL %q, got %q", st.Config.MagpieURL, testedURL)
	}

	if err := tt.Click("显示悬浮面板"); err != nil {
		t.Fatalf("click 显示悬浮面板 failed: %v", err)
	}
	if showPanelCalls != 1 {
		t.Fatalf("expected showPanelCalls=1, got %d", showPanelCalls)
	}

	if err := tt.Click("60s"); err != nil {
		t.Fatalf("click 60s failed: %v", err)
	}
	if st.Config.RefreshIntervalSecs != 60 {
		t.Fatalf("expected RefreshIntervalSecs=60, got %d", st.Config.RefreshIntervalSecs)
	}
	if len(*configs) == 0 || (*configs)[len(*configs)-1].RefreshIntervalSecs != 60 {
		t.Fatalf("expected OnConfigChange with RefreshIntervalSecs=60")
	}

	tt.Wheel(430, 320, 0, 240)
	tt.Advance(600 * time.Millisecond)

	if err := tt.Click("检查更新"); err != nil {
		t.Fatalf("click 检查更新 failed: %v", err)
	}
	if checkUpdateCalls != 1 {
		t.Fatalf("expected checkUpdateCalls=1, got %d", checkUpdateCalls)
	}

	if err := tt.Click("自动检查更新"); err != nil {
		t.Fatalf("click 自动检查更新 failed: %v", err)
	}
	if st.AutoCheckUpdates || len(autoUpdateValues) == 0 || autoUpdateValues[len(autoUpdateValues)-1] {
		t.Fatalf("expected AutoCheckUpdates=false after toggle, got %v (%v)", st.AutoCheckUpdates, autoUpdateValues)
	}

	// Switch to Appearance tab and toggle AutoCollapse.
	if err := tt.Click("外观与面板"); err != nil {
		t.Fatalf("click 外观与面板 failed: %v", err)
	}
	if st.ActiveTab != TabAppearance {
		t.Fatalf("expected ActiveTab=%q, got %q", TabAppearance, st.ActiveTab)
	}
	if !tt.HasText("边缘自动收起") {
		t.Fatalf("expected appearance tab content, got %v", tt.Texts())
	}

	if err := tt.Click("边缘自动收起"); err != nil {
		t.Fatalf("click 边缘自动收起 failed: %v", err)
	}
	if !st.Config.AutoCollapse {
		t.Fatalf("expected AutoCollapse=true after toggle")
	}
	if (*configs)[len(*configs)-1].AutoCollapse != true {
		t.Fatalf("expected OnConfigChange with AutoCollapse=true")
	}

	if err := tt.Click("液态玻璃"); err != nil {
		t.Fatalf("click 液态玻璃 failed: %v", err)
	}
	if st.Config.Surface != model.SurfaceGlass {
		t.Fatalf("expected Surface=%q, got %q", model.SurfaceGlass, st.Config.Surface)
	}

	// Switch to Rings tab and toggle ShowRemaining and SplitPools.
	if err := tt.Click("圆环与额度"); err != nil {
		t.Fatalf("click 圆环与额度 failed: %v", err)
	}
	if st.ActiveTab != TabRings {
		t.Fatalf("expected ActiveTab=%q, got %q", TabRings, st.ActiveTab)
	}

	if err := tt.Click("显示剩余百分比而非已用"); err != nil {
		t.Fatalf("click ShowRemaining failed: %v", err)
	}
	if !st.Config.ShowRemaining {
		t.Fatalf("expected ShowRemaining=true")
	}
	if !(*configs)[len(*configs)-1].ShowRemaining {
		t.Fatalf("expected OnConfigChange with ShowRemaining=true")
	}

	if err := tt.Click("按模型池拆分多池账号（如 Antigravity）"); err != nil {
		t.Fatalf("click SplitPools failed: %v", err)
	}
	if st.Config.SplitPools {
		t.Fatalf("expected SplitPools=false after toggle")
	}
	if (*configs)[len(*configs)-1].SplitPools {
		t.Fatalf("expected OnConfigChange with SplitPools=false")
	}

	if err := tt.Click("85%"); err != nil {
		t.Fatalf("click 85%% failed: %v", err)
	}
	if st.Config.WarningThreshold != 85 {
		t.Fatalf("expected WarningThreshold=85, got %d", st.Config.WarningThreshold)
	}

	// Switch to Accounts tab and test refresh, visibility switch, and ring tint preset.
	if err := tt.Click("账号总览"); err != nil {
		t.Fatalf("click 账号总览 failed: %v", err)
	}
	if st.ActiveTab != TabAccounts {
		t.Fatalf("expected ActiveTab=%q, got %q", TabAccounts, st.ActiveTab)
	}
	if !tt.HasText("alice@example.com") || !tt.HasText("token expired") {
		t.Fatalf("expected account details in accounts pane, got %v", tt.Texts())
	}
	if !tt.HasText("在线") || !tt.HasText("本地缓存") || !tt.HasText("异常") {
		t.Fatalf("expected status badges, got %v", tt.Texts())
	}

	if err := tt.Click("立即刷新"); err != nil {
		t.Fatalf("click 立即刷新 failed: %v", err)
	}
	if refreshCalls != 1 {
		t.Fatalf("expected refreshCalls=1, got %d", refreshCalls)
	}

	firstKey := st.Snapshot.Accounts[0].AccountKey()
	if err := tt.Click("在面板中显示"); err != nil {
		t.Fatalf("click 在面板中显示 failed: %v", err)
	}
	if !st.Config.HiddenAccounts[firstKey] {
		t.Fatalf("expected HiddenAccounts[%q]=true after toggle, got %v", firstKey, st.Config.HiddenAccounts)
	}
	if !(*configs)[len(*configs)-1].HiddenAccounts[firstKey] {
		t.Fatalf("expected OnConfigChange with HiddenAccounts[%q]=true", firstKey)
	}

	if err := tt.Click("在面板中显示"); err != nil {
		t.Fatalf("second click 在面板中显示 failed: %v", err)
	}
	if st.Config.HiddenAccounts[firstKey] {
		t.Fatalf("expected HiddenAccounts[%q]=false after second toggle", firstKey)
	}

	if err := tt.Click("Azure #4099FF"); err != nil {
		t.Fatalf("click ring tint preset failed: %v", err)
	}
	if st.Config.RingTints[firstKey] != "#4099FF" {
		t.Fatalf("expected RingTints[%q]=#4099FF, got %q", firstKey, st.Config.RingTints[firstKey])
	}

	if err := tt.Click("自动(按用量)"); err != nil {
		t.Fatalf("click 自动(按用量) failed: %v", err)
	}
	if _, exists := st.Config.RingTints[firstKey]; exists {
		t.Fatalf("expected RingTints[%q] cleared", firstKey)
	}
}

func TestSettingsVibrancyAndEmptyAccounts(t *testing.T) {
	st := &SettingsState{
		ActiveTab: TabAccounts,
		Config:    model.DefaultConfig(),
		Snapshot:  model.Snapshot{},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		SettingsView(c, st)
	}, 800, 560)
	tt.SetVibrancy(true)
	tt.Frame()
	if !tt.HasText("暂无账号数据") {
		t.Fatalf("expected empty accounts state, got %v", tt.Texts())
	}
}
