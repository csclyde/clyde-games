package endpoints

import (
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
