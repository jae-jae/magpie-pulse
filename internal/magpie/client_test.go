package magpie

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"magpie-pulse/internal/model"
)

func writeTempFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestDefaultMagpieDir(t *testing.T) {
	dir := DefaultMagpieDir()
	if dir == "" {
		t.Fatal("expected non-empty DefaultMagpieDir")
	}
	wantSuffix := filepath.Join(".config", "magpie")
	if !strings.HasSuffix(dir, wantSuffix) {
		t.Fatalf("DefaultMagpieDir() = %q, want suffix %q", dir, wantSuffix)
	}
}

func TestClientEndpointsAndSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "quotas.json", `{
		"claude/alice@example.com": {
			"at": "2026-11-13T10:00:00Z",
			"quota": {
				"provider": "claude",
				"name": "Claude",
				"windows": [
					{"name": "5 hours", "used": 40, "remaining": 60, "resetsAt": "2026-11-13T15:00:00Z"},
					{"name": "7 days", "used": 20, "remaining": 80, "resetsAt": "2026-11-20T00:00:00Z"}
				]
			},
			"spans": [
				{"span": 18000000000000},
				{"span": 604800000000000, "aside": true}
			]
		},
		"codex/bob@example.com": {
			"at": "2026-11-12T08:00:00Z",
			"quota": {
				"provider": "codex",
				"name": "Codex",
				"windows": [
					{"name": "7 days", "used": 55, "remaining": 45}
				]
			},
			"spans": [
				{"span": 604800000000000}
			]
		}
	}`)
	writeTempFile(t, dir, "providers.json", `{
		"providers": [
			{"id": "claude", "off": false},
			{"id": "codex", "off": true}
		]
	}`)
	writeTempFile(t, dir, "quota-history.json", `{
		"codex|bob@example.com": {
			"7 days": [
				{"at": "2026-11-12T08:00:00Z", "left": 45}
			]
		}
	}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"name":"magpie","version":"0.1.3078","models":30,"window":true,"apis":["/v1/magpie/quotas"]}`))
		case "/v1/magpie/quotas":
			_, _ = w.Write([]byte(`{"object":"list","data":[{
				"provider":"claude",
				"name":"Claude",
				"user":"alice@example.com",
				"windows":[
					{"name":"5 hours","used":42,"remaining":58,"resetsAt":"2026-11-13T15:00:00Z"},
					{"name":"7 days","used":25,"remaining":75,"resetsAt":"2026-11-20T00:00:00Z"}
				]
			}]}`))
		case "/v1/magpie/quotas/history":
			_, _ = w.Write([]byte(`{"object":"list","data":[{
				"provider":"Claude",
				"user":"Alice@example.com",
				"lines":[{"name":"5 hours","points":[{"at":"2026-11-13T10:00:00Z","left":58}]}]
			}]}`))
		case "/v1/magpie/usage":
			period := r.URL.Query().Get("period")
			switch period {
			case "30d", "1d":
				_, _ = w.Write([]byte(`{
					"period":"30d",
					"since":"2026-10-15T00:00:00Z",
					"calls":1000,
					"errors":5,
					"input":50000,
					"output":10000,
					"cost":12.5,
					"series":[
						{"label":"Nov 12","time":"2026-11-12T00:00:00Z","calls":400,"errors":2,"input":20000,"output":4000,"cost":5.0},
						{"label":"Nov 13","time":"2026-11-13T00:00:00Z","calls":120,"errors":1,"input":6000,"output":1500,"cache_read":3000,"reasoning":500,"cost":2.25}
					]
				}`))
			case "7d":
				_, _ = w.Write([]byte(`{"period":"7d","since":"2026-11-07T00:00:00Z","calls":520,"cost":7.25}`))
			default:
				http.Error(w, "bad period", http.StatusBadRequest)
			}
		case "/v1/magpie/statusline":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("claude alice 5h 42%\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL + "/")
	ctx := context.Background()

	info, err := client.FetchInfo(ctx)
	if err != nil {
		t.Fatalf("FetchInfo error: %v", err)
	}
	if info.Name != "magpie" || info.Version != "0.1.3078" || info.Models != 30 || !info.Window {
		t.Fatalf("unexpected info: %+v", info)
	}

	quotas, err := client.FetchQuotas(ctx)
	if err != nil {
		t.Fatalf("FetchQuotas error: %v", err)
	}
	if len(quotas) != 1 || quotas[0].ID != "claude/alice@example.com" {
		t.Fatalf("unexpected quotas: %+v", quotas)
	}

	histories, err := client.FetchHistories(ctx)
	if err != nil {
		t.Fatalf("FetchHistories error: %v", err)
	}
	if _, ok := histories["claude/alice@example.com"]; !ok {
		t.Fatalf("expected lowercased history key, got %+v", histories)
	}

	sl, err := client.FetchStatusLine(ctx)
	if err != nil {
		t.Fatalf("FetchStatusLine error: %v", err)
	}
	if sl != "claude alice 5h 42%" {
		t.Fatalf("FetchStatusLine = %q", sl)
	}

	snap := client.FetchSnapshot(ctx, true, dir)
	if !snap.Connected || snap.ConnectionErr != "" {
		t.Fatalf("expected connected snapshot, got connected=%v err=%q", snap.Connected, snap.ConnectionErr)
	}
	if snap.MagpieVersion != "0.1.3078" || snap.MagpieModelCnt != 30 {
		t.Fatalf("unexpected server metadata: version=%q models=%d", snap.MagpieVersion, snap.MagpieModelCnt)
	}
	if snap.StatusLine != "claude alice 5h 42%" {
		t.Fatalf("unexpected statusLine: %q", snap.StatusLine)
	}
	if snap.Usage30d == nil || snap.Usage30d.Calls != 1000 {
		t.Fatalf("unexpected Usage30d: %+v", snap.Usage30d)
	}
	if snap.Usage7d == nil || snap.Usage7d.Calls != 520 {
		t.Fatalf("unexpected Usage7d: %+v", snap.Usage7d)
	}
	if snap.UsageToday == nil || snap.UsageToday.Calls != 120 || snap.UsageToday.Cost != 2.25 || snap.UsageToday.Period != "1d" {
		t.Fatalf("expected UsageToday derived from 30d last day, got %+v", snap.UsageToday)
	}
	if _, ok := snap.Histories["codex/bob@example.com"]; !ok {
		t.Fatalf("expected local history merged for cached account, got %+v", snap.Histories)
	}

	if len(snap.Accounts) != 2 {
		t.Fatalf("expected 2 accounts (1 live + 1 cached), got %d", len(snap.Accounts))
	}
	liveAcc := snap.Accounts[0]
	if liveAcc.CachedOnly || liveAcc.Disabled {
		t.Fatalf("live account marked cached/disabled: %+v", liveAcc)
	}
	if liveAcc.Windows[0].SpanNanos != 18000000000000 || liveAcc.Windows[0].Aside {
		t.Fatalf("unexpected span on window 0: %+v", liveAcc.Windows[0])
	}
	if liveAcc.Windows[1].SpanNanos != 604800000000000 || !liveAcc.Windows[1].Aside {
		t.Fatalf("unexpected span/aside on window 1: %+v", liveAcc.Windows[1])
	}

	cachedAcc := snap.Accounts[1]
	if cachedAcc.AccountKey() != "codex/bob@example.com" || !cachedAcc.CachedOnly || !cachedAcc.Disabled {
		t.Fatalf("unexpected cached account: %+v", cachedAcc)
	}
	if cachedAcc.ReadAt != "2026-11-12T08:00:00Z" {
		t.Fatalf("expected ReadAt fallback to entry.At, got %q", cachedAcc.ReadAt)
	}
	if cachedAcc.Windows[0].SpanNanos != 604800000000000 {
		t.Fatalf("expected SpanNanos on cached window, got %d", cachedAcc.Windows[0].SpanNanos)
	}
}

