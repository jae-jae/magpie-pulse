package settings

import (
	"fmt"
	"strings"

	"magpie-pulse/internal/i18n"
	"magpie-pulse/internal/icons"
	"magpie-pulse/internal/model"

	"github.com/egoist/mygo/ui"
)

type tintPreset struct {
	Hex   string
	Label string
}

func ringTintPresetsFor(msg *i18n.Messages) []tintPreset {
	return []tintPreset{
		{Hex: "", Label: msg.RingTintAuto},
		{Hex: "#4099FF", Label: msg.RingTintAzure},
		{Hex: "#B380FF", Label: msg.RingTintIris},
		{Hex: "#FF70B8", Label: msg.RingTintRose},
		{Hex: "#FF8F33", Label: msg.RingTintAmber},
		{Hex: "#33D1D1", Label: msg.RingTintTeal},
	}
}

// AccountsPane renders the accounts overview, visibility switches, and ring tint presets.
func AccountsPane(c *ui.Context, st *SettingsState) {
	if st == nil {
		return
	}
	st.ensureDefaults()
	msg := i18n.For(st.Config.Language, c.Locale())
	t := c.Theme()

	accounts := st.Snapshot.Accounts
	visibleCount := 0
	for _, acc := range accounts {
		if !st.Config.HiddenAccounts[acc.AccountKey()] {
			visibleCount++
		}
	}

	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Row(c).FillWidth().MaxWidth(paneMaxWidth).Padding(16, 20, 12, 20).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, msg.FormatAccountsTotal(len(accounts))).FontWeight(600)
					ui.Text(c, msg.FormatAccountsVisible(visibleCount)).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				})
				ui.Button(c.Key("refresh-now"), "").Label(msg.RefreshNow).
					OnClick(func() {
						if st.OnRefreshNow != nil {
							st.OnRefreshNow()
						}
					}).
					Children(func() {
						ui.Icon(c, icons.RefreshIcon).Size(14, 14)
						ui.Text(c, msg.RefreshNow).SingleLine()
					})
			})
		})

		ui.Divider(c)

		ui.Scroll(c.Key("scroll-accounts")).Grow(1).Children(func() {
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(paneMaxWidth).Padding(16, 20).Gap(12).Children(func() {
					if len(accounts) == 0 {
						ui.Column(c).Padding(24).Gap(8).Center().Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
							ui.Icon(c, icons.AccountsIcon).Size(28, 28).TextColor(t.TextMuted)
							ui.Text(c, msg.EmptyAccountsTitle).FontWeight(600)
							ui.Text(c, msg.EmptyAccountsDesc).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
						})
						return
					}

					seenKeys := map[string]int{}
					for _, acc := range accounts {
						accKey := acc.AccountKey()
						seenKeys[accKey]++
						uniqueKey := accKey
						if seenKeys[accKey] > 1 {
							uniqueKey = fmt.Sprintf("%s#%d", accKey, seenKeys[accKey])
						}
						renderAccountCard(c.Key("acc-"+uniqueKey), st, acc, accKey, msg)
					}
				})
			})
		})
	})
}

