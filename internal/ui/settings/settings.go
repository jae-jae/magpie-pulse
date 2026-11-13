package settings

import (
	"fmt"
	"strings"
	"time"

	"magpie-pulse/internal/icons"
	"magpie-pulse/internal/model"

	"github.com/egoist/mygo/ui"
)

const (
	TabGeneral    = "general"
	TabAppearance = "appearance"
	TabRings      = "rings"
	TabAccounts   = "accounts"

	titleBarHeight float32 = 54
	paneMaxWidth   float32 = 640
)

type settingsNavTab struct {
	key   string
	label string
	icon  *ui.SVG
}

var settingsNavTabs = []settingsNavTab{
	{key: TabGeneral, label: "通用与连接", icon: icons.GeneralIcon},
	{key: TabAppearance, label: "外观与面板", icon: icons.PaletteIcon},
	{key: TabRings, label: "圆环与额度", icon: icons.RingsIcon},
	{key: TabAccounts, label: "账号总览", icon: icons.AccountsIcon},
}

type SettingsState struct {
	ActiveTab           string
	tabIndex            int
	syncedTab           string
	Config              model.Config
	Snapshot            model.Snapshot
	TestStatus          string
	TestOK              bool
	Testing             bool
	AppVersion          string
	AutoCheckUpdates    bool
	UpdateAvailable     string
	LastUpdateCheck     time.Time
	OnConfigChange      func(model.Config)
	OnTestConnection    func(url string)
	OnRefreshNow        func()
	OnShowPanel         func()
	OnCheckUpdates      func()
	OnToggleAutoUpdates func(bool)
}

func (st *SettingsState) ensureDefaults() {
	if st.ActiveTab == "" {
		st.ActiveTab = TabGeneral
	}
	if st.syncedTab != st.ActiveTab {
		st.tabIndex = navTabIndex(st.ActiveTab)
		st.syncedTab = st.ActiveTab
	}
	if st.Config.HiddenAccounts == nil {
		st.Config.HiddenAccounts = map[string]bool{}
	}
	if st.Config.RingTints == nil {
		st.Config.RingTints = map[string]string{}
	}
}

func (st *SettingsState) emitConfigChange() {
	if st.OnConfigChange != nil {
		st.OnConfigChange(st.Config)
	}
}

// SettingsView renders the native macOS-style Magpie Pulse settings interface.
func SettingsView(c *ui.Context, st *SettingsState) {
	if st == nil {
		return
	}
	st.ensureDefaults()

	t := c.Theme()
	vibrant := c.Vibrancy()
	if vibrant {
		c.Root().Background(ui.Transparent)
	}

	bar := c.TitleBar()
	headerH := max(bar.Height+10, titleBarHeight)
	leftPad := max(bar.Left+12, 16)
	rightPad := max(bar.Right+16, 16)

	ui.Column(c).Fill().AlignItems(ui.Stretch).Children(func() {
		header := ui.Row(c).Height(headerH).Shrink(0).AlignItems(ui.Center).
			Padding(0, rightPad, 0, leftPad).Gap(12).DragWindow()
		if !vibrant {
			header.Background(t.Surface)
		}
		header.Children(func() {
			if bar.Left == 0 {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(t.Accent)
					ui.Text(c, "Magpie Pulse").FontSize(13).FontWeight(600).SingleLine()
				})
			}

			ui.Row(c).Grow(1).MinWidth(0).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
				tabs := ui.TabsBase(c.Key("settings-nav-tabs"), &st.tabIndex, len(settingsNavTabs))
				trackBg := t.Background
				if vibrant {
					trackBg = t.Surface.Alpha(0.55)
				}
				tabs.List.
					Padding(3, 3).
					Gap(2).
					Radius(9).
					Background(trackBg).
					Border(1, t.Border).
					Label("设置导航").
					OnChange(func() {
						if st.tabIndex >= 0 && st.tabIndex < len(settingsNavTabs) {
							st.ActiveTab = settingsNavTabs[st.tabIndex].key
							st.syncedTab = st.ActiveTab
						}
					}).
					Children(func() {
						for i, item := range settingsNavTabs {
							on := i == st.tabIndex
							tab := tabs.Tab(i).
								Padding(5, 10).
								Gap(5).
								Radius(7).
								AlignItems(ui.Center).
								FocusRing(false).
								Label(item.label)
							if on {
								tab.Background(t.Surface).Border(1, t.Border)
							} else if tab.Hovered() {
								tab.Background(t.Text.Alpha(0.06)).Border(1, ui.Transparent)
							} else {
								tab.Border(1, ui.Transparent)
							}
							tab.Children(func() {
								iconColor := t.TextMuted
								textColor := t.TextMuted
								if on {
									iconColor = t.Accent
									textColor = t.Text
								} else if tab.Hovered() {
									iconColor = t.Text
									textColor = t.Text
								}
								ui.Icon(c, item.icon).Size(13, 13).TextColor(iconColor).Shrink(0)
								lbl := ui.Text(c, item.label).FontSize(t.FontSize - 1).TextColor(textColor).SingleLine()
								if on {
									lbl.FontWeight(600)
								} else {
									lbl.FontWeight(500)
								}
								if item.key == TabAccounts {
									if n := len(st.Snapshot.Accounts); n > 0 {
										badgeBg := t.Text.Alpha(0.08)
										badgeFg := t.TextMuted
										if on {
											badgeBg = t.Accent.Alpha(0.16)
											badgeFg = t.Accent
										}
										ui.Box(c).Padding(0, 5).Radius(999).Background(badgeBg).Shrink(0).Children(func() {
											ui.Textf(c, "%d", n).FontSize(t.FontSize - 2).FontWeight(600).TextColor(badgeFg).SingleLine()
										})
									}
								}
							})
						}
					})
			})

			renderHeaderStatusPill(c, st)
		})

		ui.Divider(c)

		pane := ui.Column(c).Grow(1).MinHeight(0).Background(t.Background)
		pane.Children(func() {
			switch st.ActiveTab {
			case TabAppearance:
				renderAppearancePane(c, st)
			case TabRings:
				renderRingsPane(c, st)
			case TabAccounts:
				AccountsPane(c, st)
			default:
				renderGeneralPane(c, st)
			}
		})
	})
}

