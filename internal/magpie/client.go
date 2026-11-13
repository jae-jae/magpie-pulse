package magpie

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"magpie-pulse/internal/model"
)

const defaultTimeout = 6 * time.Second

// ServerInfo describes metadata returned by Magpie's root endpoint.
type ServerInfo struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Models  int      `json:"models"`
	Window  bool     `json:"window"`
	APIs    []string `json:"apis"`
}

// Client communicates with a running Magpie HTTP server.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient constructs a Client with a 6-second HTTP timeout.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		HTTP:    &http.Client{Timeout: defaultTimeout},
	}
}

func (c *Client) baseURL() string {
	b := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if b == "" {
		return "http://127.0.0.1:3425"
	}
	return b
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: defaultTimeout}
}

func (c *Client) getBytes(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL()+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		if msg == "" {
			return nil, fmt.Errorf("magpie %s: status %d", path, resp.StatusCode)
		}
		return nil, fmt.Errorf("magpie %s: status %d: %s", path, resp.StatusCode, msg)
	}
	return body, nil
}

func (c *Client) getJSON(ctx context.Context, path string, dst any) error {
	body, err := c.getBytes(ctx, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("magpie %s: decode json: %w", path, err)
	}
	return nil
}

// FetchInfo calls GET <base>/ to retrieve server info.
func (c *Client) FetchInfo(ctx context.Context) (ServerInfo, error) {
	var info ServerInfo
	if err := c.getJSON(ctx, "/", &info); err != nil {
		return ServerInfo{}, err
	}
	return info, nil
}

// FetchQuotas calls GET <base>/v1/magpie/quotas and populates each account's ID.
func (c *Client) FetchQuotas(ctx context.Context) ([]model.AccountQuota, error) {
	var wrapper struct {
		Data []model.AccountQuota `json:"data"`
	}
	if err := c.getJSON(ctx, "/v1/magpie/quotas", &wrapper); err != nil {
		return nil, err
	}
	for i := range wrapper.Data {
		wrapper.Data[i].ID = wrapper.Data[i].AccountKey()
	}
	return wrapper.Data, nil
}

// FetchHistories calls GET <base>/v1/magpie/quotas/history and keys the result by "provider/user".
func (c *Client) FetchHistories(ctx context.Context) (map[string]model.AccountHistory, error) {
	var wrapper struct {
		Data []model.AccountHistory `json:"data"`
	}
	if err := c.getJSON(ctx, "/v1/magpie/quotas/history", &wrapper); err != nil {
		return nil, err
	}
	out := make(map[string]model.AccountHistory, len(wrapper.Data))
	for _, h := range wrapper.Data {
		key := strings.ToLower(h.Provider) + "/" + strings.ToLower(h.User)
		out[key] = h
	}
	return out, nil
}

// FetchUsage calls GET <base>/v1/magpie/usage?period=<period>.
func (c *Client) FetchUsage(ctx context.Context, period string) (*model.UsageSummary, error) {
	var summary model.UsageSummary
	path := "/v1/magpie/usage?period=" + url.QueryEscape(period)
	if err := c.getJSON(ctx, path, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// FetchStatusLine calls GET <base>/v1/magpie/statusline.
func (c *Client) FetchStatusLine(ctx context.Context) (string, error) {
	body, err := c.getBytes(ctx, "/v1/magpie/statusline")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// FetchSnapshot gathers live server state, usage summaries, histories, and merges local Magpie config data.
func (c *Client) FetchSnapshot(ctx context.Context, includeCached bool, magpieConfigDir string) model.Snapshot {
	if magpieConfigDir == "" {
		magpieConfigDir = DefaultMagpieDir()
	}

	var (
		wg         sync.WaitGroup
		info       ServerInfo
		infoErr    error
		quotas     []model.AccountQuota
		quotasErr  error
		histories  map[string]model.AccountHistory
		histErr    error
		usage30d   *model.UsageSummary
		usage7d    *model.UsageSummary
		usage1d    *model.UsageSummary
		statusLine string
		statusErr  error
	)

	wg.Add(7)
	go func() {
		defer wg.Done()
		info, infoErr = c.FetchInfo(ctx)
	}()
	go func() {
		defer wg.Done()
		quotas, quotasErr = c.FetchQuotas(ctx)
	}()
	go func() {
		defer wg.Done()
		histories, histErr = c.FetchHistories(ctx)
	}()
	go func() {
		defer wg.Done()
		usage30d, _ = c.FetchUsage(ctx, "30d")
	}()
	go func() {
		defer wg.Done()
		usage7d, _ = c.FetchUsage(ctx, "7d")
	}()
	go func() {
		defer wg.Done()
		usage1d, _ = c.FetchUsage(ctx, "1d")
	}()
	go func() {
		defer wg.Done()
		statusLine, statusErr = c.FetchStatusLine(ctx)
	}()
	wg.Wait()

	snap := model.Snapshot{
		FetchedAt: time.Now(),
		Histories: make(map[string]model.AccountHistory),
		Usage30d:  usage30d,
		Usage7d:   usage7d,
	}

	if infoErr == nil {
		snap.MagpieVersion = info.Version
		snap.MagpieModelCnt = info.Models
	}

	if infoErr == nil && quotasErr == nil {
		snap.Connected = true
	} else if infoErr != nil {
		snap.ConnectionErr = infoErr.Error()
	} else if quotasErr != nil {
		snap.ConnectionErr = quotasErr.Error()
	}

	if magpieConfigDir != "" {
		if localHist, err := ReadLocalHistories(magpieConfigDir); err == nil {
			for k, v := range localHist {
				snap.Histories[k] = v
			}
		}
	}
	if histErr == nil {
		for k, v := range histories {
			snap.Histories[k] = v
		}
	}

	snap.Accounts = EnrichAndMerge(quotas, magpieConfigDir, includeCached)
	snap.UsageToday = deriveTodayUsage(usage30d, usage7d, usage1d)
	if statusErr == nil {
		snap.StatusLine = statusLine
	}

	return snap
}

func deriveTodayUsage(u30, u7, u1 *model.UsageSummary) *model.UsageSummary {
	if u1 != nil && (strings.EqualFold(u1.Period, "1d") ||
		strings.EqualFold(u1.Period, "today") ||
		strings.EqualFold(u1.Period, "24h") ||
		(len(u1.Series) == 1 && (u30 == nil || len(u30.Series) != 1))) {
		return u1
	}

	var src *model.UsageSummary
	switch {
	case u30 != nil && len(u30.Series) > 0:
		src = u30
	case u7 != nil && len(u7.Series) > 0:
		src = u7
	case u1 != nil && len(u1.Series) > 0:
		src = u1
	}
	if src == nil {
		return nil
	}

	last := src.Series[len(src.Series)-1]
	return &model.UsageSummary{
		Period:     "1d",
		Since:      last.Time,
		Calls:      last.Calls,
		Errors:     last.Errors,
		Input:      last.Input,
		Output:     last.Output,
		CacheRead:  last.CacheRead,
		CacheWrite: last.CacheWrite,
		Reasoning:  last.Reasoning,
		Cost:       last.Cost,
		Series:     []model.UsageDay{last},
	}
}
