package app

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"magpie-pulse/internal/icons"
	"magpie-pulse/internal/magpie"
	"magpie-pulse/internal/model"
	"magpie-pulse/internal/ui/panel"
	"magpie-pulse/internal/ui/settings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

const (
	cardHideGrace    = 240 * time.Millisecond
	railCollapseWait = 350 * time.Millisecond
	edgeRevealGrace  = 800 * time.Millisecond
	dragSettleWait   = 200 * time.Millisecond
	pointerPollRate  = 80 * time.Millisecond
)

// App coordinates the Magpie polling service, floating dock rail, detail card flyout,
// system tray, desktop notifications, and settings window.
type App struct {
	cfg   model.Config
	snap  model.Snapshot
	slots []model.RailSlot

	service *magpie.Service

	railState     panel.RailState
	cardState     panel.CardState
	settingsState settings.SettingsState

	tray             *mygo.Tray
	trayMenu         *mygo.Menu
	menuToggleRail   *mygo.MenuItem
	menuCached       *mygo.MenuItem
	menuAutoCollapse *mygo.MenuItem
	surfaceItems     map[model.SurfaceMode]*mygo.MenuItem
	dockItems        map[model.DockEdge]*mygo.MenuItem

	railWin     *mygo.Window
	cardWin     *mygo.Window
	settingsWin *mygo.Window

	railVisible    bool
	railExpanded   bool
	quitting       bool
	hoveredSlotID  string
	selectedSlotID string
	cardHideAt     time.Time
	collapseAt     time.Time
	edgeGraceUntil time.Time
	dragActive     bool
	lastMoveAt     time.Time
	dragPreviewEdge model.DockEdge
	suppressMove   bool
	lastSlotCount  int
	trayAlert      bool
	watchActive    atomic.Bool

	stopCh   chan struct{}
	stopOnce sync.Once

	setOpenAtLogin func(bool) error
	getOpenAtLogin func() bool
	canAutoLogin   func() bool
}

// New constructs the application controller and loads persisted configuration.
func New() *App {
	cfg := model.LoadConfig()
	cfg.Normalize()

	a := &App{
		cfg: cfg,
		snap: model.Snapshot{
			Histories: map[string]model.AccountHistory{},
		},
		service:      magpie.NewService(cfg),
		railVisible:  true,
		railExpanded: !cfg.AutoCollapse,
		surfaceItems: map[model.SurfaceMode]*mygo.MenuItem{},
		dockItems:    map[model.DockEdge]*mygo.MenuItem{},
		stopCh:       make(chan struct{}),
	}

	a.railState = panel.RailState{
		Config:         cfg,
		Expanded:       a.railExpanded,
		SlotBounds:     map[string]ui.Rect{},
		OnHoverSlot:    a.onHoverSlot,
		OnLeaveRail:    a.onLeaveRail,
		OnClickSlot:    a.onClickSlot,
		OnRightClick:   a.onRailRightClick,
		OnOpenSettings: a.openSettings,
	}

	a.cardState = panel.CardState{
		Config: cfg,
		OnCopyText: func(text string) {
			if text != "" {
				mygo.Clipboard.WriteText(text)
			}
		},
		OnRefresh: a.refreshNow,
	}

	a.settingsState = settings.SettingsState{
		ActiveTab:        settings.TabGeneral,
		Config:           cfg,
		Snapshot:         a.snap,
		AppVersion:       appVersionOrDefault(),
		OnConfigChange:   a.applyConfig,
		OnTestConnection: a.testConnection,
		OnRefreshNow:     a.refreshNow,
		OnShowPanel:      a.showRail,
		OnCheckUpdates: func() {
			updater.CheckForUpdates()
		},
		OnToggleAutoUpdates: func(on bool) {
			updater.SetAutomaticChecks(on)
		},
	}

	return a
}

func appVersionOrDefault() string {
	if v := strings.TrimSpace(mygo.App.Version()); v != "" {
		return v
	}
	return "0.1.0-dev"
}

// Run starts the MyGo event loop and blocks until the application quits.
func (a *App) Run() error {
	mygo.Use(native.Plugin)
	mygo.App.SetName("Magpie Pulse")
	if strings.TrimSpace(mygo.App.Version()) == "" {
		mygo.App.SetVersion("0.1.0-dev")
	}
	applyActivationPolicy(a.cfg.ShowInDock)

	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		a.showRail()
	})
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		a.quitting = true
	})
	mygo.App.OnQuitCanceled(func() {
		a.quitting = false
	})
	mygo.App.OnWillQuit(func(e *mygo.QuitEvent) {
		a.shutdown()
	})
	mygo.App.WhenReady(a.ready)
	return mygo.App.Run()
}