func navTabIndex(tab string) int {
	for i, item := range settingsNavTabs {
		if item.key == tab {
			return i
		}
	}
	return 0
}

func renderHeaderStatusPill(c *ui.Context, st *SettingsState) {
	t := c.Theme()
	dotColor := t.TextMuted
	textColor := t.TextMuted
	label := "离线"
	pillBg := t.Background.Alpha(0.6)
	pillBorder := t.Border

	if st.Snapshot.Connected {
		dotColor = t.Success
		textColor = t.Text
		ver := strings.TrimSpace(st.Snapshot.MagpieVersion)
		if ver != "" {
			if !strings.HasPrefix(ver, "v") {
				ver = "v" + ver
			}
			label = ver
		} else {
			label = "已连接"
		}
		pillBg = t.Success.Alpha(0.10)
		pillBorder = t.Success.Alpha(0.28)
	} else if strings.TrimSpace(st.Snapshot.ConnectionErr) != "" {
		dotColor = t.Danger
		textColor = t.Danger
		label = "连接异常"
		pillBg = t.Danger.Alpha(0.10)
		pillBorder = t.Danger.Alpha(0.28)
	}

	ui.Row(c).Padding(4, 9).Gap(6).Radius(999).AlignItems(ui.Center).
		Background(pillBg).Border(1, pillBorder).Shrink(0).Children(func() {
		ui.Box(c).Size(7, 7).Radius(4).Background(dotColor).Shrink(0)
		ui.Text(c, label).FontSize(t.FontSize - 2).FontWeight(500).TextColor(textColor).SingleLine()
	})
}

func renderCenteredPaneScroll(c *ui.Context, key string, fn func()) {
	ui.Scroll(c.Key(key)).Grow(1).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(paneMaxWidth).Padding(16, 20).Gap(14).Children(fn)
		})
	})
}

