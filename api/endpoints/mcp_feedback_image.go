package endpoints

import (
	"errors"
	"net/http"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Shares the browser image policy: a matching open report is required.
func mcpFeedbackImage(c *gin.Context) {
	id, ok := mcpID(c)
	if !ok {
		return
	}
	project, ok := mcpReportProject(c)
	if !ok {
		return
	}
	if project == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "choose one canonical project for this screenshot"})
		return
	}
	data, err := models.SelectFeedbackImage(uint(id), *project)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		mcpDBError(c, err)
		return
	}
	if len(data) == 0 || len(data) > maxFeedbackImage {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/jpeg", data)
}
