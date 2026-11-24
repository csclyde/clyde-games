package endpoints

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

var steamReviewImportLock sync.Mutex

type steamReviewsResponse struct {
	Success      *int                    `json:"success"`
	Cursor       string                  `json:"cursor"`
	QuerySummary steamReviewQuerySummary `json:"query_summary"`
	Reviews      []steamReview           `json:"reviews"`
}

type steamReviewQuerySummary struct {
	NumReviews      int    `json:"num_reviews"`
	ReviewScore     int    `json:"review_score"`
	ReviewScoreDesc string `json:"review_score_desc"`
	TotalPositive   int    `json:"total_positive"`
	TotalNegative   int    `json:"total_negative"`
	TotalReviews    int    `json:"total_reviews"`
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

func StartSteamReviewImporter() { startFeedbackSchedule("steam", importAllSteamReviewsFromCheckpoints) }

func nextSteamImportTime(now time.Time, location *time.Location) time.Time {
	localNow := now.In(location)
	next := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 5, 0, 0, 0, location)
	if !next.After(localNow) {
		next = time.Date(localNow.Year(), localNow.Month(), localNow.Day()+1, 5, 0, 0, 0, location)
	}
	return next
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

	ri, rs, from, to, re := runFeedbackSource(project, "steam_reviews", "steam_", func(from, to time.Time) (int, int, error) { return ImportSteamReviewsSince(project, from, to) })
	di, ds, _, _, de := runFeedbackSource(project, "steam_discussions", "steam_", func(from, to time.Time) (int, int, error) { return ImportSteamDiscussionsSince(project, from, to) })
	return ri + di, rs + ds, from, to, errors.Join(re, de)
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

	for page := 0; page < 100; page++ {

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
			if review.RecommendationID == "" || createdAt.IsZero() || review.TimestampCreated <= 0 {
				return imported, skipped, fmt.Errorf("steam review missing identity or timestamp")
			}
			if createdAt.After(now) {
				shouldContinue = true
				continue
			}
			if createdAt.Before(cutoff) {
				continue
			}

			shouldContinue = true
			reviewURL := steamReviewURL(review, project.SteamAppID)

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

			added, err := models.AddSourceFeedback(feedback, "steam_reviews", project.SteamAppID+":"+review.RecommendationID, reviewURL)
			if err != nil {
				return imported, skipped, err
			}

			if added {
				imported++
			} else {
				skipped++
			}
		}

		if !shouldContinue {
			return imported, skipped, nil
		}

		if response.Cursor == "" || response.Cursor == cursor {
			return imported, skipped, fmt.Errorf("steam reviews cursor did not advance")
		}
		cursor = response.Cursor
	}

	return imported, skipped, fmt.Errorf("steam reviews exceeded 100-page coverage limit")
}

func fetchSteamReviewsPage(client http.Client, appID string, cursor string, dayRange int) (steamReviewsResponse, error) {
	if _, err := strconv.ParseUint(appID, 10, 32); err != nil {
		return steamReviewsResponse{}, fmt.Errorf("invalid Steam app ID")
	}
	q := url.Values{"json": {"1"}, "filter": {"recent"}, "language": {"all"}, "purchase_type": {"all"}, "review_type": {"all"}, "filter_offtopic_activity": {"0"}, "num_per_page": {"100"}, "cursor": {cursor}}
	page, storeErr := fetchSteamReviews("https://store.steampowered.com/appreviews/"+appID+"?"+q.Encode(), client)
	if storeErr == nil {
		return page, nil
	}
	// The production host's store route currently redirects to Steam's homepage.
	// Preserve the existing working Web API transport as a bounded fallback.
	return requestSteamReviews(client, appID, map[string]interface{}{"filter": 1, "languages": []string{"all"}, "purchase_type": 1, "review_type": 0, "num_per_page": 100, "cursor": cursor, "day_range": dayRange})
}

func FetchSteamReviewSummary(client http.Client, appID string) (models.SteamReviewStats, error) {
	countedResponse, err := fetchSteamReviewSummaryForPurchaseType(client, appID, "steam")
	if err != nil {
		return models.SteamReviewStats{}, err
	}
	totalResponse, err := fetchSteamReviewSummaryForPurchaseType(client, appID, "all")
	if err != nil {
		return models.SteamReviewStats{}, err
	}

	countedSummary := countedResponse.QuerySummary
	totalSummary := totalResponse.QuerySummary
	positivePercent := 0
	positivePercentRaw := 0.0
	if countedSummary.TotalReviews > 0 {
		positivePercentRaw = float64(countedSummary.TotalPositive) / float64(countedSummary.TotalReviews) * 100
		positivePercent = int(positivePercentRaw)
	}

	return models.SteamReviewStats{
		CountedReviews:     countedSummary.TotalReviews,
		TotalReviews:       totalSummary.TotalReviews,
		PositiveReviews:    countedSummary.TotalPositive,
		NegativeReviews:    countedSummary.TotalNegative,
		PositivePercent:    positivePercent,
		PositivePercentRaw: positivePercentRaw,
		ReviewScore:        countedSummary.ReviewScore,
		ReviewScoreTag:     countedSummary.ReviewScoreDesc,
	}, nil
}

func fetchSteamReviewSummaryForPurchaseType(client http.Client, appID string, purchaseType string) (steamReviewsResponse, error) {
	purchase := 0 // Steam purchases only, for the displayed review score.
	if purchaseType == "all" {
		purchase = 1
	}
	return requestSteamReviews(client, appID, map[string]interface{}{
		"filter": 1, "languages": []string{"all"}, "purchase_type": purchase,
		"review_type": 0, "num_per_page": 1,
	})
}

func requestSteamReviews(client http.Client, appID string, input map[string]interface{}) (steamReviewsResponse, error) {
	id, err := strconv.ParseUint(appID, 10, 32)
	if err != nil || id == 0 {
		return steamReviewsResponse{}, fmt.Errorf("invalid Steam app ID %q", appID)
	}
	input["appid"] = id
	encoded, err := json.Marshal(input)
	if err != nil {
		return steamReviewsResponse{}, err
	}
	query := url.Values{"input_json": {string(encoded)}}
	return fetchSteamReviews("https://api.steampowered.com/IUserReviewsService/GetAppReviews/v1/?"+query.Encode(), client)
}

func fetchSteamReviews(requestURL string, client http.Client) (steamReviewsResponse, error) {
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return steamReviewsResponse{}, err
	}
	req.Header.Set("User-Agent", "clyde-games-feedback-sync/1.0")
	res, err := feedbackGET(client, req)
	if err != nil {
		return steamReviewsResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return steamReviewsResponse{}, fmt.Errorf("steam reviews returned %s", res.Status)
	}

	var envelope struct {
		Response *steamReviewsResponse `json:"response"`
		steamReviewsResponse
	}
	if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
		return steamReviewsResponse{}, fmt.Errorf("steam reviews returned invalid JSON: %w", err)
	}

	if envelope.Response == nil && envelope.Success != nil {
		envelope.Response = &envelope.steamReviewsResponse
	}
	if envelope.Response != nil && envelope.Response.Success != nil && *envelope.Response.Success != 1 {
		return steamReviewsResponse{}, fmt.Errorf("steam reviews returned unsuccessful response")
	}
	if envelope.Response == nil {
		return steamReviewsResponse{}, fmt.Errorf("steam reviews response is missing review data")
	}

	return *envelope.Response, nil
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
	if project.SteamAppID == "" || !project.SteamSyncEnabled {
		return nil, fmt.Errorf("project %q has no Steam app ID", id)
	}
	return []models.Project{*project}, nil
}
