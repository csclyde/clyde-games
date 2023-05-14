package endpoints

import (
	"net/http"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func GetCrash(c *gin.Context) {

	// retrieve all crash from the db
	crash, err := models.SelectAllCrash()

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// check the result, and return it as JSON
	if len(crash) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No crash found"})

		return
	} else {
		c.IndentedJSON(http.StatusOK, crash)
	}
}

func AddCrash(c *gin.Context) {
	var crash models.Crash

	if err := c.BindJSON(&crash); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// open db connection
	updatedCrash, err := models.AddCrash(crash)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return

	}

	c.IndentedJSON(http.StatusCreated, updatedCrash)
}
