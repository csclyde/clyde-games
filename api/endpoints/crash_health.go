package endpoints

import (
	"api.clyde.games/models"
	"errors"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
	"time"
)

func GetCrashHealth(c *gin.Context) {
	project := strings.TrimSpace(c.DefaultQuery("project", "all"))
	if project == "" || len(project) > 64 {
		c.JSON(400, gin.H{"error": "choose a project or all"})
		return
	}
	days, e := strconv.Atoi(c.DefaultQuery("days", "30"))
	if e != nil || (days != 7 && days != 30 && days != 90) {
		c.JSON(400, gin.H{"error": "days must be 7, 30 or 90"})
		return
	}
	health, e := models.SelectCrashHealth(models.AnalyticsDB.WithContext(c.Request.Context()), project, days, time.Now().UTC())
	if errors.Is(e, models.ErrCrashHealthLimit) {
		c.JSON(413, gin.H{"error": e.Error(), "complete": false})
		return
	}
	if e != nil {
		c.JSON(500, gin.H{"error": "crash health unavailable", "complete": false})
		return
	}
	c.JSON(200, health)
}
