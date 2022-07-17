package endpoints

import (
	"net/http"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func GetEvent(c *gin.Context) {

	// retrieve all feedback from the db
	events, err := models.SelectAllEvents()

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// check the result, and return it as JSON
	if len(events) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No events found"})

		return
	} else {
		c.IndentedJSON(http.StatusOK, events)
	}
}

func AddEvent(c *gin.Context) {
	var event models.Event

	if err := c.BindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// open db connection
	updatedEvent, err := models.AddEvent(event)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusCreated, updatedEvent)
}