func (a *App) ready() {
	a.syncUpdateState()
	a.syncOpenAtLoginOnReady()
	updater.OnChange(func() {
		a.syncUpdateState()
		if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
			a.settingsWin.Invalidate()
		}
	})
	a.setupTray()
	a.createRailWindow(nil)
	a.setupService()
	mygo.Screen.OnDisplaysChanged(func() {
		a.layoutRailWindow()
	})
	a.startPointerWatcher()
	a.service.Start()
	a.refreshNow()
}

func (a *App) syncUpdateState() {
	a.settingsState.AppVersion = appVersionOrDefault()
	a.settingsState.AutoCheckUpdates = updater.AutomaticChecks()
	a.settingsState.UpdateAvailable = updater.AvailableVersion()
	a.settingsState.LastUpdateCheck = updater.LastCheck()
}

func (a *App) shutdown() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
	})
	if a.service != nil {
		a.service.Stop()
	}
}

func applyActivationPolicy(showInDock bool) {
	if showInDock {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyRegular)
	} else {
		mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	}
}

func (a *App) applyOpenAtLogin(enabled bool) {
	if a.setOpenAtLogin != nil {
		if err := a.setOpenAtLogin(enabled); err != nil {
			log.Printf("magpie-pulse: set open at login (%v): %v", enabled, err)
		}
		return
	}
	if !mygo.App.IsReady() {
		return
	}
	if err := mygo.App.SetOpenAtLogin(enabled); err != nil {
		log.Printf("magpie-pulse: set open at login (%v): %v", enabled, err)
	}
}

func (a *App) syncOpenAtLoginOnReady() {
	canAuto := a.canAutoLogin
	if canAuto == nil {
		canAuto = func() bool {
			return !mygo.IsDev() && mygo.App.IsReady() && mygo.App.IsPackaged()
		}
	}
	if !canAuto() {
		return
	}
	current := false
	if a.getOpenAtLogin != nil {
		current = a.getOpenAtLogin()
	} else {
		current = mygo.App.OpenAtLogin()
	}
	if current != a.cfg.OpenAtLogin {
		a.applyOpenAtLogin(a.cfg.OpenAtLogin)
	}
}

func (a *App) setupService() {
	a.service.OnUpdate = func(snap model.Snapshot) {
		if a.railWin == nil || a.railWin.IsDestroyed() {
			return
		}
		a.railWin.Update(func() {
			a.applySnapshot(snap)
		})
	}
	a.service.OnAlert = func(ev magpie.AlertEvent) {
		if !mygo.NotificationsSupported() {
			return
		}
		title := fmt.Sprintf("%s Quota Alert", ev.Title)
		body := fmt.Sprintf("%s is at %.0f%% used", ev.WindowName, ev.UsedPct)
		if ev.Spent {
			body = fmt.Sprintf("%s quota exhausted (100%% used)", ev.WindowName)
		}
		n := mygo.NewNotification(mygo.NotificationOptions{
			ID:       fmt.Sprintf("magpie-pulse:%s:%s", ev.AccountKey, ev.WindowName),
			Title:    title,
			Body:     body,
			Subtitle: "Magpie Pulse",
			Group:    "quota-alerts",
		})
		go func() {
			_ = n.Show()
		}()
	}
}

func (a *App) applySnapshot(snap model.Snapshot) {
	a.snap = snap
	a.slots = model.BuildRailSlots(snap.Accounts, a.cfg, time.Now())
	a.railState.Slots = a.slots
	a.railState.Refreshing = false
	a.settingsState.Snapshot = snap

	if len(a.slots) != a.lastSlotCount {
		a.layoutRailWindow()
	}

	alert, tip := WorstAlertSummary(a.slots, a.cfg, a.snap)
	if a.tray != nil {
		if alert != a.trayAlert {
			a.trayAlert = alert
			_ = a.tray.SetIcon(icons.TrayIconPNG(alert), !alert)
		}
		a.tray.SetToolTip(tip)
	}

	if a.cardWin != nil && !a.cardWin.IsDestroyed() && a.cardWin.IsVisible() && a.cardState.Slot != nil {
		if updated := findSlotByID(a.slots, a.cardState.Slot.SlotID); updated != nil {
			a.populateCardState(updated)
			a.cardWin.Invalidate()
		} else {
			a.hideCard()
		}
	}

	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Invalidate()
	}
}

