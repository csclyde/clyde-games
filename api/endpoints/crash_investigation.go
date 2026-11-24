package endpoints

import (
	"api.clyde.games/models"
	"errors"
	"github.com/gin-gonic/gin"
)

// Canonical group investigation read. Legacy report notes remain immutable history.
func getCrashInvestigation(c *gin.Context) {
	project, ok := noteProject(c)
	if !ok {
		c.JSON(400, gin.H{"error": "explicit canonical project required"})
		return
	}
	id, ok := mcpID(c)
	if !ok {
		return
	}
	after, ok := noteCursor(c, "after_id")
	if !ok {
		return
	}
	limit, ok := noteLimit(c)
	if !ok {
		return
	}
	families := c.Request.URL.Query()["family"]
	if len(families) > 1 || (c.Query("family") != "" && c.Query("family") != "access_violations") {
		c.JSON(400, gin.H{"error": "invalid family"})
		return
	}
	data, err := models.ReadCrashInvestigationLog(mcpDB(c), uint(id), project, c.Query("family"), after, limit)
	if errors.Is(err, models.ErrNoteInput) {
		c.JSON(400, gin.H{"error": "invalid crash investigation scope"})
		return
	}
	if err != nil {
		crashWorkError(c, err)
		return
	}
	c.JSON(200, data)
}
