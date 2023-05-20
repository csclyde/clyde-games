package endpoints

import (
	"fmt"
	"hash/fnv"
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

func FNV32a(text string) string {
	algorithm := fnv.New32a()
	algorithm.Write([]byte(text))
	return fmt.Sprint(algorithm.Sum32())
}

func AddCrash(c *gin.Context) {
	var crash models.Crash

	err := c.BindJSON(&crash)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	crash.Hash = FNV32a(crash.Message + crash.Stack)

	// open db connection
	updatedCrash, err := models.AddCrash(crash)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return

	}

	c.IndentedJSON(http.StatusCreated, updatedCrash)
}