func TestFetchSnapshotNative1dAndOfflineFallback(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "quotas.json", `{
		"gemini-web/u@example.com": {
			"at": "2026-11-13T09:00:00Z",
			"quota": {
				"provider": "gemini-web",
				"user": "u@example.com",
				"windows": [{"name": "5 hours", "used": 10, "remaining": 90}]
			},
			"spans": [{"span": 18000000000000}]
		}
	}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"name":"magpie","version":"0.2.0"}`))
		case "/v1/magpie/quotas":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/v1/magpie/quotas/history":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/v1/magpie/usage":
			if r.URL.Query().Get("period") == "1d" {
				_, _ = w.Write([]byte(`{"period":"1d","calls":42,"cost":1.5,"series":[{"label":"Today","calls":42,"cost":1.5}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"period":"30d","calls":999,"series":[{"label":"Today","calls":10}]}`))
		case "/v1/magpie/statusline":
			_, _ = w.Write([]byte("ok"))
		}
	}))

	client := NewClient(srv.URL)
	snap := client.FetchSnapshot(context.Background(), false, dir)
	if snap.UsageToday == nil || snap.UsageToday.Calls != 42 || snap.UsageToday.Period != "1d" {
		t.Fatalf("expected native 1d usage to be used, got %+v", snap.UsageToday)
	}

	srv.Close()
	offlineSnap := client.FetchSnapshot(context.Background(), true, dir)
	if offlineSnap.Connected || offlineSnap.ConnectionErr == "" {
		t.Fatalf("expected disconnected snapshot with error, got connected=%v err=%q", offlineSnap.Connected, offlineSnap.ConnectionErr)
	}
	if len(offlineSnap.Accounts) != 1 || !offlineSnap.Accounts[0].CachedOnly {
		t.Fatalf("expected cached account when offline and includeCached=true, got %+v", offlineSnap.Accounts)
	}
}