func (a *App) setupTray() {
	a.menuToggleRail = &mygo.MenuItem{
		Label: "Hide Panel",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.toggleRail()
		},
	}
	a.menuCached = &mygo.MenuItem{
		Label:   "Show Cached Accounts",
		Type:    mygo.MenuItemCheckbox,
		Checked: a.cfg.IncludeCachedAccounts,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			next := a.cfg
			next.IncludeCachedAccounts = !next.IncludeCachedAccounts
			a.applyConfig(next)
		},
	}
	a.menuAutoCollapse = &mygo.MenuItem{
		Label:   "Auto-Collapse Edge",
		Type:    mygo.MenuItemCheckbox,
		Checked: a.cfg.AutoCollapse,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			next := a.cfg
			next.AutoCollapse = !next.AutoCollapse
			a.applyConfig(next)
		},
	}

	surfaceDefs := []struct {
		mode  model.SurfaceMode
		label string
	}{
		{model.SurfaceDark, "Dark"},
		{model.SurfaceLight, "Light"},
		{model.SurfaceGlass, "Liquid Glass"},
	}
	var surfaceSubmenu []*mygo.MenuItem
	for _, def := range surfaceDefs {
		mode := def.mode
		item := &mygo.MenuItem{
			Label:   def.label,
			Type:    mygo.MenuItemRadio,
			Checked: a.cfg.Surface == mode,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				next := a.cfg
				next.Surface = mode
				a.applyConfig(next)
			},
		}
		a.surfaceItems[mode] = item
		surfaceSubmenu = append(surfaceSubmenu, item)
	}

	dockDefs := []struct {
		edge  model.DockEdge
		label string
	}{
		{model.DockRight, "Right Edge"},
		{model.DockLeft, "Left Edge"},
		{model.DockTop, "Top Edge"},
		{model.DockBottom, "Bottom Edge"},
		{model.DockFloating, "Floating"},
	}
	var dockSubmenu []*mygo.MenuItem
	for _, def := range dockDefs {
		edge := def.edge
		item := &mygo.MenuItem{
			Label:   def.label,
			Type:    mygo.MenuItemRadio,
			Checked: a.cfg.DockEdge == edge,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				next := a.cfg
				next.DockEdge = edge
				a.applyConfig(next)
			},
		}
		a.dockItems[edge] = item
		dockSubmenu = append(dockSubmenu, item)
	}

	a.trayMenu = mygo.NewMenu([]*mygo.MenuItem{
		{
			Label: "Refresh Now",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.refreshNow()
			},
		},
		a.menuToggleRail,
		mygo.Separator(),
		a.menuCached,
		a.menuAutoCollapse,
		{
			Label:   "Surface",
			Submenu: surfaceSubmenu,
		},
		{
			Label:   "Dock Position",
			Submenu: dockSubmenu,
		},
		mygo.Separator(),
		{
			Label: "Settings...",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.openSettings()
			},
		},
			updater.MenuItem(),
		mygo.Separator(),
		{
			Label: "Quit Magpie Pulse",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.quitting = true
				mygo.App.Quit()
			},
		},
	})

	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "magpie-pulse",
		Icon:           icons.TrayIconPNG(false),
		IconIsTemplate: true,
		ToolTip:        "Magpie Pulse",
	})
	if err != nil {
		return
	}
	a.tray = tray
	a.tray.OnClick(func() {
		a.toggleRail()
	})
	a.tray.OnRightClick(func() {
		a.syncMenuItems()
		a.trayMenu.Popup(nil)
	})
}

func (a *App) syncMenuItems() {
	if a.menuToggleRail != nil {
		if a.railVisible {
			a.menuToggleRail.SetLabel("Hide Panel")
		} else {
			a.menuToggleRail.SetLabel("Show Panel")
		}
	}
	if a.menuCached != nil {
		a.menuCached.SetChecked(a.cfg.IncludeCachedAccounts)
	}
	if a.menuAutoCollapse != nil {
		a.menuAutoCollapse.SetChecked(a.cfg.AutoCollapse)
	}
	for mode, item := range a.surfaceItems {
		item.SetChecked(a.cfg.Surface == mode)
	}
	for edge, item := range a.dockItems {
		item.SetChecked(a.cfg.DockEdge == edge)
	}
}

func (a *App) createRailWindow(preferredBounds *mygo.Rectangle) {
	display := a.targetDisplay(preferredBounds)
	bounds := ComputeRailBounds(display.WorkArea, a.cfg, len(a.slots))
	a.lastSlotCount = len(a.slots)

	opts := mygo.WindowOptions{
		Title:         "Magpie Pulse",
		Width:         bounds.Width,
		Height:        bounds.Height,
		X:             bounds.X,
		Y:             bounds.Y,
		Hidden:        !a.railVisible,
		Frameless:     true,
		Transparent:   true,
		AlwaysOnTop:   true,
		DisableShadow: true,
		DisableResize: true,
		SkipTaskbar:   true,
		Panel:         true,
		Content:       ui.View(a.renderRail),
	}
	if a.cfg.DockEdge == model.DockFloating {
		m := model.ComputeMetrics(a.cfg)
		opts.CornerRadius = int(math.Round(float64(m.Width / 2)))
		if a.cfg.Surface == model.SurfaceGlass {
			opts.Vibrancy = mygo.VibrancyGlassClear
		}
	}

	win := mygo.NewWindow(opts)
	win.SetVisibleOnAllWorkspaces(true)
	if a.cfg.DockEdge == model.DockFloating && a.cfg.Surface == model.SurfaceGlass {
		win.SetActiveAppearance(true)
	}

	win.OnMove(func() {
		if a.suppressMove || win != a.railWin || a.railWin == nil || a.railWin.IsDestroyed() {
			return
		}
		a.onRailMoved()
	})

	win.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting {
			return
		}
		e.PreventDefault()
		a.hideRail()
	})
	win.OnResize(func() {
		win.Invalidate()
	})

	a.railWin = win
	a.updateRailInputRegion()
	a.updateWatchActive()
}

