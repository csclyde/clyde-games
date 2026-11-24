package endpoints

import (
	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// Triage and translation preserve the original message and resolution state.
func mcpTriageFeedback(c *gin.Context) {
	id, ok := mcpID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	var request struct {
		State      string  `json:"state"`
		Note       string  `json:"note"`
		Translated *string `json:"translated"`
	}
	if c.ShouldBindJSON(&request) != nil || (request.State != "pending" && request.State != "reviewed" && request.State != "noise") || len([]rune(request.Note)) > 2000 || (request.Translated != nil && len([]rune(*request.Translated)) > 30000) {
		c.JSON(400, gin.H{"error": "state must be pending, reviewed or noise; note <=2000 and translation <=30000 characters"})
		return
	}
	var item models.Feedback
	if err := mcpDB(c).First(&item, id).Error; err != nil {
		mcpDBError(c, err)
		return
	}
	updates := map[string]interface{}{"triage_state": request.State, "triage_note": request.Note, "triage_at": time.Now().UTC()}
	if request.Translated != nil {
		updates["translated"] = *request.Translated
	}
	if err := mcpDB(c).Model(&item).Updates(updates).Error; err != nil {
		mcpDBError(c, err)
		return
	}
	c.JSON(200, gin.H{"id": id, "triage_state": request.State, "resolved": item.Resolved, "original_message_preserved": true, "reversible": true})
}