func TestEnrichAndMergeReorderedWindows(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "quotas.json", `{
		"antigravity/u@example.com": {
			"at": "2026-11-13T10:00:00Z",
			"quota": {
				"provider": "antigravity",
				"user": "u@example.com",
				"windows": [
					{"name": "5 hours", "pool": "Gemini"},
					{"name": "7 days", "pool": "Gemini"},
					{"name": "7 days", "pool": "Claude & GPT"}
				]
			},
			"spans": [
				{"span": 18000000000000},
				{"span": 604800000000000, "aside": true},
				{"span": 2592000000000000}
			]
		}
	}`)

	live := []model.AccountQuota{
		{
			Provider: "antigravity",
			User:     "u@example.com",
			Windows: []model.QuotaWindow{
				{Name: "7 days", Pool: "Claude & GPT", Used: 10},
				{Name: "5 hours", Pool: "Gemini", Used: 20},
			},
		},
	}

	enriched := EnrichAndMerge(live, dir, false)
	if len(enriched) != 1 {
		t.Fatalf("len(enriched) = %d, want 1", len(enriched))
	}
	if enriched[0].Windows[0].SpanNanos != 2592000000000000 || enriched[0].Windows[0].Aside {
		t.Fatalf("reordered window 0 mismatch: %+v", enriched[0].Windows[0])
	}
	if enriched[0].Windows[1].SpanNanos != 18000000000000 {
		t.Fatalf("reordered window 1 mismatch: %+v", enriched[0].Windows[1])
	}
	if live[0].Windows[0].SpanNanos != 0 {
		t.Fatalf("input slice mutated")
	}
}

func TestReadLocalHistoriesStructuredAndMap(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "quota-history.json", `{
		"antigravity|A@example.com": {
			"Gemini · 7 days": [{"at": "2026-11-13T00:00:00Z", "left": 80}],
			"Gemini · 5 hours": [{"at": "2026-11-13T00:00:00Z", "left": 90}]
		},
		"claude|b@example.com": {
			"provider": "claude",
			"user": "b@example.com",
			"lines": [{"name": "5 hours", "points": [{"at": "2026-11-13T01:00:00Z", "left": 50}]}]
		}
	}`)

	histories, err := ReadLocalHistories(dir)
	if err != nil {
		t.Fatalf("ReadLocalHistories error: %v", err)
	}
	h1, ok := histories["antigravity/a@example.com"]
	if !ok || len(h1.Lines) != 2 || h1.Lines[0].Name != "Gemini · 5 hours" {
		t.Fatalf("unexpected antigravity history: %+v (ok=%v)", h1, ok)
	}
	h2, ok := histories["claude/b@example.com"]
	if !ok || len(h2.Lines) != 1 || h2.Lines[0].Points[0].Left != 50 {
		t.Fatalf("unexpected claude history: %+v (ok=%v)", h2, ok)
	}
}

