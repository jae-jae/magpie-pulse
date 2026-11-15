package settings

import (
	"fmt"
	"strings"
	"time"

	"magpie-pulse/internal/i18n"
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

func navTabsFor(msg *i18n.Messages) []settingsNavTab {
	return []settingsNavTab{
		{key: TabGeneral, label: msg.TabGeneral, icon: icons.GeneralIcon},
		{key: TabAppearance, label: msg.TabAppearance, icon: icons.PaletteIcon},
		{key: TabRings, label: msg.TabRings, icon: icons.RingsIcon},
		{key: TabAccounts, label: msg.TabAccounts, icon: icons.AccountsIcon},
	}
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
	st.Config.Language = model.NormalizeLanguage(st.Config.Language)
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
	msg := i18n.For(st.Config.Language, c.Locale())
	navTabs := navTabsFor(msg)

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
				tabs := ui.TabsBase(c.Key("settings-nav-tabs"), &st.tabIndex, len(navTabs))
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
					Label(msg.SettingsNavLabel).
					OnChange(func() {
						if st.tabIndex >= 0 && st.tabIndex < len(navTabs) {
							st.ActiveTab = navTabs[st.tabIndex].key
							st.syncedTab = st.ActiveTab
						}
					}).
					Children(func() {
						for i, item := range navTabs {
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

			renderHeaderStatusPill(c, st, msg)
		})

		ui.Divider(c)

		pane := ui.Column(c).Grow(1).MinHeight(0).Background(t.Background)
		pane.Children(func() {
			switch st.ActiveTab {
			case TabAppearance:
				renderAppearancePane(c, st, msg)
			case TabRings:
				renderRingsPane(c, st, msg)
			case TabAccounts:
				AccountsPane(c, st)
			default:
				renderGeneralPane(c, st, msg)
			}
		})
	})
}

func navTabIndex(tab string) int {
	switch tab {
	case TabGeneral:
		return 0
	case TabAppearance:
		return 1
	case TabRings:
		return 2
	case TabAccounts:
		return 3
	default:
		return 0
	}
}

