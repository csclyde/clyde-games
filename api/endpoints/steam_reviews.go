package endpoints

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

var steamReviewImportLock sync.Mutex

type steamReviewsResponse struct {
	Success int           `json:"success"`
	Cursor  string        `json:"cursor"`
	Reviews []steamReview `json:"reviews"`
}

type steamReview struct {
	RecommendationID string            `json:"recommendationid"`
	Review           string            `json:"review"`
	VotedUp          bool              `json:"voted_up"`
	TimestampCreated int64             `json:"timestamp_created"`
	Author           steamReviewAuthor `json:"author"`
}

type steamReviewAuthor struct {
	SteamID string `json:"steamid"`
}

func StartSteamReviewImporter() {
	go func() {
		importAllSteamReviewsFromCheckpoints()

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			importAllSteamReviewsFromCheckpoints()
		}
	}()
}

func ImportSteamReviewsNow(c *gin.Context) {
	projects, err := steamProjectsForRequest(c.Query("project"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	imported, skipped := 0, 0
	var from, to time.Time
	for _, project := range projects {
		projectImported, projectSkipped, projectFrom, projectTo, syncErr := ImportSteamReviewsFromCheckpoint(project)
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

func importAllSteamReviewsFromCheckpoints() {
	projects, err := models.SelectProjectsWithSteam()
	if err != nil {
		log.Printf("steam project lookup failed: %v", err)
		return
	}
	for _, project := range projects {
		imported, skipped, from, to, syncErr := ImportSteamReviewsFromCheckpoint(project)
		if syncErr != nil {
			log.Printf("steam import failed for %s: %v", project.ID, syncErr)
			continue
		}
		log.Printf("steam import complete for %s: from=%s to=%s imported=%d skipped=%d", project.ID, from.Format(time.RFC3339), to.Format(time.RFC3339), imported, skipped)
	}
}

func ImportSteamReviewsFromCheckpoint(project models.Project) (int, int, time.Time, time.Time, error) {
	steamReviewImportLock.Lock()
	defer steamReviewImportLock.Unlock()

	syncName := "steam_" + project.ID
	state, err := models.GetSyncState(syncName)
	if err != nil {
		return 0, 0, time.Time{}, time.Time{}, err
	}

	to := time.Now()
	from := to.Add(-24 * time.Hour)
	if state != nil && !state.LastSuccessAt.IsZero() {
		from = state.LastSuccessAt
	}

	imported, skipped, err := ImportSteamFeedbackSince(project, from, to)
	if err != nil {
		return imported, skipped, from, to, err
	}

	if _, err := models.SaveSyncSuccess(syncName, to); err != nil {
		return imported, skipped, from, to, err
	}

	return imported, skipped, from, to, nil
}

func ImportSteamFeedbackSince(project models.Project, cutoff time.Time, now time.Time) (int, int, error) {
	reviewImported, reviewSkipped, err := ImportSteamReviewsSince(project, cutoff, now)
	if err != nil {
		return reviewImported, reviewSkipped, err
	}

	discussionImported, discussionSkipped, err := ImportSteamDiscussionsSince(project, cutoff, now)
	if err != nil {
		return reviewImported + discussionImported, reviewSkipped + discussionSkipped, err
	}

	return reviewImported + discussionImported, reviewSkipped + discussionSkipped, nil
}

func ImportSteamReviewsSince(project models.Project, cutoff time.Time, now time.Time) (int, int, error) {
	client := http.Client{Timeout: 12 * time.Second}
	cursor := "*"
	dayRange := steamReviewDayRange(cutoff, now)
	imported := 0
	skipped := 0

	for page := 0; page < 20; page++ {
		response, err := fetchSteamReviewsPage(client, project.SteamAppID, cursor, dayRange)
		if err != nil {
			return imported, skipped, err
		}

		if len(response.Reviews) == 0 {
			return imported, skipped, nil
		}

		shouldContinue := false
		for _, review := range response.Reviews {
			createdAt := time.Unix(review.TimestampCreated, 0)
			if createdAt.Before(cutoff) {
				continue
			}

			shouldContinue = true
			reviewURL := steamReviewURL(review, project.SteamAppID)
			exists, err := models.FeedbackMessageContains(reviewURL)
			if err != nil {
				return imported, skipped, err
			}

			if exists {
				skipped += 1
				continue
			}

			feedback := models.Feedback{
				PID:      review.Author.SteamID,
				Project:  project.ID,
				Message:  strings.TrimSpace(review.Review) + "\n" + reviewURL,
				Rating:   steamReviewRating(review),
				Env:      "production",
				Category: "Steam Review",
				Platform: "Steam",
			}
			feedback.CreatedAt = createdAt

			_, err = models.AddFeedback(feedback)
			if err != nil {
				return imported, skipped, err
			}

			imported += 1
		}

		if !shouldContinue || response.Cursor == "" || response.Cursor == cursor {
			return imported, skipped, nil
		}

		cursor = response.Cursor
	}

	return imported, skipped, nil
}

func fetchSteamReviewsPage(client http.Client, appID string, cursor string, dayRange int) (steamReviewsResponse, error) {
	values := url.Values{}
	values.Set("json", "1")
	values.Set("filter", "recent")
	values.Set("language", "all")
	values.Set("purchase_type", "all")
	values.Set("num_per_page", "100")
	values.Set("day_range", fmt.Sprintf("%d", dayRange))
	values.Set("cursor", cursor)

	requestURL := "https://store.steampowered.com/appreviews/" + url.PathEscape(appID) + "?" + values.Encode()
	res, err := client.Get(requestURL)
	if err != nil {
		return steamReviewsResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return steamReviewsResponse{}, fmt.Errorf("steam reviews returned %s", res.Status)
	}

	var response steamReviewsResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return steamReviewsResponse{}, err
	}

	if response.Success != 1 {
		return steamReviewsResponse{}, fmt.Errorf("steam reviews response was not successful")
	}

	return response, nil
}

func steamReviewDayRange(cutoff time.Time, now time.Time) int {
	days := int(math.Ceil(now.Sub(cutoff).Hours() / 24))
	if days < 1 {
		return 1
	}

	return days
}

func steamReviewRating(review steamReview) uint8 {
	if review.VotedUp {
		return 5
	}

	return 1
}

func steamReviewURL(review steamReview, appID string) string {
	if review.Author.SteamID != "" {
		return "https://steamcommunity.com/profiles/" + review.Author.SteamID + "/recommended/" + appID + "/"
	}

	return "https://steamcommunity.com/app/" + appID + "/reviews/" + review.RecommendationID + "/"
}

func steamProjectsForRequest(id string) ([]models.Project, error) {
	if strings.TrimSpace(id) == "" || id == "all" {
		return models.SelectProjectsWithSteam()
	}
	project, err := models.SelectProject(id)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("project %q was not found", id)
	}
	if project.SteamAppID == "" {
		return nil, fmt.Errorf("project %q has no Steam app ID", id)
	}
	return []models.Project{*project}, nil
}
