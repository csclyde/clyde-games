package endpoints

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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

var crashMachineInfoStart = regexp.MustCompile(`(?im)^\s*(computer|system|hardware|operating system|os|os version|processor|cpu|gpu( model| memory)?|tracked gpu memory|graphics card|video card|memory|ram|directx|display adapter|render surface|window|scene|level|cursemark|scalegrid guard|physics contact guard|machine|device)\s*(information|info|details)?\s*[:=]`)

// crashFingerprint keeps the error and call stack as the crash identity while
// excluding diagnostic machine details that may be appended to newer reports.
// Truncating at the section boundary also preserves the legacy hash for reports
// that contain only the original message and stack.
func crashFingerprint(crash models.Crash) string {
	stack := crash.Stack
	if location := crashMachineInfoStart.FindStringIndex(stack); location != nil {
		stack = strings.TrimRight(stack[:location[0]], " \t\r\n")
	}
	return FNV32a(crash.Message + stack)
}

func AddCrash(c *gin.Context) {
	var crash models.Crash

	err := c.BindJSON(&crash)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	crash.Hash = crashFingerprint(crash)

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
