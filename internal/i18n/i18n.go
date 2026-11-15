package i18n

import (
	"fmt"
	"strings"

	"magpie-pulse/internal/model"
)

const (
	LocaleEn     = "en"
	LocaleZhHans = "zh-Hans"
)

// Messages holds all user-facing strings for a single locale.
type Messages struct {
	Locale string

	// App & Window
	AppName             string
	SettingsWindowTitle string

	// Tray Menu
	TrayRefreshNow   string
	TrayHidePanel    string
	TrayShowPanel    string
	TrayShowCached   string
	TrayAutoCollapse string
	TraySurface      string
	TraySurfaceDark  string
	TraySurfaceLight string
	TraySurfaceGlass string
	TrayDockPosition string
	TrayDockRight    string
	TrayDockLeft     string
	TrayDockTop      string
	TrayDockBottom   string
	TrayDockFloating string
	TraySettings     string
	TrayQuit         string

	// Notifications
	AlertTitleFmt      string
	AlertBodyUsedFmt   string
	AlertBodyExhausted string

	// Tooltip
	TooltipOffline string

	// Rail
	RailSettingsLabel string

	// Card
	CardUsageSuffixFmt   string
	CardRefreshLabel     string
	CardUpdatedFmt       string
	CardCachedPill       string
	CardPctUsedFmt       string
	CardPctLeftFmt       string
	CardShowAllFmt       string
	CardShowSummary      string
	CardQuotaHistory     string
	CardNamedHistoryFmt  string
	CardSparklineLeftFmt string
	CardToday            string
	Card7Days            string
	Card30Days           string
	CardTokenUnit        string
	CardCallsFmt         string

	// Settings Navigation & Header
	SettingsNavLabel   string
	TabGeneral         string
	TabAppearance      string
	TabRings           string
	TabAccounts        string
	StatusOffline      string
	StatusConnected    string
	StatusError        string
	StatusNotConnected string
	StatusTesting      string

	// Settings - General
	CardMagpieConn       string
	MagpieAPIURL         string
	TestConnection       string
	CardPollingScope     string
	RefreshInterval      string
	RefreshIntervalDesc  string
	IncludeCached        string
	IncludeCachedDesc    string
	CardSystemWindow     string
	Language             string
	LanguageDesc         string
	LanguageAuto         string
	LanguageZhHans       string
	LanguageEn           string
	OpenAtLogin          string
	OpenAtLoginDesc      string
	ShowInDock           string
	ShowInDockDesc       string
	NotifyThreshold      string
	NotifyThresholdDesc  string
	FloatingPanel        string
	FloatingPanelDesc    string
	ShowPanel            string
	CardUpdates          string
	UpdateAvailableFmt   string
	UpdatesDescDefault   string
	LastCheckedFmt       string
	CheckUpdates         string
	AutoCheckUpdates     string
	AutoCheckUpdatesDesc string

	// Settings - Appearance
	CardPanelStyle      string
	SurfaceMode         string
	SurfaceModeDesc     string
	SurfaceDark         string
	SurfaceLight        string
	SurfaceGlass        string
	GlassTransparency   string
	GlassCurrentFmt     string
	DockPosition        string
	DockPositionDesc    string
	DockRight           string
	DockLeft            string
	DockTop             string
	DockBottom          string
	DockFloating        string
	PanelScale          string
	PanelScaleDesc      string
	ScaleSmall          string
	ScaleStandard       string
	ScaleLarge          string
	BarStyle            string
	BarStyleDesc        string
	BarContinuous       string
	BarSegmented        string
	CardBehaviorDetails string
	RoundEnds           string
	RoundEndsDesc       string
	AutoCollapse        string
	AutoCollapseDesc    string
	ShowPercentages     string
	ShowPercentagesDesc string
	ShowDetailedCard    string
	ShowDetailedCardDesc string

	// Settings - Rings
	CardAlertsRings    string
	WarningThreshold   string
	WarningThreshDesc  string
	ShowRemaining      string
	ShowRemainingDesc  string
	ShowSecondRing     string
	ShowSecondRingDesc string
	ShowClockArc       string
	ShowClockArcDesc   string
	SplitPools         string
	SplitPoolsDesc     string

	// Settings - Accounts
	RefreshNow         string
	EmptyAccountsTitle string
	EmptyAccountsDesc  string
	HighestWindowUsage string
	CurrentStatus      string
	RingColor          string
	RingTintAuto       string
	RingTintAzure      string
	RingTintIris       string
	RingTintRose       string
	RingTintAmber      string
	RingTintTeal       string
	ShowInPanel        string
	AccountStatusError string
	AccountStatusCached string
	AccountStatusOnline string
}