func renderAccountCard(c *ui.Context, st *SettingsState, acc model.AccountQuota, accKey string, msg *i18n.Messages) {
	t := c.Theme()
	hasError := strings.TrimSpace(acc.Error) != ""
	statusLabel, statusColor := accountStatusBadge(t, acc, hasError, msg)
	maxUsed, hasNumeric := highestWindowUsage(acc)
	ringTintPresets := ringTintPresetsFor(msg)

	ui.Column(c).Padding(14, 16).Gap(10).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Grow(1).MinWidth(0).Children(func() {
				ui.Box(c).Size(36, 36).Radius(8).Background(t.Background).Border(1, t.Border).Center().Shrink(0).Children(func() {
					ui.Icon(c, icons.ForProvider(acc.Provider)).Size(20, 20).TextColor(t.Text)
				})
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
						ui.Text(c, acc.DisplayNameLocale(msg.Locale)).FontWeight(600).SingleLine()
						if strings.TrimSpace(acc.Plan) != "" {
							ui.Badge(c, acc.Plan)
						}
						ui.Box(c).Padding(1, 7).Radius(999).Background(statusColor.Alpha(0.16)).Shrink(0).Children(func() {
							ui.Text(c, statusLabel).FontSize(t.FontSize - 2).FontWeight(500).TextColor(statusColor).SingleLine()
						})
					})
					if strings.TrimSpace(acc.User) != "" {
						ui.Text(c, acc.User).FontSize(t.FontSize - 1).TextColor(t.TextMuted).SingleLine()
					}
				})
			})

			ui.Column(c).AlignItems(ui.End).Gap(2).Shrink(0).Children(func() {
				if hasNumeric {
					pctColor := model.UsageTintColor(maxUsed/100.0, maxUsed >= 100, st.Config.WarningThreshold, !t.Dark)
					ui.Textf(c, "%.0f%%", maxUsed).FontSize(16).Bold().TextColor(pctColor).SingleLine()
					ui.Text(c, msg.HighestWindowUsage).FontSize(t.FontSize - 2).TextColor(t.TextMuted).SingleLine()
				} else if len(acc.Windows) > 0 && strings.TrimSpace(acc.Windows[0].Display) != "" {
					ui.Text(c, acc.Windows[0].Display).FontSize(t.FontSize - 1).FontWeight(500).SingleLine()
					ui.Text(c, msg.CurrentStatus).FontSize(t.FontSize - 2).TextColor(t.TextMuted).SingleLine()
				} else {
					ui.Text(c, "--").FontSize(15).TextColor(t.TextMuted).SingleLine()
				}
			})
		})

		if hasError {
			ui.Row(c).Padding(8, 10).Gap(8).Radius(6).Background(t.Danger.Alpha(0.12)).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.AlertIcon).Size(15, 15).TextColor(t.Danger).Shrink(0)
				ui.Text(c, acc.Error).FontSize(t.FontSize - 1).TextColor(t.Danger).Grow(1)
			})
		}

		ui.Divider(c)

		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(12).Wrap().Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, msg.RingColor).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				currentTint := strings.TrimSpace(st.Config.RingTints[accKey])
				for _, preset := range ringTintPresets {
					hex := preset.Hex
					selected := strings.EqualFold(currentTint, hex)
					if hex == "" {
						btn := ui.ButtonBase(c.Key("tint-auto")).
							Padding(3, 8).
							Radius(999).
							Center().
							Label(preset.Label).
							OnClick(func() {
								if st.Config.RingTints == nil {
									st.Config.RingTints = map[string]string{}
								}
								delete(st.Config.RingTints, accKey)
								st.emitConfigChange()
							})
						if selected {
							btn.Background(t.Accent.Alpha(0.18)).Border(1, t.Accent)
						} else {
							btn.Background(t.Background).Border(1, t.Border)
						}
						btn.Children(func() {
							ui.Text(c, preset.Label).FontSize(t.FontSize - 2).SingleLine()
						})
						continue
					}

					swatchColor, ok := model.ParseHexColor(hex)
					if !ok {
						continue
					}
					swatch := ui.ButtonBase(c.Key("tint-"+hex)).
						Size(22, 22).
						Radius(11).
						Center().
						Label(preset.Label).
						Tooltip(preset.Label).
						OnClick(func() {
							if st.Config.RingTints == nil {
								st.Config.RingTints = map[string]string{}
							}
							st.Config.RingTints[accKey] = hex
							st.emitConfigChange()
						})
					if selected {
						swatch.Background(t.Background).Border(2, swatchColor).Children(func() {
							ui.Box(c).Size(12, 12).Radius(6).Background(swatchColor)
						})
					} else {
						swatch.Background(swatchColor).Border(1, t.Border)
					}
				}
			})

			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				visible := !st.Config.HiddenAccounts[accKey]
				lbl := ui.Text(c, msg.ShowInPanel).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				ui.Switch(c.Key("visible"), &visible).
					Label(msg.ShowInPanel).
					LabelledBy(lbl).
					OnChange(func() {
						if st.Config.HiddenAccounts == nil {
							st.Config.HiddenAccounts = map[string]bool{}
						}
						if visible {
							delete(st.Config.HiddenAccounts, accKey)
						} else {
							st.Config.HiddenAccounts[accKey] = true
						}
						st.emitConfigChange()
					})
			})
		})
	})
}

func accountStatusBadge(t *ui.Theme, acc model.AccountQuota, hasError bool, msg *i18n.Messages) (string, ui.Color) {
	if hasError {
		return msg.AccountStatusError, t.Danger
	}
	if acc.CachedOnly || acc.Disabled {
		return msg.AccountStatusCached, t.Warning
	}
	return msg.AccountStatusOnline, t.Success
}

func highestWindowUsage(acc model.AccountQuota) (float64, bool) {
	found := false
	maxUsed := 0.0
	for _, w := range acc.Windows {
		if w.IsText {
			continue
		}
		if !found || w.Used > maxUsed {
			maxUsed = w.Used
			found = true
		}
	}
	return maxUsed, found
}
