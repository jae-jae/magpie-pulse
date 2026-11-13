package magpie

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"magpie-pulse/internal/model"
)

type localQuotaSpan struct {
	Span  int64  `json:"span,omitempty"`
	Aside bool   `json:"aside,omitempty"`
	Model string `json:"model,omitempty"`
}

type localQuotaEntry struct {
	At    string             `json:"at"`
	Quota model.AccountQuota `json:"quota"`
	Spans []localQuotaSpan   `json:"spans"`
}

// DefaultMagpieDir returns the default Magpie configuration directory (~/.config/magpie).
func DefaultMagpieDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "magpie")
}

// ReadLocalQuotas reads cached quota snapshots and window span metadata from quotas.json.
func ReadLocalQuotas(dir string) (map[string]localQuotaEntry, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "quotas.json"))
	if err != nil {
		return nil, err
	}
	var parsed map[string]localQuotaEntry
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make(map[string]localQuotaEntry, len(parsed))
	for k, entry := range parsed {
		normKey := strings.ToLower(strings.TrimSpace(k))
		if prov, usr, ok := strings.Cut(normKey, "/"); ok {
			if strings.TrimSpace(entry.Quota.Provider) == "" {
				entry.Quota.Provider = prov
			}
			if strings.TrimSpace(entry.Quota.User) == "" {
				entry.Quota.User = usr
			}
		}
		if entry.Quota.ID == "" {
			if normKey != "" {
				entry.Quota.ID = normKey
			} else {
				entry.Quota.ID = entry.Quota.AccountKey()
			}
		}
		if entry.Quota.ReadAt == "" && entry.At != "" {
			entry.Quota.ReadAt = entry.At
		}
		key := entry.Quota.AccountKey()
		if normKey != "" {
			key = normKey
		}
		out[key] = entry
	}
	return out, nil
}

// ReadLocalHistories reads quota-history.json and normalizes keys from "provider|user" to "provider/user".
func ReadLocalHistories(dir string) (map[string]model.AccountHistory, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "quota-history.json"))
	if err != nil {
		return nil, err
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make(map[string]model.AccountHistory, len(parsed))
	for rawKey, val := range parsed {
		provider, user := splitHistoryKey(rawKey)
		var structured model.AccountHistory
		if err := json.Unmarshal(val, &structured); err == nil && len(structured.Lines) > 0 {
			if structured.Provider == "" {
				structured.Provider = provider
			}
			if structured.User == "" {
				structured.User = user
			}
			normKey := strings.ToLower(structured.Provider) + "/" + strings.ToLower(structured.User)
			out[normKey] = structured
			continue
		}

		var lineMap map[string][]model.HistoryPoint
		if err := json.Unmarshal(val, &lineMap); err != nil {
			return nil, err
		}
		lineNames := make([]string, 0, len(lineMap))
		for name := range lineMap {
			lineNames = append(lineNames, name)
		}
		sort.Strings(lineNames)
		lines := make([]model.HistoryLine, 0, len(lineNames))
		for _, name := range lineNames {
			lines = append(lines, model.HistoryLine{
				Name:   name,
				Points: lineMap[name],
			})
		}
		normKey := strings.ToLower(provider) + "/" + strings.ToLower(user)
		out[normKey] = model.AccountHistory{
			Provider: provider,
			User:     user,
			Lines:    lines,
		}
	}
	return out, nil
}

func splitHistoryKey(raw string) (string, string) {
	if prov, usr, ok := strings.Cut(raw, "|"); ok {
		return strings.TrimSpace(prov), strings.TrimSpace(usr)
	}
	if prov, usr, ok := strings.Cut(raw, "/"); ok {
		return strings.TrimSpace(prov), strings.TrimSpace(usr)
	}
	return strings.TrimSpace(raw), ""
}