// Resolve determines the active UI locale ("en" or "zh-Hans") from the configured
// language preference and the runtime context locale.
func Resolve(cfgLang model.Language, ctxLocale string) string {
	switch model.NormalizeLanguage(cfgLang) {
	case model.LangZhHans:
		return LocaleZhHans
	case model.LangEn:
		return LocaleEn
	default:
		low := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(ctxLocale), "_", "-"))
		if low == "zh" || strings.HasPrefix(low, "zh-hans") || strings.HasPrefix(low, "zh-cn") || strings.HasPrefix(low, "zh-sg") {
			return LocaleZhHans
		}
		return LocaleEn
	}
}

// For returns the localized message table for the configured language and context locale.
func For(cfgLang model.Language, ctxLocale string) *Messages {
	return ByLocale(Resolve(cfgLang, ctxLocale))
}

// ByLocale returns the message table matching a resolved BCP 47 locale tag.
func ByLocale(locale string) *Messages {
	low := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
	if low == "zh" || strings.HasPrefix(low, "zh-hans") || strings.HasPrefix(low, "zh-cn") || strings.HasPrefix(low, "zh-sg") {
		return &zhHansMessages
	}
	return &enMessages
}

// FormatConnected formats the Magpie server connection summary.
func (m *Messages) FormatConnected(ver string, models int) string {
	ver = strings.TrimSpace(ver)
	if ver != "" && !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}
	if m.Locale == LocaleZhHans {
		if ver == "" {
			return fmt.Sprintf("已连接 · %d 个模型", models)
		}
		return fmt.Sprintf("已连接 · %s · %d 个模型", ver, models)
	}
	unit := "models"
	if models == 1 {
		unit = "model"
	}
	if ver == "" {
		return fmt.Sprintf("Connected · %d %s", models, unit)
	}
	return fmt.Sprintf("Connected · %s · %d %s", ver, models, unit)
}

// FormatAccountsTotal formats the total account count header in Accounts overview.
func (m *Messages) FormatAccountsTotal(n int) string {
	if m.Locale == LocaleZhHans {
		return fmt.Sprintf("共 %d 个账号", n)
	}
	if n == 1 {
		return "1 account"
	}
	return fmt.Sprintf("%d accounts", n)
}

// FormatAccountsVisible formats the visible account count subtitle in Accounts overview.
func (m *Messages) FormatAccountsVisible(n int) string {
	if m.Locale == LocaleZhHans {
		return fmt.Sprintf("面板当前显示 %d 个账号", n)
	}
	if n == 1 {
		return "Showing 1 account in panel"
	}
	return fmt.Sprintf("Showing %d accounts in panel", n)
}

// FormatTooltipAccounts formats the tray tooltip when connected without active usage slots.
func (m *Messages) FormatTooltipAccounts(n int) string {
	if m.Locale == LocaleZhHans {
		return fmt.Sprintf("Magpie Pulse · %d 个账号", n)
	}
	if n == 1 {
		return "Magpie Pulse · 1 account"
	}
	return fmt.Sprintf("Magpie Pulse · %d accounts", n)
}