func (a *App) recreateRailWindow() {
	a.destroyCardWindow()
	old := a.railWin
	var prevBounds *mygo.Rectangle
	if old != nil && !old.IsDestroyed() {
		b := old.Bounds()
		if b.Width > 0 && b.Height > 0 {
			prevBounds = &b
		}
	}
	a.railWin = nil
	a.createRailWindow(prevBounds)
	if old != nil && !old.IsDestroyed() {
		old.Destroy()
	}
}

func (a *App) layoutRailWindow() {
	if a.railWin == nil || a.railWin.IsDestroyed() {
		return
	}
	display := a.targetDisplay(nil)
	bounds := ComputeRailBounds(display.WorkArea, a.cfg, len(a.slots))
	a.lastSlotCount = len(a.slots)

	a.suppressMove = true
	a.railWin.SetBounds(bounds)
	a.suppressMove = false

	a.updateRailInputRegion()
	a.railWin.Invalidate()
}

func (a *App) targetDisplay(preferredBounds *mygo.Rectangle) mygo.Display {
	if a.cfg.DockEdge == model.DockFloating && (a.cfg.FloatX != 0 || a.cfg.FloatY != 0) {
		return mygo.Screen.DisplayNearestPoint(mygo.Point{X: a.cfg.FloatX, Y: a.cfg.FloatY})
	}
	if preferredBounds != nil && preferredBounds.Width > 0 && preferredBounds.Height > 0 {
		return mygo.Screen.DisplayMatching(*preferredBounds)
	}
	if a.railWin != nil && !a.railWin.IsDestroyed() {
		b := a.railWin.Bounds()
		if b.Width > 0 && b.Height > 0 {
			return mygo.Screen.DisplayMatching(b)
		}
	}
	return mygo.Screen.PrimaryDisplay()
}

func (a *App) updateRailInputRegion() {
	if a.railWin == nil || a.railWin.IsDestroyed() {
		return
	}
	w, h := a.railWin.Size()
	region := ComputeSliverInputRegion(w, h, a.cfg, a.railExpanded)
	a.railWin.SetInputRegion(region)
}

func (a *App) updateWatchActive() {
	cardVisible := a.cardWin != nil && !a.cardWin.IsDestroyed() && a.cardWin.IsVisible()
	active := a.dragActive || (a.railVisible && ((a.cfg.AutoCollapse && a.cfg.DockEdge.IsDocked()) || cardVisible || !a.cardHideAt.IsZero()))
	a.watchActive.Store(active)
}

func (a *App) renderRail(c *ui.Context) {
	cfg := a.cfg
	if a.dragPreviewEdge != "" {
		cfg.DockEdge = a.dragPreviewEdge
	}
	a.railState.Config = cfg
	a.railState.Slots = a.slots
	a.railState.Expanded = a.railExpanded
	a.railState.HoveredSlotID = a.hoveredSlotID
	a.railState.SelectedSlotID = a.selectedSlotID
	panel.RailView(c, &a.railState)
}

func (a *App) renderCard(c *ui.Context) {
	if a.cardState.Slot != nil && a.railWin != nil && !a.railWin.IsDestroyed() && a.cardWin != nil && !a.cardWin.IsDestroyed() {
		if slotRect, ok := a.railState.SlotBounds[a.cardState.Slot.SlotID]; ok {
			a.cardState.PointerCenter = ComputeCardPointerCenter(a.railWin.Bounds(), slotRect, a.cardWin.Bounds(), a.cfg.DockEdge)
		}
	}
	panel.CardView(c, &a.cardState)
}

func (a *App) renderSettings(c *ui.Context) {
	settings.SettingsView(c, &a.settingsState)
}

