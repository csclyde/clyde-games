package endpoints

import (
	"net/http"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
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
