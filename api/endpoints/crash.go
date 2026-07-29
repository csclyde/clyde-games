package endpoints

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"time"

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

func GetAccessViolationCrashes(c *gin.Context) {
	days := 30
	if requestedDays, err := strconv.Atoi(c.DefaultQuery("days", "30")); err == nil && requestedDays > 0 && requestedDays <= 365 {
		days = requestedDays
	}
	crashes, err := models.SelectAccessViolationCrashesSince(time.Now().AddDate(0, 0, -days))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, crashes)
}

func GetCrashBuilds(c *gin.Context) {
	builds, err := models.SelectCrashBuilds()

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, builds)
}

func GetCrashSettings(c *gin.Context) {
	settings, err := models.GetCrashSettings()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func UpdateCrashSettings(c *gin.Context) {
	var settings models.CrashSettings
	if err := c.BindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := models.SaveOldestCrashBuild(settings.OldestBuild)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
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

func ResolveCrash(c *gin.Context) {
	if c.Query("all") == "true" {
		if err := models.ResolveAllCrashes(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"resolved": true})
		return
	}

	crash, err := models.ResolveCrash(c.Query("hash"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, crash)

}