func (a *App) onHoverSlot(slotID string, rect ui.Rect) {
	a.hoveredSlotID = slotID
	a.cardHideAt = time.Time{}
	a.collapseAt = time.Time{}
	a.edgeGraceUntil = time.Time{}
	if a.cfg.AutoCollapse && a.cfg.DockEdge.IsDocked() && !a.railExpanded {
		a.railExpanded = true
		a.railState.Expanded = true
		a.updateRailInputRegion()
	}
	a.showCardForSlot(slotID, rect)
	if a.railWin != nil && !a.railWin.IsDestroyed() {
		a.railWin.Invalidate()
	}
}

func (a *App) onLeaveRail() {
	a.hoveredSlotID = ""
	a.railState.HoveredSlotID = ""
	now := time.Now()
	if a.cardHideAt.IsZero() {
		a.cardHideAt = now.Add(cardHideGrace)
	}
	if a.cfg.AutoCollapse && a.cfg.DockEdge.IsDocked() && a.collapseAt.IsZero() {
		a.collapseAt = now.Add(railCollapseWait)
	}
	a.updateWatchActive()
	if a.railWin != nil && !a.railWin.IsDestroyed() {
		a.railWin.Invalidate()
	}
}

func (a *App) onClickSlot(slotID string) {
	if a.selectedSlotID == slotID {
		a.selectedSlotID = ""
		a.railState.SelectedSlotID = ""
		a.hideCard()
	} else {
		a.selectedSlotID = slotID
		a.railState.SelectedSlotID = slotID
		a.cardHideAt = time.Time{}
		a.collapseAt = time.Time{}
		if rect, ok := a.railState.SlotBounds[slotID]; ok {
			a.showCardForSlot(slotID, rect)
		} else {
			a.showCardForSlot(slotID, ui.Rect{})
		}
	}
	if a.railWin != nil && !a.railWin.IsDestroyed() {
		a.railWin.Invalidate()
	}
}

func (a *App) onRailRightClick() {
	a.syncMenuItems()
	if a.trayMenu != nil {
		a.trayMenu.Popup(a.railWin)
	}
}

func (a *App) ensureCardWindow(anchor mygo.Rectangle) {
	if a.cardWin != nil && !a.cardWin.IsDestroyed() {
		a.cardWin.SetAnchor(anchor)
		return
	}
	if a.railWin == nil || a.railWin.IsDestroyed() {
		return
	}

	m := model.ComputeMetrics(a.cfg)
	extraW := float32(0)
	if a.cfg.DockEdge == model.DockRight || a.cfg.DockEdge == model.DockLeft {
		extraW = m.PointerWidth
	}
	cardW := int(math.Ceil(float64(m.CardWidth + extraW)))

	fly := mygo.NewFlyout(mygo.FlyoutOptions{
		Parent:     a.railWin,
		Anchor:     anchor,
		Placement:  CardPlacementForEdge(a.cfg.DockEdge),
		Gap:        int(math.Round(float64(m.CardGap))),
		Width:      cardW,
		Height:     320,
		FitContent: true,
		MinWidth:   cardW,
		MaxWidth:   cardW,
		MinHeight:  80,
		MaxHeight:  620,
		Focusable:  false,
		Hidden:     true,
		Shadow:     false,
		Content:    ui.View(a.renderCard),
	})
	fly.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting {
			return
		}
		e.PreventDefault()
		a.selectedSlotID = ""
		a.hoveredSlotID = ""
		a.hideCard()
	})
	fly.OnResize(func() {
		if a.cardWin != nil && !a.cardWin.IsDestroyed() && a.cardWin.IsVisible() {
			a.cardWin.Invalidate()
		}
	})
	a.cardWin = fly
}

func (a *App) destroyCardWindow() {
	if a.cardWin != nil && !a.cardWin.IsDestroyed() {
		a.cardWin.Destroy()
	}
	a.cardWin = nil
}

func (a *App) showCardForSlot(slotID string, rect ui.Rect) {
	slot := findSlotByID(a.slots, slotID)
	if slot == nil {
		return
	}
	if a.railWin == nil || a.railWin.IsDestroyed() {
		return
	}
	rw, rh := a.railWin.Size()
	anchor := SlotAnchorRect(rect, rw, rh)

	sameSlot := a.cardState.Slot != nil && a.cardState.Slot.SlotID == slot.SlotID
	a.populateCardState(slot)
	if !sameSlot {
		a.cardState.ShowAllModels = false
		a.cardState.Scroll = ui.ScrollState{}
	}

	a.ensureCardWindow(anchor)
	if a.cardWin != nil && !a.cardWin.IsDestroyed() {
		a.cardWin.ShowInactive()
		a.cardWin.Invalidate()
	}
	a.updateWatchActive()
}

