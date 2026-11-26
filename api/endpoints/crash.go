package endpoints

import (
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	builds, err := models.SelectCrashBuilds(models.NormalizeProjectID(c.Query("project")))

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, builds)
}

func GetCrashSettings(c *gin.Context) {
	if c.Query("project") == "all" {
		c.JSON(400, gin.H{"error": "choose one project"})
		return
	}
	settings, err := models.GetCrashSettings(c.Query("project"))
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

	updated, err := models.SaveOldestCrashBuild(settings.OldestBuild, c.Query("project"))
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

// crashFingerprint preserves the legacy compatibility hash while
// excluding diagnostic machine details that may be appended to newer reports.
// Truncating at the section boundary also preserves the legacy hash for reports
// that contain only the original message and stack. Grouping uses the separate,
// project-scoped versioned identity in models.CrashIdentityV1.
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

	var reportID *string
	if models.NormalizeProjectID(crash.Project) == models.FarmBuilderProject {
		ids := c.Request.Header.Values(models.CrashReportIDHeader)
		if len(ids) > 1 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_CRASH_REPORT_ID", "error": "send a single crash report ID"})
			return
		}
		if len(ids) == 1 {
			reportID = &ids[0]
		}
	}
	updatedCrash, err := models.AddCrashWithReportID(crash, reportID)
	if errors.Is(err, models.ErrInvalidCrashReportID) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_CRASH_REPORT_ID", "error": err.Error()})
		return
	}

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

	var crash *models.Crash
	var err error
	if rawID := c.Query("id"); rawID != "" {
		id, parseErr := strconv.ParseUint(rawID, 10, 32)
		if parseErr != nil || id == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report id"})
			return
		}
		crash, err = models.ResolveCrashID(models.AnalyticsDB.WithContext(c.Request.Context()), id)
	} else {
		if c.Query("hash") == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "report id or hash required"})
			return
		}
		crash, err = models.ResolveCrash(c.Query("hash"), c.Query("project"))
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, crash)

}

func IgnoreCrash(c *gin.Context) {
	id, err := strconv.ParseUint(c.Query("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid report id"})
		return
	}
	crash, err := models.IgnoreCrashID(models.AnalyticsDB.WithContext(c.Request.Context()), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Crash report not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Crash group could not be ignored"})
		return
	}
	c.JSON(http.StatusOK, crash)
}