func renderHeaderStatusPill(c *ui.Context, st *SettingsState, msg *i18n.Messages) {
	t := c.Theme()
	dotColor := t.TextMuted
	textColor := t.TextMuted
	label := msg.StatusOffline
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
			label = msg.StatusConnected
		}
		pillBg = t.Success.Alpha(0.10)
		pillBorder = t.Success.Alpha(0.28)
	} else if strings.TrimSpace(st.Snapshot.ConnectionErr) != "" {
		dotColor = t.Danger
		textColor = t.Danger
		label = msg.StatusError
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

func renderGeneralPane(c *ui.Context, st *SettingsState, msg *i18n.Messages) {
	t := c.Theme()
	renderCenteredPaneScroll(c, "scroll-general", func() {
		settingsCard(c, msg.CardMagpieConn, func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, msg.MagpieAPIURL).FontWeight(500)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c.Key("magpie-url"), &st.Config.MagpieURL).
						Placeholder("http://127.0.0.1:3425").
						Label(msg.MagpieAPIURL).
						Grow(1).
						OnChange(func() {
							st.emitConfigChange()
						})
					ui.Button(c.Key("test-connection"), msg.TestConnection).
						Disabled(st.Testing).
						OnClick(func() {
							if st.OnTestConnection != nil {
								st.OnTestConnection(strings.TrimSpace(st.Config.MagpieURL))
							}
						})
				})
				statusText, statusColor := connectionStatusDisplay(t, st, msg)
				if statusText != "" {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(8, 8).Radius(4).Background(statusColor).Shrink(0)
						ui.Text(c, statusText).FontSize(t.FontSize - 1).TextColor(statusColor)
					})
				}
			})
		})

		settingsCard(c, msg.CardPollingScope, func() {
			refreshOptions := []int{15, 30, 60, 300}
			refreshIdx := indexOfInt(refreshOptions, st.Config.RefreshIntervalSecs, 1)
			settingsControlRow(c, msg.RefreshInterval, msg.RefreshIntervalDesc, func() {
				ui.Segmented(c.Key("refresh-interval"), &refreshIdx, "15s", "30s", "60s", "5m").
					Label(msg.RefreshInterval).
					OnChange(func() {
						st.Config.RefreshIntervalSecs = refreshOptions[refreshIdx]
						st.emitConfigChange()
					})
			})

			ui.Divider(c)

			settingsSwitchRow(c, "include-cached", msg.IncludeCached, msg.IncludeCachedDesc, &st.Config.IncludeCachedAccounts, func() {
				st.emitConfigChange()
			})
		})

		settingsCard(c, msg.CardSystemWindow, func() {
			langValues := []model.Language{model.LangAuto, model.LangZhHans, model.LangEn}
			langIdx := indexOfLanguage(langValues, st.Config.Language, 0)
			settingsControlRow(c, msg.Language, msg.LanguageDesc, func() {
				ui.Segmented(c.Key("app-language"), &langIdx, msg.LanguageAuto, msg.LanguageZhHans, msg.LanguageEn).
					Label(msg.Language).
					OnChange(func() {
						st.Config.Language = langValues[langIdx]
						st.emitConfigChange()
					})
			})

			ui.Divider(c)

			settingsSwitchRow(c, "show-in-dock", msg.ShowInDock, msg.ShowInDockDesc, &st.Config.ShowInDock, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			settingsSwitchRow(c, "notify-threshold", msg.NotifyThreshold, msg.NotifyThresholdDesc, &st.Config.NotifyOnThreshold, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, msg.FloatingPanel).FontWeight(500)
					ui.Text(c, msg.FloatingPanelDesc).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				})
				ui.PrimaryButton(c.Key("show-panel"), msg.ShowPanel).
					OnClick(func() {
						if st.OnShowPanel != nil {
							st.OnShowPanel()
						}
					})
			})
		})

		settingsCard(c, msg.CardUpdates, func() {
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
								ui.Textf(c, msg.UpdateAvailableFmt, avail).FontSize(t.FontSize - 2).FontWeight(600).TextColor(t.Accent).SingleLine()
							})
						}
					})
					sub := msg.UpdatesDescDefault
					if !st.LastUpdateCheck.IsZero() {
						sub = fmt.Sprintf(msg.LastCheckedFmt, model.FormatRelativeTimeLocale(msg.Locale, time.Now(), st.LastUpdateCheck))
					}
					ui.Text(c, sub).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				})
				ui.Button(c.Key("check-updates"), msg.CheckUpdates).
					OnClick(func() {
						if st.OnCheckUpdates != nil {
							st.OnCheckUpdates()
						}
					})
			})

			ui.Divider(c)

			settingsSwitchRow(c, "auto-check-updates", msg.AutoCheckUpdates, msg.AutoCheckUpdatesDesc, &st.AutoCheckUpdates, func() {
				if st.OnToggleAutoUpdates != nil {
					st.OnToggleAutoUpdates(st.AutoCheckUpdates)
				}
			})
		})
	})
}

