package endpoints

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"api.clyde.games/models"
)

var steamProfileID = regexp.MustCompile(`^7656[0-9]{13}$`)

type steamPersonaEntry struct {
	name    string
	expires time.Time
}
type steamPersonaResolver struct {
	client    *http.Client
	lock      sync.Mutex
	cache     map[string]steamPersonaEntry
	nextBatch time.Time
}

var feedbackSteamPersonas = steamPersonaResolver{
	client: &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) > 3 || r.URL.Scheme != "https" || r.URL.Host != "steamcommunity.com" || r.URL.User != nil {
				return http.ErrUseLastResponse
			}
			return nil
		},
	},
	cache: map[string]steamPersonaEntry{},
}

func loadSteamPersona(ctx context.Context, client *http.Client, id string) string {
	if !steamProfileID.MatchString(id) {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://steamcommunity.com/profiles/"+id+"/?xml=1", nil)
	if err != nil {
		return ""
	}
	response, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}
	var profile struct {
		XMLName xml.Name `xml:"profile"`
		ID      string   `xml:"steamID64"`
		Name    string   `xml:"steamID"`
	}
	if xml.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&profile) != nil || strings.TrimSpace(profile.ID) != id {
		return ""
	}
	name := strings.Join(strings.Fields(profile.Name), " ")
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > 128 || steamProfileID.MatchString(name) {
		return ""
	}
	return name
}

func isFeedbackSteamReview(row models.Feedback) bool {
	if !steamProfileID.MatchString(row.PID) {
		return false
	}
	if row.Source == "steam_reviews" {
		return true
	}
	if row.Source != "" || row.Platform != "Steam" || row.Category != "Steam Review" {
		return false
	}
	// Preserve compatibility with historical reviews whose source metadata has not been linked.
	expected := fmt.Sprintf("https://steamcommunity.com/profiles/%s/recommended/", row.PID)
	last := strings.TrimSpace(row.Message)
	if i := strings.LastIndexByte(last, '\n'); i >= 0 {
		last = strings.TrimSpace(last[i+1:])
	}
	return strings.HasPrefix(last, expected)
}

func (r *steamPersonaResolver) attach(ctx context.Context, feedback []models.Feedback) {
	now := time.Now()
	r.lock.Lock()
	var pending []string
	batchAllowed := !now.Before(r.nextBatch)
	seen := map[string]bool{}
	for i := range feedback {
		row := &feedback[i]
		if !isFeedbackSteamReview(*row) {
			continue
		}
		if hit, ok := r.cache[row.PID]; ok && now.Before(hit.expires) {
			row.DisplayAuthor = hit.name
			continue
		}
		if batchAllowed && len(pending) < 6 && !seen[row.PID] {
			pending = append(pending, row.PID)
			seen[row.PID] = true
		}
	}
	if len(pending) > 0 {
		r.nextBatch = now.Add(5 * time.Second)
	}
	r.lock.Unlock()
	if len(pending) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for _, id := range pending {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			name := loadSteamPersona(ctx, r.client, id)
			ttl := 6 * time.Hour
			if name == "" {
				ttl = 3 * time.Minute
			}
			r.lock.Lock()
			defer r.lock.Unlock()
			if len(r.cache) >= 4096 {
				for key, x := range r.cache {
					if time.Now().After(x.expires) {
						delete(r.cache, key)
					}
				}
				if len(r.cache) >= 4096 {
					for key := range r.cache {
						delete(r.cache, key)
						break
					}
				}
			}
			r.cache[id] = steamPersonaEntry{name: name, expires: time.Now().Add(ttl)}
		}(id)
	}
	wg.Wait()
	r.lock.Lock()
	defer r.lock.Unlock()
	for i := range feedback {
		if seen[feedback[i].PID] && isFeedbackSteamReview(feedback[i]) {
			feedback[i].DisplayAuthor = r.cache[feedback[i].PID].name
		}
	}
}
