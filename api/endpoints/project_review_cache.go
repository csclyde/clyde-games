package endpoints

import (
	"api.clyde.games/models"
	"sync"
	"time"
)

// Cache only external Steam summaries; local report/event counts remain live.
// Entries are keyed by Steam app ID so editing a project's app ID takes effect immediately.
type reviewSummaryEntry struct {
	ready   chan struct{}
	summary models.SteamReviewStats
	err     error
	expires time.Time
}
type reviewSummaryCache struct {
	sync.Mutex
	entries map[string]*reviewSummaryEntry
}

var projectReviewCache = reviewSummaryCache{entries: make(map[string]*reviewSummaryEntry)}

func (cache *reviewSummaryCache) get(id string, fetch func() (models.SteamReviewStats, error)) (models.SteamReviewStats, error) {
	now := time.Now()
	cache.Lock()
	if entry := cache.entries[id]; entry != nil && (entry.expires.IsZero() || now.Before(entry.expires)) {
		cache.Unlock()
		<-entry.ready
		return entry.summary, entry.err
	}
	// Evict expired entries to bound old IDs after project edits.
	for key, entry := range cache.entries {
		if !entry.expires.IsZero() && !now.Before(entry.expires) {
			delete(cache.entries, key)
		}
	}
	entry := &reviewSummaryEntry{ready: make(chan struct{})}
	cache.entries[id] = entry
	cache.Unlock()
	summary, err := fetch()
	cache.Lock()
	entry.summary, entry.err = summary, err
	ttl := 5 * time.Minute
	if err != nil {
		ttl = 15 * time.Second
	}
	entry.expires = time.Now().Add(ttl)
	close(entry.ready)
	cache.Unlock()
	return summary, err
}