func (a *App) populateCardState(slot *model.RailSlot) {
	a.cardState.Slot = slot
	a.cardState.Config = a.cfg
	a.cardState.History = a.lookupHistory(slot.Account)
	a.cardState.Usage30d = a.snap.Usage30d
	a.cardState.Usage7d = a.snap.Usage7d
	a.cardState.UsageToday = a.snap.UsageToday
}

func (a *App) lookupHistory(acc model.AccountQuota) *model.AccountHistory {
	if len(a.snap.Histories) == 0 {
		return nil
	}
	keys := []string{
		acc.AccountKey(),
		strings.ToLower(acc.Provider) + "/" + strings.ToLower(acc.User),
		strings.ToLower(acc.Provider) + "/",
	}
	for _, k := range keys {
		if h, ok := a.snap.Histories[k]; ok {
			cp := h
			return &cp
		}
	}
	return nil
}

func (a *App) hideCard() {
	a.cardHideAt = time.Time{}
	if a.cardWin != nil && !a.cardWin.IsDestroyed() && a.cardWin.IsVisible() {
		a.cardWin.Hide()
	}
	a.updateWatchActive()
}

func (a *App) showRail() {
	a.railVisible = true
	if a.cfg.AutoCollapse && a.cfg.DockEdge.IsDocked() {
		a.railExpanded = true
		a.collapseAt = time.Now().Add(2 * time.Second)
	} else {
		a.railExpanded = true
	}
	if a.railWin == nil || a.railWin.IsDestroyed() {
		a.createRailWindow(nil)
	}
	a.layoutRailWindow()
	a.railWin.ShowInactive()
	a.railWin.Invalidate()
	a.syncMenuItems()
	a.updateWatchActive()
}

func (a *App) hideRail() {
	a.railVisible = false
	a.hoveredSlotID = ""
	a.selectedSlotID = ""
	a.hideCard()
	if a.railWin != nil && !a.railWin.IsDestroyed() {
		a.railWin.Hide()
	}
	a.syncMenuItems()
	a.updateWatchActive()
}

func (a *App) toggleRail() {
	if a.railVisible {
		a.hideRail()
	} else {
		a.showRail()
	}
}

func (a *App) openSettings() {
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		mygo.App.Show()
		return
	}

	a.settingsState.Config = a.cfg
	a.settingsState.Snapshot = a.snap
	a.syncUpdateState()

	win := mygo.NewWindow(mygo.WindowOptions{
		Title:         "Magpie Pulse Settings",
		Width:         680,
		Height:        600,
		MinWidth:      620,
		MinHeight:     460,
		TitleBarStyle: mygo.TitleBarHiddenInset,
		Vibrancy:      mygo.VibrancyTitlebar,
		StateKey:      "settings-v2",
		Content:       ui.View(a.renderSettings),
	})
	win.OnClosed(func() {
		a.settingsWin = nil
	})
	a.settingsWin = win
	win.Show()
	win.Focus()
	mygo.App.Show()
}

func (a *App) applyConfig(next model.Config) {
	next.Normalize()
	prev := a.cfg
	a.cfg = next
	_ = model.SaveConfig(next)

	if prev.ShowInDock != next.ShowInDock {
		applyActivationPolicy(next.ShowInDock)
	}
	if prev.OpenAtLogin != next.OpenAtLogin {
		a.applyOpenAtLogin(next.OpenAtLogin)
	}

	a.service.UpdateConfig(next)

	a.slots = model.BuildRailSlots(a.snap.Accounts, next, time.Now())
	a.railState.Config = next
	a.railState.Slots = a.slots
	a.cardState.Config = next
	a.settingsState.Config = next

	if !next.AutoCollapse || !next.DockEdge.IsDocked() {
		a.railExpanded = true
		a.collapseAt = time.Time{}
		a.edgeGraceUntil = time.Time{}
	} else if prev.AutoCollapse != next.AutoCollapse || prev.DockEdge != next.DockEdge {
		a.railExpanded = false
		a.collapseAt = time.Time{}
		a.edgeGraceUntil = time.Time{}
	}

	needRecreateRail := (prev.DockEdge == model.DockFloating) != (next.DockEdge == model.DockFloating) ||
		(next.DockEdge == model.DockFloating && (prev.Surface == model.SurfaceGlass) != (next.Surface == model.SurfaceGlass)) ||
		(next.DockEdge == model.DockFloating && prev.Scale != next.Scale)
	needRecreateCard := prev.DockEdge != next.DockEdge || prev.Scale != next.Scale

	if needRecreateRail {
		a.recreateRailWindow()
	} else {
		if needRecreateCard {
			a.destroyCardWindow()
		}
		a.layoutRailWindow()
	}

	if a.cardWin != nil && !a.cardWin.IsDestroyed() && a.cardWin.IsVisible() {
		if a.cardState.Slot != nil {
			if updated := findSlotByID(a.slots, a.cardState.Slot.SlotID); updated != nil {
				a.populateCardState(updated)
				a.cardWin.Invalidate()
			} else {
				a.hideCard()
			}
		}
	}

	alert, tip := WorstAlertSummary(a.slots, a.cfg, a.snap)
	if a.tray != nil {
		if alert != a.trayAlert {
			a.trayAlert = alert
			_ = a.tray.SetIcon(icons.TrayIconPNG(alert), !alert)
		}
		a.tray.SetToolTip(tip)
	}

	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Invalidate()
	}
	a.syncMenuItems()
	a.updateWatchActive()
}