var enMessages = Messages{
	Locale: LocaleEn,

	AppName:             "Magpie Pulse",
	SettingsWindowTitle: "Magpie Pulse Settings",

	TrayRefreshNow:   "Refresh Now",
	TrayHidePanel:    "Hide Panel",
	TrayShowPanel:    "Show Panel",
	TrayShowCached:   "Show Cached Accounts",
	TrayAutoCollapse: "Auto-Collapse Edge",
	TraySurface:      "Surface",
	TraySurfaceDark:  "Dark",
	TraySurfaceLight: "Light",
	TraySurfaceGlass: "Liquid Glass",
	TrayDockPosition: "Dock Position",
	TrayDockRight:    "Right Edge",
	TrayDockLeft:     "Left Edge",
	TrayDockTop:      "Top Edge",
	TrayDockBottom:   "Bottom Edge",
	TrayDockFloating: "Floating",
	TraySettings:     "Settings...",
	TrayQuit:         "Quit Magpie Pulse",

	AlertTitleFmt:      "%s Quota Alert",
	AlertBodyUsedFmt:   "%s is at %.0f%% used",
	AlertBodyExhausted: "%s quota exhausted (100%% used)",

	TooltipOffline: "Magpie Pulse · Offline",

	RailSettingsLabel: "Settings",

	CardUsageSuffixFmt:   "%s Usage",
	CardRefreshLabel:     "Refresh",
	CardUpdatedFmt:       "Updated %s",
	CardCachedPill:       "Cached",
	CardPctUsedFmt:       "%.0f%% Used",
	CardPctLeftFmt:       "%.0f%% Left",
	CardShowAllFmt:       "Show all %d model limits",
	CardShowSummary:      "Show summary only",
	CardQuotaHistory:     "Quota History",
	CardNamedHistoryFmt:  "%s History",
	CardSparklineLeftFmt: "%.0f%% left",
	CardToday:            "Today",
	Card7Days:            "7 days",
	Card30Days:           "30 days",
	CardTokenUnit:        "tok",
	CardCallsFmt:         "%s calls",

	SettingsNavLabel:   "Settings navigation",
	TabGeneral:         "General",
	TabAppearance:      "Appearance",
	TabRings:           "Rings & Quotas",
	TabAccounts:        "Accounts",
	StatusOffline:      "Offline",
	StatusConnected:    "Connected",
	StatusError:        "Error",
	StatusNotConnected: "Not connected",
	StatusTesting:      "Testing connection...",

	CardMagpieConn:       "Magpie Connection",
	MagpieAPIURL:         "Magpie API URL",
	TestConnection:       "Test Connection",
	CardPollingScope:     "Polling & Scope",
	RefreshInterval:      "Refresh Interval",
	RefreshIntervalDesc:  "How often to fetch the latest quota data from Magpie",
	IncludeCached:        "Include Cached / Disabled Accounts",
	IncludeCachedDesc:    "Show offline cached or disabled accounts in the panel and overview",
	CardSystemWindow:     "System & Window",
	Language:             "Language",
	LanguageDesc:         "Choose the display language, or follow system settings automatically",
	LanguageAuto:         "Auto",
	LanguageZhHans:       "简体中文",
	LanguageEn:           "English",
	OpenAtLogin:          "Launch at Login",
	OpenAtLoginDesc:      "Automatically start Magpie Pulse when you log in",
	ShowInDock:           "Show in Dock",
	ShowInDockDesc:       "Display the application icon in the macOS Dock",
	NotifyThreshold:      "Threshold Notifications",
	NotifyThresholdDesc:  "Send a system notification when any account reaches the warning threshold",
	FloatingPanel:        "Floating Panel",
	FloatingPanelDesc:    "Show or focus the desktop floating quota panel immediately",
	ShowPanel:            "Show Panel",
	CardUpdates:          "Updates",
	UpdateAvailableFmt:   "Update %s available",
	UpdatesDescDefault:   "Check for and install signed updates from GitHub Releases",
	LastCheckedFmt:       "Last checked: %s",
	CheckUpdates:         "Check for Updates",
	AutoCheckUpdates:     "Automatic Checks",
	AutoCheckUpdatesDesc: "Automatically check GitHub for new releases on launch and daily",

	CardPanelStyle:       "Panel Style",
	SurfaceMode:          "Surface",
	SurfaceModeDesc:      "Choose the floating panel material and theme",
	SurfaceDark:          "Dark",
	SurfaceLight:         "Light",
	SurfaceGlass:         "Liquid Glass",
	GlassTransparency:    "Glass Transparency",
	GlassCurrentFmt:      "Current %.0f%%",
	DockPosition:         "Dock Position",
	DockPositionDesc:     "Snap to a screen edge or float freely",
	DockRight:            "Right",
	DockLeft:             "Left",
	DockTop:              "Top",
	DockBottom:           "Bottom",
	DockFloating:         "Floating",
	PanelScale:           "Panel Size",
	PanelScaleDesc:       "Adjust ring diameter, spacing, and typography scale",
	ScaleSmall:           "Compact",
	ScaleStandard:        "Standard",
	ScaleLarge:           "Large",
	BarStyle:             "Progress Bar Style",
	BarStyleDesc:         "How time-window progress bars are drawn in the detail card",
	BarContinuous:        "Continuous",
	BarSegmented:         "Segmented",
	CardBehaviorDetails:  "Behavior & Details",
	RoundEnds:            "Rounded Ends",
	RoundEndsDesc:        "Give the floating rail fully rounded pill ends",
	AutoCollapse:         "Auto-Collapse Edge",
	AutoCollapseDesc:     "Collapse docked panel into a slim edge sliver when idle",
	ShowPercentages:      "Show Percentages",
	ShowPercentagesDesc:  "Display used or remaining percentage below each ring",
	ShowDetailedCard:     "Show Usage Summary & Sparkline",
	ShowDetailedCardDesc: "Include token totals and 30-day trend sparkline in detail cards",

	CardAlertsRings:    "Alerts & Ring Display",
	WarningThreshold:   "Warning Threshold",
	WarningThreshDesc:  "Turn rings into warning color once usage reaches this percentage",
	ShowRemaining:      "Show Remaining Instead of Used",
	ShowRemainingDesc:  "Display remaining available quota percentage and arc",
	ShowSecondRing:     "Show Inner Second Ring",
	ShowSecondRingDesc: "Also display secondary window (such as 7-day or weekly limit)",
	ShowClockArc:       "Show Reset Clock Arc",
	ShowClockArcDesc:   "Draw elapsed cycle time as a thin outer arc around the ring",
	SplitPools:         "Split Multi-Pool Accounts (e.g. Antigravity)",
	SplitPoolsDesc:     "Split accounts with multiple quota pools into separate rings",

	RefreshNow:          "Refresh",
	EmptyAccountsTitle:  "No Accounts Yet",
	EmptyAccountsDesc:   "Ensure Magpie is running, or click Refresh to fetch again.",
	HighestWindowUsage:  "Highest Window",
	CurrentStatus:       "Current Status",
	RingColor:           "Ring Color",
	RingTintAuto:        "Auto (Usage)",
	RingTintAzure:       "Azure #4099FF",
	RingTintIris:        "Iris #B380FF",
	RingTintRose:        "Rose #FF70B8",
	RingTintAmber:       "Amber #FF8F33",
	RingTintTeal:        "Teal #33D1D1",
	ShowInPanel:         "Show in Panel",
	AccountStatusError:  "Error",
	AccountStatusCached: "Cached",
	AccountStatusOnline: "Online",
}