func renderGeneralPane(c *ui.Context, st *SettingsState) {
	t := c.Theme()
	renderCenteredPaneScroll(c, "scroll-general", func() {
		settingsCard(c, "Magpie 服务连接", func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "Magpie API 地址").FontWeight(500)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c.Key("magpie-url"), &st.Config.MagpieURL).
						Placeholder("http://127.0.0.1:3425").
						Label("Magpie API 地址").
						Grow(1).
						OnChange(func() {
							st.emitConfigChange()
						})
					ui.Button(c.Key("test-connection"), "测试连接").
						Disabled(st.Testing).
						OnClick(func() {
							if st.OnTestConnection != nil {
								st.OnTestConnection(strings.TrimSpace(st.Config.MagpieURL))
							}
						})
				})
				statusText, statusColor := connectionStatusDisplay(t, st)
				if statusText != "" {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(8, 8).Radius(4).Background(statusColor).Shrink(0)
						ui.Text(c, statusText).FontSize(t.FontSize - 1).TextColor(statusColor)
					})
				}
			})
		})

		settingsCard(c, "轮询与账号范围", func() {
			refreshOptions := []int{15, 30, 60, 300}
			refreshIdx := indexOfInt(refreshOptions, st.Config.RefreshIntervalSecs, 1)
			settingsControlRow(c, "刷新间隔", "定时从 Magpie 拉取最新额度数据", func() {
				ui.Segmented(c.Key("refresh-interval"), &refreshIdx, "15s", "30s", "60s", "5m").
					Label("刷新间隔").
					OnChange(func() {
						st.Config.RefreshIntervalSecs = refreshOptions[refreshIdx]
						st.emitConfigChange()
					})
			})

			ui.Divider(c)

			settingsSwitchRow(c, "include-cached", "包含本地缓存 / 停用账号", "在面板与总览中显示离线缓存或已停用的账号", &st.Config.IncludeCachedAccounts, func() {
				st.emitConfigChange()
			})
		})

		settingsCard(c, "系统集成与窗口", func() {
			settingsSwitchRow(c, "show-in-dock", "在程序坞显示图标", "开启后应用图标将显示在 macOS Dock 栏中", &st.Config.ShowInDock, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			settingsSwitchRow(c, "notify-threshold", "超阈值桌面通知", "当任意账号用量达到告警阈值时发送系统通知", &st.Config.NotifyOnThreshold, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, "悬浮面板").FontWeight(500)
					ui.Text(c, "立即唤起或聚焦桌面悬浮配额面板").FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				})
				ui.PrimaryButton(c.Key("show-panel"), "显示悬浮面板").
					OnClick(func() {
						if st.OnShowPanel != nil {
							st.OnShowPanel()
						}
					})
			})
		})

		settingsCard(c, "版本与自动更新", func() {
			ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ver := strings.TrimSpace(st.AppVersion)
						if ver == "" {
							ver = "0.1.0"
						}
						if !strings.HasPrefix(ver, "v") {
							ver = "v" + ver
						}
						ui.Textf(c, "Magpie Pulse %s", ver).FontWeight(500).SingleLine()
						if avail := strings.TrimSpace(st.UpdateAvailable); avail != "" {
							if !strings.HasPrefix(avail, "v") {
								avail = "v" + avail
							}
							ui.Box(c).Padding(1, 7).Radius(999).Background(t.Accent.Alpha(0.16)).Shrink(0).Children(func() {
								ui.Textf(c, "可更新至 %s", avail).FontSize(t.FontSize - 2).FontWeight(600).TextColor(t.Accent).SingleLine()
							})
						}
					})
					sub := "从 GitHub Releases 检查并自动安装签名更新"
					if !st.LastUpdateCheck.IsZero() {
						sub = "上次检查：" + model.FormatRelativeTime(time.Now(), st.LastUpdateCheck)
					}
					ui.Text(c, sub).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				})
				ui.Button(c.Key("check-updates"), "检查更新").
					OnClick(func() {
						if st.OnCheckUpdates != nil {
							st.OnCheckUpdates()
						}
					})
			})

			ui.Divider(c)

			settingsSwitchRow(c, "auto-check-updates", "自动检查更新", "启动后及每日自动检查 GitHub 新版本", &st.AutoCheckUpdates, func() {
				if st.OnToggleAutoUpdates != nil {
					st.OnToggleAutoUpdates(st.AutoCheckUpdates)
				}
			})
		})
	})
}