func (a *App) refreshNow() {
	a.railState.Refreshing = true
	if a.railWin != nil && !a.railWin.IsDestroyed() {
		a.railWin.Invalidate()
	}
	go a.service.RefreshNow()
}

func (a *App) testConnection(rawURL string) {
	a.settingsState.Testing = true
	a.settingsState.TestStatus = ""
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Invalidate()
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		info, err := a.service.TestConnection(ctx, rawURL)
		if a.settingsWin == nil || a.settingsWin.IsDestroyed() {
			return
		}
		a.settingsWin.Update(func() {
			a.settingsState.Testing = false
			if err != nil {
				a.settingsState.TestOK = false
				a.settingsState.TestStatus = err.Error()
				return
			}
			ver := strings.TrimSpace(info.Version)
			if ver != "" && !strings.HasPrefix(ver, "v") {
				ver = "v" + ver
			}
			a.settingsState.TestOK = true
			if ver == "" {
				a.settingsState.TestStatus = fmt.Sprintf("Connected · %d models", info.Models)
			} else {
				a.settingsState.TestStatus = fmt.Sprintf("Connected · %s · %d models", ver, info.Models)
			}
		})
	}()
}

func (a *App) startPointerWatcher() {
	go func() {
		ticker := time.NewTicker(pointerPollRate)
		defer ticker.Stop()
		for {
			select {
			case <-a.stopCh:
				return
			case <-ticker.C:
				if !a.watchActive.Load() {
					continue
				}
				mygo.RunOnMain(a.pollPointerState)
			}
		}
	}()
}

func (a *App) onRailMoved() {
	a.dragActive = true
	a.lastMoveAt = time.Now()
	a.collapseAt = time.Time{}
	a.edgeGraceUntil = time.Time{}

	if a.cardWin != nil && !a.cardWin.IsDestroyed() && a.cardWin.IsVisible() {
		a.hoveredSlotID = ""
		a.selectedSlotID = ""
		a.railState.HoveredSlotID = ""
		a.railState.SelectedSlotID = ""
		a.hideCard()
	}

	winBounds := a.railWin.Bounds()
	disp := mygo.Screen.DisplayMatching(winBounds)
	cursor := mygo.Screen.CursorScreenPoint()
	targetEdge, snapped := DetectSnapEdge(disp.WorkArea, winBounds, cursor, a.cfg.DockEdge)
	var preview model.DockEdge
	if (a.cfg.DockEdge == model.DockRight || a.cfg.DockEdge == model.DockLeft) && !snapped && targetEdge == model.DockFloating {
		preview = model.DockFloating
	}
	if preview != a.dragPreviewEdge {
		a.dragPreviewEdge = preview
		a.railWin.Invalidate()
	}
	a.updateWatchActive()
}

func (a *App) settleWindowMove() {
	a.dragActive = false
	a.dragPreviewEdge = ""
	if a.railWin == nil || a.railWin.IsDestroyed() {
		a.updateWatchActive()
		return
	}

	winBounds := a.railWin.Bounds()
	disp := mygo.Screen.DisplayMatching(winBounds)
	cursor := mygo.Screen.CursorScreenPoint()
	targetEdge, snapped := DetectSnapEdge(disp.WorkArea, winBounds, cursor, a.cfg.DockEdge)

	if snapped {
		newRatio := UpdateDockedRatioForTransition(disp.WorkArea, winBounds, targetEdge, a.cfg, len(a.slots))
		if targetEdge != a.cfg.DockEdge {
			next := a.cfg
			next.DockEdge = targetEdge
			next.RailRatio = newRatio
			a.applyConfig(next)
			return
		}
		a.cfg.RailRatio = newRatio
		a.railState.Config = a.cfg
		a.cardState.Config = a.cfg
		a.settingsState.Config = a.cfg
		_ = model.SaveConfig(a.cfg)
		a.layoutRailWindow()
		if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
			a.settingsWin.Invalidate()
		}
		a.updateWatchActive()
		return
	}

	if a.cfg.DockEdge != model.DockFloating {
		fx, fy := ComputeFloatingPosition(disp.WorkArea, winBounds, a.cfg, len(a.slots))
		next := a.cfg
		next.DockEdge = model.DockFloating
		next.FloatX = fx
		next.FloatY = fy
		a.applyConfig(next)
		return
	}

	maxX := disp.WorkArea.X + max(disp.WorkArea.Width-winBounds.Width, 0)
	maxY := disp.WorkArea.Y + max(disp.WorkArea.Height-winBounds.Height, 0)
	clampedX := clampInt(winBounds.X, disp.WorkArea.X, maxX)
	clampedY := clampInt(winBounds.Y, disp.WorkArea.Y, maxY)
	if clampedX == 0 && clampedY == 0 {
		clampedY = 1
	}
	if clampedX != a.cfg.FloatX || clampedY != a.cfg.FloatY {
		a.cfg.FloatX = clampedX
		a.cfg.FloatY = clampedY
		a.railState.Config = a.cfg
		a.cardState.Config = a.cfg
		a.settingsState.Config = a.cfg
		_ = model.SaveConfig(a.cfg)
	}
	if clampedX != winBounds.X || clampedY != winBounds.Y {
		a.layoutRailWindow()
	} else {
		a.railWin.Invalidate()
	}
	a.updateWatchActive()
}

