package magpie

import (
	"context"
	"sync"
	"time"

	"magpie-pulse/internal/model"
)

// AlertEvent describes a threshold crossing or quota exhaustion event for a window.
type AlertEvent struct {
	AccountKey string
	Title      string
	WindowName string
	UsedPct    float64
	Spent      bool
}

type alertKey struct {
	accountKey string
	windowName string
	resetsAt   string
	level      string
}

// Service manages background Magpie polling, snapshot state, and threshold alert deduplication.
type Service struct {
	Refreshing bool
	OnUpdate   func(model.Snapshot)
	OnAlert    func(AlertEvent)

	mu        sync.RWMutex
	refreshMu sync.Mutex
	cfg       model.Config
	client    *Client
	magpieDir string
	snapshot  model.Snapshot
	alerted   map[alertKey]bool
	running   bool
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	triggerCh chan struct{}
	resetCh   chan struct{}
}

// NewService creates a polling service configured with cfg.
func NewService(cfg model.Config) *Service {
	cfg.Normalize()
	return &Service{
		cfg:       cfg,
		client:    NewClient(cfg.MagpieURL),
		magpieDir: DefaultMagpieDir(),
		snapshot: model.Snapshot{
			Histories: make(map[string]model.AccountHistory),
		},
		alerted:   make(map[alertKey]bool),
		triggerCh: make(chan struct{}, 1),
		resetCh:   make(chan struct{}, 1),
	}
}

// Start begins background polling. Calling Start on an already-running service is a no-op.
func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true
	s.wg.Add(1)
	s.mu.Unlock()

	go s.loop(ctx)
}

// Stop halts background polling and waits for the polling goroutine to exit.
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	cancel := s.cancel
	s.cancel = nil
	s.running = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// UpdateConfig applies new settings and updates the active client and polling interval.
func (s *Service) UpdateConfig(cfg model.Config) {
	cfg.Normalize()

	s.mu.Lock()
	urlChanged := s.cfg.MagpieURL != cfg.MagpieURL
	cachedChanged := s.cfg.IncludeCachedAccounts != cfg.IncludeCachedAccounts
	intervalChanged := s.cfg.RefreshIntervalSecs != cfg.RefreshIntervalSecs
	s.cfg = cfg
	s.client = NewClient(cfg.MagpieURL)
	running := s.running
	s.mu.Unlock()

	if running {
		if intervalChanged {
			select {
			case s.resetCh <- struct{}{}:
			default:
			}
		}
		if urlChanged || cachedChanged {
			select {
			case s.triggerCh <- struct{}{}:
			default:
			}
		}
	}
}

// RefreshNow triggers an immediate snapshot refresh.
func (s *Service) RefreshNow() {
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()

	if running {
		select {
		case s.triggerCh <- struct{}{}:
		default:
		}
		return
	}
	s.refresh(context.Background())
}

// Snapshot returns the latest fetched snapshot.
func (s *Service) Snapshot() model.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

// IsRefreshing reports whether a refresh is currently in flight.
func (s *Service) IsRefreshing() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Refreshing
}

// TestConnection probes an arbitrary Magpie server URL without mutating service configuration.
func (s *Service) TestConnection(ctx context.Context, rawURL string) (ServerInfo, error) {
	return NewClient(rawURL).FetchInfo(ctx)
}

func (s *Service) loop(ctx context.Context) {
	defer s.wg.Done()

	s.refresh(ctx)

	interval := s.pollInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(ctx)
		case <-s.triggerCh:
			s.refresh(ctx)
			ticker.Reset(s.pollInterval())
		case <-s.resetCh:
			ticker.Reset(s.pollInterval())
		}
	}
}

func (s *Service) pollInterval() time.Duration {
	s.mu.RLock()
	secs := s.cfg.RefreshIntervalSecs
	s.mu.RUnlock()
	if secs < 5 {
		secs = 5
	}
	return time.Duration(secs) * time.Second
}

func (s *Service) refresh(ctx context.Context) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if ctx.Err() != nil {
		return
	}

	s.mu.Lock()
	client := s.client
	includeCached := s.cfg.IncludeCachedAccounts
	magpieDir := s.magpieDir
	s.Refreshing = true
	s.mu.Unlock()

	snap := client.FetchSnapshot(ctx, includeCached, magpieDir)

	s.mu.Lock()
	s.Refreshing = false
	if ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.snapshot = snap
	alerts := s.collectAlertsLocked(snap, s.cfg)
	onAlert := s.OnAlert
	onUpdate := s.OnUpdate
	s.mu.Unlock()

	if onAlert != nil {
		for _, ev := range alerts {
			onAlert(ev)
		}
	}
	if onUpdate != nil {
		onUpdate(snap)
	}
}

func (s *Service) collectAlertsLocked(snap model.Snapshot, cfg model.Config) []AlertEvent {
	if !cfg.NotifyOnThreshold {
		return nil
	}
	var events []AlertEvent
	for _, acc := range snap.Accounts {
		key := acc.AccountKey()
		if acc.CachedOnly || acc.Disabled || cfg.HiddenAccounts[key] {
			continue
		}
		title := acc.DisplayName()
		for _, w := range acc.Windows {
			if w.IsText {
				continue
			}
			spent := w.IsSpent()
			warning := !spent && w.Used >= float64(cfg.WarningThreshold)
			if !spent && !warning {
				continue
			}
			level := "warning"
			if spent {
				level = "spent"
			}
			ak := alertKey{
				accountKey: key,
				windowName: w.Name,
				resetsAt:   w.ResetsAt,
				level:      level,
			}
			if s.alerted[ak] {
				continue
			}
			s.alerted[ak] = true
			if spent {
				s.alerted[alertKey{
					accountKey: key,
					windowName: w.Name,
					resetsAt:   w.ResetsAt,
					level:      "warning",
				}] = true
			}
			events = append(events, AlertEvent{
				AccountKey: key,
				Title:      title,
				WindowName: w.Name,
				UsedPct:    w.Used,
				Spent:      spent,
			})
		}
	}
	return events
}