func renderAppearancePane(c *ui.Context, st *SettingsState, msg *i18n.Messages) {
	t := c.Theme()
	renderCenteredPaneScroll(c, "scroll-appearance", func() {
		settingsCard(c, msg.CardPanelStyle, func() {
			surfaceValues := []model.SurfaceMode{model.SurfaceDark, model.SurfaceLight, model.SurfaceGlass}
			surfaceIdx := indexOfSurface(surfaceValues, st.Config.Surface, 0)
			settingsControlRow(c, msg.SurfaceMode, msg.SurfaceModeDesc, func() {
				ui.Segmented(c.Key("surface-mode"), &surfaceIdx, msg.SurfaceDark, msg.SurfaceLight, msg.SurfaceGlass).
					Label(msg.SurfaceMode).
					OnChange(func() {
						st.Config.Surface = surfaceValues[surfaceIdx]
						st.emitConfigChange()
					})
			})

			if st.Config.Surface == model.SurfaceGlass {
				ui.Divider(c)
				ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
					ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, msg.GlassTransparency).FontWeight(500)
						ui.Textf(c, msg.GlassCurrentFmt, st.Config.GlassTransparency*100).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
					})
					ui.Slider(c.Key("glass-alpha"), &st.Config.GlassTransparency, 0.1, 0.95).
						Width(180).
						Label(msg.GlassTransparency).
						OnChange(func() {
							st.emitConfigChange()
						})
				})
			}

			ui.Divider(c)

			dockValues := []model.DockEdge{model.DockRight, model.DockLeft, model.DockTop, model.DockBottom, model.DockFloating}
			dockIdx := indexOfDock(dockValues, st.Config.DockEdge, 0)
			settingsControlRow(c, msg.DockPosition, msg.DockPositionDesc, func() {
				ui.Segmented(c.Key("dock-edge"), &dockIdx, msg.DockRight, msg.DockLeft, msg.DockTop, msg.DockBottom, msg.DockFloating).
					Label(msg.DockPosition).
					OnChange(func() {
						st.Config.DockEdge = dockValues[dockIdx]
						st.emitConfigChange()
					})
			})

			ui.Divider(c)

			scaleValues := []model.PanelScale{model.ScaleSmall, model.ScaleStandard, model.ScaleLarge}
			scaleIdx := indexOfScale(scaleValues, st.Config.Scale, 1)
			settingsControlRow(c, msg.PanelScale, msg.PanelScaleDesc, func() {
				ui.Segmented(c.Key("panel-scale"), &scaleIdx, msg.ScaleSmall, msg.ScaleStandard, msg.ScaleLarge).
					Label(msg.PanelScale).
					OnChange(func() {
						st.Config.Scale = scaleValues[scaleIdx]
						st.emitConfigChange()
					})
			})

			ui.Divider(c)

			barValues := []model.BarStyle{model.BarContinuous, model.BarSegmented}
			barIdx := indexOfBarStyle(barValues, st.Config.BarStyle, 0)
			settingsControlRow(c, msg.BarStyle, msg.BarStyleDesc, func() {
				ui.Segmented(c.Key("bar-style"), &barIdx, msg.BarContinuous, msg.BarSegmented).
					Label(msg.BarStyle).
					OnChange(func() {
						st.Config.BarStyle = barValues[barIdx]
						st.emitConfigChange()
					})
			})
		})

		settingsCard(c, msg.CardBehaviorDetails, func() {
			settingsSwitchRow(c, "round-ends", msg.RoundEnds, msg.RoundEndsDesc, &st.Config.RoundEnds, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			settingsSwitchRow(c, "auto-collapse", msg.AutoCollapse, msg.AutoCollapseDesc, &st.Config.AutoCollapse, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			settingsSwitchRow(c, "show-percentages", msg.ShowPercentages, msg.ShowPercentagesDesc, &st.Config.ShowPercentages, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			settingsSwitchRow(c, "show-detailed-card", msg.ShowDetailedCard, msg.ShowDetailedCardDesc, &st.Config.ShowDetailedCard, func() {
				st.emitConfigChange()
			})
		})
	})
}

func renderRingsPane(c *ui.Context, st *SettingsState, msg *i18n.Messages) {
	renderCenteredPaneScroll(c, "scroll-rings", func() {
		settingsCard(c, msg.CardAlertsRings, func() {
			thresholdValues := []int{60, 70, 75, 80, 85, 90}
			threshIdx := indexOfInt(thresholdValues, st.Config.WarningThreshold, 2)
			settingsControlRow(c, msg.WarningThreshold, msg.WarningThreshDesc, func() {
				ui.Segmented(c.Key("warning-threshold"), &threshIdx, "60%", "70%", "75%", "80%", "85%", "90%").
					Label(msg.WarningThreshold).
					OnChange(func() {
						st.Config.WarningThreshold = thresholdValues[threshIdx]
						st.emitConfigChange()
					})
			})

			ui.Divider(c)

			settingsSwitchRow(c, "show-remaining", msg.ShowRemaining, msg.ShowRemainingDesc, &st.Config.ShowRemaining, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			settingsSwitchRow(c, "show-second-ring", msg.ShowSecondRing, msg.ShowSecondRingDesc, &st.Config.ShowSecondRing, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			settingsSwitchRow(c, "show-clock-arc", msg.ShowClockArc, msg.ShowClockArcDesc, &st.Config.ShowClockArc, func() {
				st.emitConfigChange()
			})

			ui.Divider(c)

			settingsSwitchRow(c, "split-pools", msg.SplitPools, msg.SplitPoolsDesc, &st.Config.SplitPools, func() {
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

func connectionStatusDisplay(t *ui.Theme, st *SettingsState, msg *i18n.Messages) (string, ui.Color) {
	if st.Testing {
		return msg.StatusTesting, t.TextMuted
	}
	if strings.TrimSpace(st.TestStatus) != "" {
		if st.TestOK {
			return st.TestStatus, t.Success
		}
		return st.TestStatus, t.Danger
	}
	if st.Snapshot.Connected {
		return msg.FormatConnected(st.Snapshot.MagpieVersion, st.Snapshot.MagpieModelCnt), t.Success
	}
	if strings.TrimSpace(st.Snapshot.ConnectionErr) != "" {
		return st.Snapshot.ConnectionErr, t.Danger
	}
	return msg.StatusNotConnected, t.TextMuted
}

func indexOfLanguage(opts []model.Language, val model.Language, fallback int) int {
	for i, v := range opts {
		if v == val {
			return i
		}
	}
	return fallback
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