func (a *App) pollPointerState() {
	if a.dragActive {
		if time.Since(a.lastMoveAt) >= dragSettleWait {
			a.settleWindowMove()
		}
		return
	}
	if !a.railVisible || a.railWin == nil || a.railWin.IsDestroyed() {
		return
	}
	cursor := mygo.Screen.CursorScreenPoint()
	railBounds := a.railWin.Bounds()
	disp := mygo.Screen.DisplayMatching(railBounds)
	now := time.Now()

	cardVisible := a.cardWin != nil && !a.cardWin.IsDestroyed() && a.cardWin.IsVisible()
	inCard := cardVisible && PointInRect(cursor, a.cardWin.Bounds(), 6)
	inEdge := a.cfg.AutoCollapse && a.cfg.DockEdge.IsDocked() && IsCursorOnDockEdge(disp.WorkArea, disp.Bounds, cursor, a.cfg.DockEdge)

	inRail := false
	if a.cfg.AutoCollapse && a.cfg.DockEdge.IsDocked() && !a.railExpanded {
		w, h := a.railWin.Size()
		slivers := ComputeSliverInputRegion(w, h, a.cfg, false)
		hitTrigger := inEdge
		if !hitTrigger {
			for _, sr := range slivers {
				screenSliver := mygo.Rectangle{
					X:      railBounds.X + sr.X,
					Y:      railBounds.Y + sr.Y,
					Width:  sr.Width,
					Height: sr.Height,
				}
				if PointInRect(cursor, screenSliver, 8) {
					hitTrigger = true
					break
				}
			}
		}
		if hitTrigger {
			a.railExpanded = true
			a.railState.Expanded = true
			a.collapseAt = time.Time{}
			if !PointInRect(cursor, railBounds, 6) {
				a.edgeGraceUntil = now.Add(edgeRevealGrace)
			} else {
				a.edgeGraceUntil = time.Time{}
			}
			a.updateRailInputRegion()
			a.railWin.Invalidate()
			inRail = PointInRect(cursor, railBounds, 6)
		}
	} else {
		inRail = PointInRect(cursor, railBounds, 6)
	}

	if inCard || inRail {
		a.cardHideAt = time.Time{}
		a.collapseAt = time.Time{}
		a.edgeGraceUntil = time.Time{}
		return
	}

	if cardVisible {
		if a.cardHideAt.IsZero() {
			a.cardHideAt = now.Add(cardHideGrace)
		} else if now.After(a.cardHideAt) {
			a.hoveredSlotID = ""
			a.selectedSlotID = ""
			a.railState.HoveredSlotID = ""
			a.railState.SelectedSlotID = ""
			a.hideCard()
			a.railWin.Invalidate()
		}
	} else if !a.cardHideAt.IsZero() && now.After(a.cardHideAt) {
		a.hideCard()
	}

	if a.cfg.AutoCollapse && a.cfg.DockEdge.IsDocked() && a.railExpanded && (a.cardWin == nil || !a.cardWin.IsVisible()) {
		if inEdge || (!a.edgeGraceUntil.IsZero() && now.Before(a.edgeGraceUntil)) {
			a.collapseAt = now.Add(railCollapseWait)
		} else if a.collapseAt.IsZero() {
			a.collapseAt = now.Add(railCollapseWait)
		} else if now.After(a.collapseAt) {
			a.collapseAt = time.Time{}
			a.edgeGraceUntil = time.Time{}
			a.railExpanded = false
			a.railState.Expanded = false
			a.updateRailInputRegion()
			a.railWin.Invalidate()
		}
	}
}
