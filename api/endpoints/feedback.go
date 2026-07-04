package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

const (
	defaultPlankaBaseURL      = "https://qa.clyde.games"
	defaultPlankaBoardID      = "1809767445724923501"
	defaultPlankaFeedbackList = "Feedback"
)

func GetFeedback(c *gin.Context) {

	// retrieve all feedback from the db
	feedback, err := models.SelectAllFeedback()

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// check the result, and return it as JSON
	if len(feedback) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No feedback found"})

		return
	} else {
		c.IndentedJSON(http.StatusOK, feedback)
	}
}

func AddFeedback(c *gin.Context) {
	var feedback models.Feedback

	if err := c.BindJSON(&feedback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// open db connection
	updatedFeedback, err := models.AddFeedback(feedback)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return

	}

	c.IndentedJSON(http.StatusCreated, updatedFeedback)
}

func ResolveFeedback(c *gin.Context) {

	models.ResolveFeedback(c.Query("id"))

}

func MakeFeedbackTicket(c *gin.Context) {
	feedback, err := models.SelectFeedback(c.Query("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	card, err := createPlankaFeedbackCard(*feedback)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"card": card})
}

type plankaList struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type plankaBoardResponse struct {
	Included struct {
		Lists []plankaList `json:"lists"`
	} `json:"included"`
}

type plankaCard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type plankaCardResponse struct {
	Item plankaCard `json:"item"`
}

func createPlankaFeedbackCard(feedback models.Feedback) (*plankaCard, error) {
	apiKey := os.Getenv("PLANKA_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("PLANKA_API_KEY is not configured")
	}

	baseURL := envOrDefault("PLANKA_BASE_URL", defaultPlankaBaseURL)
	listID := os.Getenv("PLANKA_FEEDBACK_LIST_ID")
	if listID == "" {
		var err error
		listID, err = findPlankaFeedbackListID(baseURL, apiKey)
		if err != nil {
			return nil, err
		}
	}

	payload := map[string]interface{}{
		"type":        "story",
		"position":    65536,
		"name":        feedbackCardName(feedback),
		"description": feedbackCardDescription(feedback),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/lists/"+listID+"/cards", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	var response plankaCardResponse
	if err := doPlankaRequest(req, &response); err != nil {
		return nil, err
	}

	return &response.Item, nil
}

func findPlankaFeedbackListID(baseURL string, apiKey string) (string, error) {
	boardID := envOrDefault("PLANKA_FEEDBACK_BOARD_ID", defaultPlankaBoardID)
	listName := envOrDefault("PLANKA_FEEDBACK_LIST_NAME", defaultPlankaFeedbackList)

	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/boards/"+boardID, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("x-api-key", apiKey)

	var response plankaBoardResponse
	if err := doPlankaRequest(req, &response); err != nil {
		return "", err
	}

	for _, list := range response.Included.Lists {
		if strings.EqualFold(list.Name, listName) {
			return list.ID, nil
		}
	}

	return "", fmt.Errorf("Planka list %q not found on board %s", listName, boardID)
}

func doPlankaRequest(req *http.Request, target interface{}) error {
	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("Planka request failed: %s %s", res.Status, strings.TrimSpace(string(body)))
	}

	return json.NewDecoder(res.Body).Decode(target)
}

func feedbackCardName(feedback models.Feedback) string {
	message := strings.TrimSpace(feedback.Message)
	if message == "" {
		message = "Feedback"
	}

	runes := []rune(message)
	if len(runes) > 80 {
		message = string(runes[:77]) + "..."
	}

	return "Feedback: " + message
}

func feedbackCardDescription(feedback models.Feedback) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "Feedback ID: %d\n", feedback.ID)
	fmt.Fprintf(&builder, "Created: %s\n", feedback.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&builder, "Project: %s\n", valueOrUnknown(feedback.Project))
	fmt.Fprintf(&builder, "Platform: %s\n", valueOrUnknown(feedback.Platform))
	fmt.Fprintf(&builder, "Env: %s\n", valueOrUnknown(feedback.Env))
	fmt.Fprintf(&builder, "PID: %s\n", valueOrUnknown(feedback.PID))
	fmt.Fprintf(&builder, "Category: %s\n", valueOrUnknown(feedback.Category))
	fmt.Fprintf(&builder, "Rating: %d\n", feedback.Rating)
	fmt.Fprintf(&builder, "Build: %s\n", valueOrUnknown(feedback.Build))
	fmt.Fprintf(&builder, "Commit: %s\n\n", valueOrUnknown(feedback.Commit))
	fmt.Fprintf(&builder, "Message:\n%s", feedback.Message)

	return builder.String()
}

func envOrDefault(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}

	return value
}
