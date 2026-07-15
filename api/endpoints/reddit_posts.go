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
		importAllRedditPostsFromCheckpoints()

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			importAllRedditPostsFromCheckpoints()
		}
	}()
}

func ImportRedditPostsNow(c *gin.Context) {
	projects, err := redditProjectsForRequest(c.Query("project"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	imported, skipped := 0, 0
	var from, to time.Time
	for _, project := range projects {
		projectImported, projectSkipped, projectFrom, projectTo, syncErr := ImportRedditPostsFromCheckpoint(project)
		if syncErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": syncErr.Error()})
			return
		}
		imported += projectImported
		skipped += projectSkipped
		from = projectFrom
		to = projectTo
	}

	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"skipped":  skipped,
		"from":     from,
		"to":       to,
	})
}

func importAllRedditPostsFromCheckpoints() {
	projects, err := models.SelectProjectsWithSubreddit()
	if err != nil {
		log.Printf("reddit project lookup failed: %v", err)
		return
	}
	for _, project := range projects {
		imported, skipped, from, to, syncErr := ImportRedditPostsFromCheckpoint(project)
		if syncErr != nil {
			log.Printf("reddit import failed for %s: %v", project.ID, syncErr)
			continue
		}
		log.Printf("reddit import complete for %s: from=%s to=%s imported=%d skipped=%d", project.ID, from.Format(time.RFC3339), to.Format(time.RFC3339), imported, skipped)
	}
}

func ImportRedditPostsFromCheckpoint(project models.Project) (int, int, time.Time, time.Time, error) {
	redditPostImportLock.Lock()
	defer redditPostImportLock.Unlock()

	syncName := "reddit_" + project.ID
	state, err := models.GetSyncState(syncName)
	if err != nil {
		return 0, 0, time.Time{}, time.Time{}, err
	}

	to := time.Now()
	from := to.Add(-24 * time.Hour)
	if state != nil && !state.LastSuccessAt.IsZero() {
		from = state.LastSuccessAt
	}

	imported, skipped, err := importRedditPostsSince(http.Client{Timeout: 12 * time.Second}, redditFeedURL(project.SubredditURL), project.ID, from, to)
	if err != nil {
		return imported, skipped, from, to, err
	}

	if _, err := models.SaveSyncSuccess(syncName, to); err != nil {
		return imported, skipped, from, to, err
	}

	return imported, skipped, from, to, nil
}

func importRedditPostsSince(client http.Client, feedURL string, projectID string, cutoff time.Time, now time.Time) (int, int, error) {
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
			Project:  projectID,
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

func redditFeedURL(subredditURL string) string {
	value := strings.TrimRight(strings.TrimSpace(subredditURL), "/")
	if strings.HasSuffix(value, ".rss") {
		return value
	}
	if strings.HasSuffix(value, "/new") {
		return value + ".rss"
	}
	return value + "/new.rss"
}

func redditProjectsForRequest(id string) ([]models.Project, error) {
	if strings.TrimSpace(id) == "" || id == "all" {
		return models.SelectProjectsWithSubreddit()
	}
	project, err := models.SelectProject(id)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("project %q was not found", id)
	}
	if project.SubredditURL == "" {
		return nil, fmt.Errorf("project %q has no subreddit URL", id)
	}
	return []models.Project{*project}, nil
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