// ReadDisabledProviders reads providers.json and returns a map of lowercased provider IDs that have off=true.
func ReadDisabledProviders(dir string) (map[string]bool, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "providers.json"))
	if err != nil {
		return nil, err
	}
	type providerEntry struct {
		ID  string `json:"id"`
		Off bool   `json:"off"`
	}
	trimmed := bytes.TrimSpace(raw)
	out := make(map[string]bool)

	if bytes.HasPrefix(trimmed, []byte("[")) {
		var list []providerEntry
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, err
		}
		for _, p := range list {
			if p.Off && p.ID != "" {
				out[strings.ToLower(p.ID)] = true
			}
		}
		return out, nil
	}

	var doc struct {
		Providers []providerEntry `json:"providers"`
	}
	if err := json.Unmarshal(trimmed, &doc); err != nil {
		return nil, err
	}
	for _, p := range doc.Providers {
		if p.Off && p.ID != "" {
			out[strings.ToLower(p.ID)] = true
		}
	}
	return out, nil
}

// EnrichAndMerge attaches SpanNanos and Aside metadata from quotas.json to live windows,
// and appends cached-only accounts from quotas.json when includeCached is true.
func EnrichAndMerge(live []model.AccountQuota, dir string, includeCached bool) []model.AccountQuota {
	result := make([]model.AccountQuota, len(live))
	seen := make(map[string]bool, len(live))
	for i, acc := range live {
		copied := cloneAccountQuota(acc)
		copied.ID = copied.AccountKey()
		result[i] = copied
		seen[copied.ID] = true
	}

	if dir == "" {
		return result
	}

	localQuotas, err := ReadLocalQuotas(dir)
	if err != nil || len(localQuotas) == 0 {
		return result
	}

	for i := range result {
		key := result[i].AccountKey()
		if entry, ok := localQuotas[key]; ok {
			applySpans(&result[i], entry)
		}
	}

	if !includeCached {
		return result
	}

	disabled, _ := ReadDisabledProviders(dir)
	var cachedKeys []string
	for key := range localQuotas {
		if !seen[key] {
			cachedKeys = append(cachedKeys, key)
		}
	}
	sort.Strings(cachedKeys)

	for _, key := range cachedKeys {
		entry := localQuotas[key]
		cached := cloneAccountQuota(entry.Quota)
		cached.ID = key
		cached.CachedOnly = true
		if disabled[strings.ToLower(cached.Provider)] {
			cached.Disabled = true
		}
		applySpans(&cached, entry)
		result = append(result, cached)
	}

	return result
}

func cloneAccountQuota(acc model.AccountQuota) model.AccountQuota {
	if len(acc.Windows) > 0 {
		wins := make([]model.QuotaWindow, len(acc.Windows))
		copy(wins, acc.Windows)
		acc.Windows = wins
	}
	return acc
}

func applySpans(acc *model.AccountQuota, entry localQuotaEntry) {
	if len(acc.Windows) == 0 || len(entry.Spans) == 0 {
		return
	}

	type winKey struct {
		name string
		pool string
	}
	byNamePool := make(map[winKey]localQuotaSpan, len(entry.Spans))
	byName := make(map[string]localQuotaSpan, len(entry.Spans))
	n := min(len(entry.Quota.Windows), len(entry.Spans))
	for i := 0; i < n; i++ {
		w := entry.Quota.Windows[i]
		sp := entry.Spans[i]
		byNamePool[winKey{name: w.Name, pool: w.Pool}] = sp
		if _, exists := byName[w.Name]; !exists {
			byName[w.Name] = sp
		}
	}

	for i := range acc.Windows {
		w := &acc.Windows[i]
		var sp localQuotaSpan
		var matched bool

		if i < len(entry.Spans) {
			if i >= len(entry.Quota.Windows) ||
				(entry.Quota.Windows[i].Name == w.Name && entry.Quota.Windows[i].Pool == w.Pool) {
				sp = entry.Spans[i]
				matched = true
			}
		}
		if !matched {
			if s, ok := byNamePool[winKey{name: w.Name, pool: w.Pool}]; ok {
				sp = s
				matched = true
			} else if s, ok := byName[w.Name]; ok {
				sp = s
				matched = true
			}
		}
		if matched {
			if sp.Span > 0 {
				w.SpanNanos = sp.Span
			}
			if sp.Aside {
				w.Aside = true
			}
		}
	}
}
