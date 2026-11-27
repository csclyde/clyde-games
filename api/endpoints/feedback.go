package endpoints

import (
	"errors"
	"net/http"
	"strings"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

type feedbackTranslationRequest struct {
	Translated string `json:"translated"`
}

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
		feedbackSteamPersonas.attach(c.Request.Context(), feedback)
		c.IndentedJSON(http.StatusOK, feedback)
	}
}

func AddFeedback(c *gin.Context) {
	feedback, screenshot, err := parseFeedbackUpload(c)
	if err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.Is(err, errFeedbackLimit) || errors.As(err, &maxErr):
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": "FEEDBACK_UPLOAD_TOO_LARGE", "error": errFeedbackLimit.Error()})
		case errors.Is(err, errFeedbackImage):
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"code": "FEEDBACK_IMAGE_INVALID", "error": errFeedbackImage.Error()})
		case errors.Is(err, errFeedbackBusy):
			c.Header("Retry-After", "2")
			c.JSON(http.StatusTooManyRequests, gin.H{"code": "FEEDBACK_IMAGE_BUSY", "error": errFeedbackBusy.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid feedback upload."})
		}
		return
	}

	updatedFeedback, err := models.AddFeedback(feedback, screenshot)
	if errors.Is(err, models.ErrFeedbackBlocked) {
		c.JSON(http.StatusForbidden, gin.H{"code": "FEEDBACK_BLOCKED", "error": "Feedback submissions are blocked for this player."})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return

	}

	c.IndentedJSON(http.StatusCreated, updatedFeedback)
}

func UpdateFeedbackTranslation(c *gin.Context) {
	var request feedbackTranslationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	request.Translated = strings.TrimSpace(request.Translated)
	if request.Translated == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Translated feedback is required"})
		return
	}

	feedback, err := models.UpdateFeedbackTranslation(c.Query("id"), request.Translated)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if feedback == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Feedback not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, feedback)
}

func ResolveFeedback(c *gin.Context) {
	feedback, err := models.ResolveFeedback(c.Query("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if feedback == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Feedback not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, feedback)
}

type feedbackCloseRequest struct {
	Disposition string `json:"disposition"`
}

func CloseFeedback(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	var request feedbackCloseRequest
	if c.ShouldBindJSON(&request) != nil ||
		(request.Disposition != models.FeedbackResolved && request.Disposition != models.FeedbackIgnored) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose resolved or ignored"})
		return
	}
	feedback, err := models.CloseFeedback(c.Query("id"), request.Disposition)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Feedback could not be closed"})
		return
	}
	if feedback == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Feedback not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, feedback)
}
