package endpoints

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

const (
	redditSyncName         = "reddit_cursemark"
	redditFeedURL          = "https://www.reddit.com/r/cursemark/new.rss"
	redditDefaultUserAgent = "server:clyde-games-feedback:v1.0 (contact: https://clyde.games)"
	redditFeedCacheTTL     = 2 * time.Minute
)

var redditPostImportLock sync.Mutex
var redditFeedCache = struct {
	URL       string
	Feed      redditFeed
	FetchedAt time.Time
}{}

type redditFeed struct {
	Entries []redditFeedEntry `xml:"entry"`
}

type redditFeedEntry struct {
	ID        string           `xml:"id"`
	Title     string           `xml:"title"`
	Author    redditFeedAuthor `xml:"author"`
	Content   string           `xml:"content"`
	Published string           `xml:"published"`
	Updated   string           `xml:"updated"`
	Links     []redditFeedLink `xml:"link"`
}

type redditFeedAuthor struct {
	Name string `xml:"name"`
}

type redditFeedLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

func StartRedditPostImporter() {
	go func() {
		importRedditPostsFromCheckpoint()

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			importRedditPostsFromCheckpoint()
		}
	}()
}

func ImportRedditPostsNow(c *gin.Context) {
	imported, skipped, from, to, err := ImportRedditPostsFromCheckpoint()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"skipped":  skipped,
		"from":     from,
		"to":       to,
	})
}

func importRedditPostsFromCheckpoint() {
	imported, skipped, from, to, err := ImportRedditPostsFromCheckpoint()
	if err != nil {
		log.Printf("reddit post import failed: %v", err)
		return
	}

	log.Printf("reddit post import complete: from=%s to=%s imported=%d skipped=%d", from.Format(time.RFC3339), to.Format(time.RFC3339), imported, skipped)
}

func ImportRedditPostsFromCheckpoint() (int, int, time.Time, time.Time, error) {
	redditPostImportLock.Lock()
	defer redditPostImportLock.Unlock()

	state, err := models.GetSyncState(redditSyncName)
	if err != nil {
		return 0, 0, time.Time{}, time.Time{}, err
	}

	to := time.Now()
	from := to.Add(-24 * time.Hour)
	if state != nil && !state.LastSuccessAt.IsZero() {
		from = state.LastSuccessAt
	}

	imported, skipped, err := importRedditPostsSince(http.Client{Timeout: 12 * time.Second}, redditFeedURL, from, to)
	if err != nil {
		return imported, skipped, from, to, err
	}

	if _, err := models.SaveSyncSuccess(redditSyncName, to); err != nil {
		return imported, skipped, from, to, err
	}

	return imported, skipped, from, to, nil
}

func importRedditPostsSince(client http.Client, feedURL string, cutoff time.Time, now time.Time) (int, int, error) {
	feed, err := fetchRedditFeed(client, feedURL)
	if err != nil {
		return 0, 0, err
	}

	imported := 0
	skipped := 0
	for _, entry := range feed.Entries {
		createdAt, err := redditEntryTime(entry)
		if err != nil || createdAt.Before(cutoff) || createdAt.After(now) {
			continue
		}

		postURL := redditEntryURL(entry)
		if postURL == "" {
			continue
		}

		exists, err := models.FeedbackMessageContains(postURL)
		if err != nil {
			return imported, skipped, err
		}
		if exists {
			skipped++
			continue
		}

		feedback := models.Feedback{
			PID:      strings.TrimPrefix(strings.TrimSpace(entry.Author.Name), "/u/"),
			Project:  "Cursemark",
			Message:  redditFeedbackMessage(entry),
			Rating:   3,
			Env:      "production",
			Category: "Reddit Post",
			Platform: "Reddit",
		}
		feedback.CreatedAt = createdAt

		if _, err := models.AddFeedback(feedback); err != nil {
			return imported, skipped, err
		}
		imported++
	}

	return imported, skipped, nil
}

func fetchRedditFeed(client http.Client, feedURL string) (redditFeed, error) {
	if redditFeedCache.URL == feedURL && time.Since(redditFeedCache.FetchedAt) < redditFeedCacheTTL {
		return redditFeedCache.Feed, nil
	}

	req, err := http.NewRequest(http.MethodGet, feedURL, nil)
	if err != nil {
		return redditFeed{}, err
	}
	req.Header.Set("Accept", "application/atom+xml, application/rss+xml;q=0.9")
	req.Header.Set("User-Agent", redditFeedUserAgent())

	res, err := client.Do(req)
	if err != nil {
		return redditFeed{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return redditFeed{}, fmt.Errorf("reddit feed returned %s", res.Status)
	}

	var feed redditFeed
	if err := xml.NewDecoder(res.Body).Decode(&feed); err != nil {
		return redditFeed{}, err
	}
	redditFeedCache.URL = feedURL
	redditFeedCache.Feed = feed
	redditFeedCache.FetchedAt = time.Now()
	return feed, nil
}

func redditFeedUserAgent() string {
	if value := strings.TrimSpace(os.Getenv("REDDIT_USER_AGENT")); value != "" {
		return value
	}
	return redditDefaultUserAgent
}

func redditEntryTime(entry redditFeedEntry) (time.Time, error) {
	value := entry.Published
	if value == "" {
		value = entry.Updated
	}
	return time.Parse(time.RFC3339, value)
}

func redditEntryURL(entry redditFeedEntry) string {
	for _, link := range entry.Links {
		if link.Rel == "alternate" && link.Href != "" {
			return link.Href
		}
	}
	for _, link := range entry.Links {
		if link.Href != "" {
			return link.Href
		}
	}
	return ""
}

func redditEntryBody(entry redditFeedEntry) string {
	doc, err := html.Parse(strings.NewReader(entry.Content))
	if err != nil {
		return ""
	}
	return cleanText(textWithBreaks(doc))
}

func redditFeedbackMessage(entry redditFeedEntry) string {
	parts := []string{strings.TrimSpace(entry.Title)}
	if body := redditEntryBody(entry); body != "" && body != "[deleted]" && body != "[removed]" {
		parts = append(parts, body)
	}
	parts = append(parts, redditEntryURL(entry))
	return strings.Join(parts, "\n\n")
}