var zhHansMessages = Messages{
	Locale: LocaleZhHans,

	AppName:             "Magpie Pulse",
	SettingsWindowTitle: "Magpie Pulse 设置",

	TrayRefreshNow:   "立即刷新",
	TrayHidePanel:    "隐藏面板",
	TrayShowPanel:    "显示面板",
	TrayShowCached:   "显示本地缓存账号",
	TrayAutoCollapse: "边缘自动收起",
	TraySurface:      "面板外观",
	TraySurfaceDark:  "深色纯黑",
	TraySurfaceLight: "浅色",
	TraySurfaceGlass: "液态玻璃",
	TrayDockPosition: "停靠位置",
	TrayDockRight:    "右侧边缘",
	TrayDockLeft:     "左侧边缘",
	TrayDockTop:      "顶部边缘",
	TrayDockBottom:   "底部边缘",
	TrayDockFloating: "自由悬浮",
	TraySettings:     "设置…",
	TrayQuit:         "退出 Magpie Pulse",

	AlertTitleFmt:      "%s 额度告警",
	AlertBodyUsedFmt:   "%s 已使用 %.0f%%",
	AlertBodyExhausted: "%s 额度已耗尽 (已用 100%%)",

	TooltipOffline: "Magpie Pulse · 离线",

	RailSettingsLabel: "设置",

	CardUsageSuffixFmt:   "%s 用量",
	CardRefreshLabel:     "刷新",
	CardUpdatedFmt:       "更新于 %s",
	CardCachedPill:       "本地缓存",
	CardPctUsedFmt:       "已用 %.0f%%",
	CardPctLeftFmt:       "剩余 %.0f%%",
	CardShowAllFmt:       "展开全部 %d 个模型限额",
	CardShowSummary:      "仅显示汇总",
	CardQuotaHistory:     "额度趋势",
	CardNamedHistoryFmt:  "%s 趋势",
	CardSparklineLeftFmt: "剩余 %.0f%%",
	CardToday:            "今日",
	Card7Days:            "7 天",
	Card30Days:           "30 天",
	CardTokenUnit:        "tokens",
	CardCallsFmt:         "%s 次调用",

	SettingsNavLabel:   "设置导航",
	TabGeneral:         "通用与连接",
	TabAppearance:      "外观与面板",
	TabRings:           "圆环与额度",
	TabAccounts:        "账号总览",
	StatusOffline:      "离线",
	StatusConnected:    "已连接",
	StatusError:        "连接异常",
	StatusNotConnected: "未连接",
	StatusTesting:      "正在测试连接...",

	CardMagpieConn:       "Magpie 服务连接",
	MagpieAPIURL:         "Magpie API 地址",
	TestConnection:       "测试连接",
	CardPollingScope:     "轮询与账号范围",
	RefreshInterval:      "刷新间隔",
	RefreshIntervalDesc:  "定时从 Magpie 拉取最新额度数据",
	IncludeCached:        "包含本地缓存 / 停用账号",
	IncludeCachedDesc:    "在面板与总览中显示离线缓存或已停用的账号",
	CardSystemWindow:     "系统集成与窗口",
	Language:             "界面语言",
	LanguageDesc:         "选择界面显示语言，默认自动跟随系统语言",
	LanguageAuto:         "自动",
	LanguageZhHans:       "简体中文",
	LanguageEn:           "English",
	OpenAtLogin:          "开机启动",
	OpenAtLoginDesc:      "登录系统时自动启动 Magpie Pulse",
	ShowInDock:           "在程序坞显示图标",
	ShowInDockDesc:       "开启后应用图标将显示在 macOS Dock 栏中",
	NotifyThreshold:      "超阈值桌面通知",
	NotifyThresholdDesc:  "当任意账号用量达到告警阈值时发送系统通知",
	FloatingPanel:        "悬浮面板",
	FloatingPanelDesc:    "立即唤起或聚焦桌面悬浮配额面板",
	ShowPanel:            "显示悬浮面板",
	CardUpdates:          "版本与自动更新",
	UpdateAvailableFmt:   "可更新至 %s",
	UpdatesDescDefault:   "从 GitHub Releases 检查并自动安装签名更新",
	LastCheckedFmt:       "上次检查：%s",
	CheckUpdates:         "检查更新",
	AutoCheckUpdates:     "自动检查更新",
	AutoCheckUpdatesDesc: "启动后及每日自动检查 GitHub 新版本",

	CardPanelStyle:       "面板风格",
	SurfaceMode:          "面板外观",
	SurfaceModeDesc:      "选择悬浮面板的背景材质与明暗主题",
	SurfaceDark:          "深色纯黑",
	SurfaceLight:         "浅色",
	SurfaceGlass:         "液态玻璃",
	GlassTransparency:    "玻璃透明度",
	GlassCurrentFmt:      "当前 %.0f%%",
	DockPosition:         "停靠位置",
	DockPositionDesc:     "贴合屏幕边缘或自由拖动悬浮",
	DockRight:            "右侧",
	DockLeft:             "左侧",
	DockTop:              "顶部",
	DockBottom:           "底部",
	DockFloating:         "自由悬浮",
	PanelScale:           "面板尺寸",
	PanelScaleDesc:       "调整圆环直径、间距与字体比例",
	ScaleSmall:           "紧凑",
	ScaleStandard:        "标准",
	ScaleLarge:           "宽松",
	BarStyle:             "进度条样式",
	BarStyleDesc:         "详情卡内各时间窗口的进度条绘制风格",
	BarContinuous:        "连续胶囊",
	BarSegmented:         "分段刻度",
	CardBehaviorDetails:  "交互与细节",
	RoundEnds:            "圆润末端",
	RoundEndsDesc:        "使悬浮条两端呈现完全半圆胶囊造型",
	AutoCollapse:         "边缘自动收起",
	AutoCollapseDesc:     "鼠标移出停靠面板后自动贴边折叠成细条",
	ShowPercentages:      "显示百分比数字",
	ShowPercentagesDesc:  "在每个圆环下方显示当前用量或剩余百分比",
	ShowDetailedCard:     "详情卡展示用量概览与历史曲线",
	ShowDetailedCardDesc: "悬停或点击圆环时展示 Token 用量与 30 天趋势曲线",

	CardAlertsRings:    "告警与圆环显示",
	WarningThreshold:   "告警阈值",
	WarningThreshDesc:  "用量超过该百分比时圆环转为高饱和告警色",
	ShowRemaining:      "显示剩余百分比而非已用",
	ShowRemainingDesc:  "圆环数值与进度以剩余可用额度比例呈现",
	ShowSecondRing:     "显示内侧第二环",
	ShowSecondRingDesc: "同时展示次要时间窗口（如 7 天或周限额）",
	ShowClockArc:       "显示重置周期外环",
	ShowClockArcDesc:   "在主圆环外侧绘制当前周期已流逝的时间弧线",
	SplitPools:         "按模型池拆分多池账号（如 Antigravity）",
	SplitPoolsDesc:     "将包含多个独立配额池的账号拆分为多个独立圆环",

	RefreshNow:          "立即刷新",
	EmptyAccountsTitle:  "暂无账号数据",
	EmptyAccountsDesc:   "请确认 Magpie 服务已启动，或点击“立即刷新”重新拉取。",
	HighestWindowUsage:  "最高窗口用量",
	CurrentStatus:       "当前状态",
	RingColor:           "圆环颜色",
	RingTintAuto:        "自动(按用量)",
	RingTintAzure:       "天蓝 #4099FF",
	RingTintIris:        "鸢尾紫 #B380FF",
	RingTintRose:        "玫瑰粉 #FF70B8",
	RingTintAmber:       "琥珀橙 #FF8F33",
	RingTintTeal:        "青绿 #33D1D1",
	ShowInPanel:         "在面板中显示",
	AccountStatusError:  "异常",
	AccountStatusCached: "本地缓存",
	AccountStatusOnline: "在线",
}
