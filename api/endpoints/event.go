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

func GetEventBuilds(c *gin.Context) {
	builds, err := models.SelectEventBuilds()

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, builds)
}

func GetEventSettings(c *gin.Context) {
	settings, err := models.GetEventSettings()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func UpdateEventSettings(c *gin.Context) {
	var settings models.EventSettings
	if err := c.BindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := models.SaveOldestEventBuild(settings.OldestBuild)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
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

func DeleteEventsByBuild(c *gin.Context) {
	build := c.Query("build")
	if build == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "build is required"})
		return
	}

	deleted, err := models.DeleteEventsByBuild(build, c.Query("project"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}