func TestServiceAlertDeduplicationAndLifecycle(t *testing.T) {
	var step atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"name":"magpie","version":"1.0.0","models":5}`))
		case "/v1/magpie/quotas":
			var accounts []model.AccountQuota
			switch step.Load() {
			case 0:
				accounts = []model.AccountQuota{
					{
						Provider: "claude",
						User:     "u@example.com",
						Windows: []model.QuotaWindow{
							{Name: "5 hours", Used: 60, ResetsAt: "2026-11-13T15:00:00Z"},
							{Name: "Note", Used: 100, IsText: true},
						},
					},
				}
			case 1:
				accounts = []model.AccountQuota{
					{
						Provider: "claude",
						User:     "u@example.com",
						Windows: []model.QuotaWindow{
							{Name: "5 hours", Used: 80, ResetsAt: "2026-11-13T15:00:00Z"},
						},
					},
				}
			case 2:
				accounts = []model.AccountQuota{
					{
						Provider: "claude",
						User:     "u@example.com",
						Windows: []model.QuotaWindow{
							{Name: "5 hours", Used: 85, ResetsAt: "2026-11-13T15:00:00Z"},
						},
					},
				}
			case 3:
				accounts = []model.AccountQuota{
					{
						Provider: "claude",
						User:     "u@example.com",
						Windows: []model.QuotaWindow{
							{Name: "5 hours", Used: 100, ResetsAt: "2026-11-13T15:00:00Z"},
						},
					},
				}
			case 4:
				accounts = []model.AccountQuota{
					{
						Provider: "claude",
						User:     "u@example.com",
						Windows: []model.QuotaWindow{
							{Name: "5 hours", Used: 100, ResetsAt: "2026-11-13T15:00:00Z"},
						},
					},
				}
			case 5:
				accounts = []model.AccountQuota{
					{
						Provider: "claude",
						User:     "u@example.com",
						Windows: []model.QuotaWindow{
							{Name: "5 hours", Used: 82, ResetsAt: "2026-11-13T20:00:00Z"},
						},
					},
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": accounts})
		case "/v1/magpie/quotas/history":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/v1/magpie/usage":
			_, _ = w.Write([]byte(`{"period":"30d","series":[]}`))
		case "/v1/magpie/statusline":
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()

	cfg := model.DefaultConfig()
	cfg.MagpieURL = srv.URL
	cfg.WarningThreshold = 75
	cfg.NotifyOnThreshold = true

	svc := NewService(cfg)
	svc.magpieDir = t.TempDir()

	var (
		mu     sync.Mutex
		alerts []AlertEvent
	)
	updates := make(chan model.Snapshot, 10)
	svc.OnAlert = func(ev AlertEvent) {
		mu.Lock()
		alerts = append(alerts, ev)
		mu.Unlock()
	}
	svc.OnUpdate = func(snap model.Snapshot) {
		updates <- snap
	}

	svc.RefreshNow()
	step.Store(1)
	svc.RefreshNow()
	step.Store(2)
	svc.RefreshNow()
	step.Store(3)
	svc.RefreshNow()
	step.Store(4)
	svc.RefreshNow()
	step.Store(5)
	svc.RefreshNow()

	mu.Lock()
	gotAlerts := append([]AlertEvent(nil), alerts...)
	mu.Unlock()

	if len(gotAlerts) != 3 {
		t.Fatalf("expected 3 alerts, got %d: %+v", len(gotAlerts), gotAlerts)
	}
	if gotAlerts[0].Spent || gotAlerts[0].UsedPct != 80 || gotAlerts[0].AccountKey != "claude/u@example.com" || gotAlerts[0].Title != "Claude" {
		t.Fatalf("unexpected alert[0]: %+v", gotAlerts[0])
	}
	if !gotAlerts[1].Spent || gotAlerts[1].UsedPct != 100 {
		t.Fatalf("unexpected alert[1]: %+v", gotAlerts[1])
	}
	if gotAlerts[2].Spent || gotAlerts[2].UsedPct != 82 {
		t.Fatalf("unexpected alert[2]: %+v", gotAlerts[2])
	}

	info, err := svc.TestConnection(context.Background(), srv.URL)
	if err != nil || info.Version != "1.0.0" {
		t.Fatalf("TestConnection failed: info=%+v err=%v", info, err)
	}

	for len(updates) > 0 {
		<-updates
	}
	svc.Start()
	select {
	case snap := <-updates:
		if !snap.Connected {
			t.Fatalf("expected connected snapshot on Start")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial Start update")
	}

	cfg.RefreshIntervalSecs = 10
	cfg.NotifyOnThreshold = false
	svc.UpdateConfig(cfg)
	svc.RefreshNow()

	select {
	case <-updates:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RefreshNow update")
	}

	if svc.IsRefreshing() {
		t.Fatal("expected IsRefreshing() to be false after update")
	}
	if !svc.Snapshot().Connected {
		t.Fatal("expected Snapshot().Connected to be true")
	}
	svc.Stop()
	svc.Stop()
}