func renderAppearancePane(c *ui.Context, st *SettingsState) {
	t := c.Theme()
	renderCenteredPaneScroll(c, "scroll-appearance", func() {
			settingsCard(c, "面板风格", func() {
				surfaceValues := []model.SurfaceMode{model.SurfaceDark, model.SurfaceLight, model.SurfaceGlass}
				surfaceIdx := indexOfSurface(surfaceValues, st.Config.Surface, 0)
				settingsControlRow(c, "面板外观", "选择悬浮面板的背景材质与明暗主题", func() {
					ui.Segmented(c.Key("surface-mode"), &surfaceIdx, "深色纯黑", "浅色", "液态玻璃").
						Label("面板外观").
						OnChange(func() {
							st.Config.Surface = surfaceValues[surfaceIdx]
							st.emitConfigChange()
						})
				})

				if st.Config.Surface == model.SurfaceGlass {
					ui.Divider(c)
					ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
						ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
							ui.Text(c, "玻璃透明度").FontWeight(500)
							ui.Textf(c, "当前 %.0f%%", st.Config.GlassTransparency*100).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
						})
						ui.Slider(c.Key("glass-alpha"), &st.Config.GlassTransparency, 0.1, 0.95).
							Width(180).
							Label("玻璃透明度").
							OnChange(func() {
								st.emitConfigChange()
							})
					})
				}

				ui.Divider(c)

				dockValues := []model.DockEdge{model.DockRight, model.DockLeft, model.DockTop, model.DockBottom, model.DockFloating}
				dockIdx := indexOfDock(dockValues, st.Config.DockEdge, 0)
				settingsControlRow(c, "停靠位置", "贴合屏幕边缘或自由拖动悬浮", func() {
					ui.Segmented(c.Key("dock-edge"), &dockIdx, "右侧", "左侧", "顶部", "底部", "自由悬浮").
						Label("停靠位置").
						OnChange(func() {
							st.Config.DockEdge = dockValues[dockIdx]
							st.emitConfigChange()
						})
				})

				ui.Divider(c)

				scaleValues := []model.PanelScale{model.ScaleSmall, model.ScaleStandard, model.ScaleLarge}
				scaleIdx := indexOfScale(scaleValues, st.Config.Scale, 1)
				settingsControlRow(c, "面板尺寸", "调整圆环直径、间距与字体比例", func() {
					ui.Segmented(c.Key("panel-scale"), &scaleIdx, "紧凑", "标准", "宽松").
						Label("面板尺寸").
						OnChange(func() {
							st.Config.Scale = scaleValues[scaleIdx]
							st.emitConfigChange()
						})
				})

				ui.Divider(c)

				barValues := []model.BarStyle{model.BarContinuous, model.BarSegmented}
				barIdx := indexOfBarStyle(barValues, st.Config.BarStyle, 0)
				settingsControlRow(c, "进度条样式", "详情卡内各时间窗口的进度条绘制风格", func() {
					ui.Segmented(c.Key("bar-style"), &barIdx, "连续胶囊", "分段刻度").
						Label("进度条样式").
						OnChange(func() {
							st.Config.BarStyle = barValues[barIdx]
							st.emitConfigChange()
						})
				})
			})

			settingsCard(c, "交互与细节", func() {
				settingsSwitchRow(c, "round-ends", "圆润末端", "使悬浮条两端呈现完全半圆胶囊造型", &st.Config.RoundEnds, func() {
					st.emitConfigChange()
				})

				ui.Divider(c)

				settingsSwitchRow(c, "auto-collapse", "边缘自动收起", "鼠标移出停靠面板后自动贴边折叠成细条", &st.Config.AutoCollapse, func() {
					st.emitConfigChange()
				})

				ui.Divider(c)

				settingsSwitchRow(c, "show-percentages", "显示百分比数字", "在每个圆环下方显示当前用量或剩余百分比", &st.Config.ShowPercentages, func() {
					st.emitConfigChange()
				})

				ui.Divider(c)

				settingsSwitchRow(c, "show-detailed-card", "详情卡展示用量概览与历史曲线", "悬停或点击圆环时展示 Token 用量与 30 天趋势曲线", &st.Config.ShowDetailedCard, func() {
					st.emitConfigChange()
				})
			})
		})
}

func renderRingsPane(c *ui.Context, st *SettingsState) {
	renderCenteredPaneScroll(c, "scroll-rings", func() {
			settingsCard(c, "告警与圆环显示", func() {
				thresholdValues := []int{60, 70, 75, 80, 85, 90}
				threshIdx := indexOfInt(thresholdValues, st.Config.WarningThreshold, 2)
				settingsControlRow(c, "告警阈值", "用量超过该百分比时圆环转为高饱和告警色", func() {
					ui.Segmented(c.Key("warning-threshold"), &threshIdx, "60%", "70%", "75%", "80%", "85%", "90%").
						Label("告警阈值").
						OnChange(func() {
							st.Config.WarningThreshold = thresholdValues[threshIdx]
							st.emitConfigChange()
						})
				})

				ui.Divider(c)

				settingsSwitchRow(c, "show-remaining", "显示剩余百分比而非已用", "圆环数值与进度以剩余可用额度比例呈现", &st.Config.ShowRemaining, func() {
					st.emitConfigChange()
				})

				ui.Divider(c)

				settingsSwitchRow(c, "show-second-ring", "显示内侧第二环", "同时展示次要时间窗口（如 7 天或周限额）", &st.Config.ShowSecondRing, func() {
					st.emitConfigChange()
				})

				ui.Divider(c)

				settingsSwitchRow(c, "show-clock-arc", "显示重置周期外环", "在主圆环外侧绘制当前周期已流逝的时间弧线", &st.Config.ShowClockArc, func() {
					st.emitConfigChange()
				})

				ui.Divider(c)

				settingsSwitchRow(c, "split-pools", "按模型池拆分多池账号（如 Antigravity）", "将包含多个独立配额池的账号拆分为多个独立圆环", &st.Config.SplitPools, func() {
					st.emitConfigChange()
				})
			})
		})
}

func settingsCard(c *ui.Context, title string, fn func()) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(t.FontSize - 1).FontWeight(600).TextColor(t.TextMuted).Padding(0, 4)
		}
		ui.Column(c).Padding(14, 16).Gap(12).Radius(10).Background(t.Surface).Border(1, t.Border).Children(fn)
	})
}

func settingsControlRow(c *ui.Context, title, subtitle string, control func()) {
	t := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, title).FontWeight(500)
			if subtitle != "" {
				ui.Text(c, subtitle).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			}
		})
		ui.Row(c).AlignItems(ui.Center).Children(control)
	})
}

func settingsSwitchRow(c *ui.Context, key, title, subtitle string, val *bool, onChange func()) {
	t := c.Theme()
	ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
		var labelEl ui.Element
		var descEl ui.Element
		ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
			labelEl = ui.Text(c, title).FontWeight(500)
			if subtitle != "" {
				descEl = ui.Text(c, subtitle).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			}
		})
		sw := ui.Switch(c.Key(key), val).Label(title).LabelledBy(labelEl).OnChange(onChange)
		if descEl.Valid() {
			sw.DescribedBy(descEl)
		}
	})
}

func connectionStatusDisplay(t *ui.Theme, st *SettingsState) (string, ui.Color) {
	if st.Testing {
		return "正在测试连接...", t.TextMuted
	}
	if strings.TrimSpace(st.TestStatus) != "" {
		if st.TestOK {
			return st.TestStatus, t.Success
		}
		return st.TestStatus, t.Danger
	}
	if st.Snapshot.Connected {
		ver := strings.TrimSpace(st.Snapshot.MagpieVersion)
		if ver != "" && !strings.HasPrefix(ver, "v") {
			ver = "v" + ver
		}
		if ver == "" {
			return fmt.Sprintf("Connected · %d models", st.Snapshot.MagpieModelCnt), t.Success
		}
		return fmt.Sprintf("Connected · %s · %d models", ver, st.Snapshot.MagpieModelCnt), t.Success
	}
	if strings.TrimSpace(st.Snapshot.ConnectionErr) != "" {
		return st.Snapshot.ConnectionErr, t.Danger
	}
	return "未连接", t.TextMuted
}

func indexOfInt(opts []int, val int, fallback int) int {
	for i, v := range opts {
		if v == val {
			return i
		}
	}
	return fallback
}

func indexOfSurface(opts []model.SurfaceMode, val model.SurfaceMode, fallback int) int {
	for i, v := range opts {
		if v == val {
			return i
		}
	}
	return fallback
}

func indexOfDock(opts []model.DockEdge, val model.DockEdge, fallback int) int {
	for i, v := range opts {
		if v == val {
			return i
		}
	}
	return fallback
}

func indexOfScale(opts []model.PanelScale, val model.PanelScale, fallback int) int {
	for i, v := range opts {
		if v == val {
			return i
		}
	}
	return fallback
}

func indexOfBarStyle(opts []model.BarStyle, val model.BarStyle, fallback int) int {
	for i, v := range opts {
		if v == val {
			return i
		}
	}
	return fallback
}
