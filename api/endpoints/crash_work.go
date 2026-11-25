package endpoints

import (
	"api.clyde.games/models"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
)

func crashWorkError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, models.ErrCrashClaim), errors.Is(e, models.ErrCrashClaimConflict):
		c.JSON(409, gin.H{"error": e.Error(), "code": "crash_group_claim_required"})
	case errors.Is(e, models.ErrCrashHealthLimit):
		c.JSON(413, gin.H{"error": "crash investigation index exceeds safety limit; choose a project", "index_complete": false})
	default:
		noteError(c, e)
	}
}
func crashWorkScope(c *gin.Context) (uint, string, bool) {
	p, ok := noteProject(c)
	if !ok {
		c.JSON(400, gin.H{"error": "explicit canonical project required"})
		return 0, "", false
	}
	id, ok := mcpID(c)
	return uint(id), p, ok
}
func crashJSON(c *gin.Context, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10))
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		c.JSON(400, gin.H{"error": "invalid group investigation JSON"})
		return false
	}
	if e := dec.Decode(new(any)); e != io.EOF {
		c.JSON(400, gin.H{"error": "one JSON object required"})
		return false
	}
	return true
}
func listCrashInvestigations(c *gin.Context) {
	project, ok := mcpReportProject(c)
	if !ok {
		return
	}
	p := "all"
	if project != nil {
		p = *project
	}
	out, e := models.ListCrashWork(mcpDB(c), p)
	if e != nil {
		crashWorkError(c, e)
		return
	}
	c.JSON(200, out)
}
func claimCrashInvestigation(c *gin.Context) {
	id, p, ok := crashWorkScope(c)
	if !ok {
		return
	}
	var in struct {
		RequestKey string `json:"request_key"`
		ClaimToken string `json:"claim_token,omitempty"`
	}
	if !crashJSON(c, &in) {
		return
	}
	out, e := models.ClaimCrashWork(mcpDB(c), id, p, in.RequestKey, in.ClaimToken)
	if e != nil {
		crashWorkError(c, e)
		return
	}
	c.JSON(200, out)
}
func appendCrashInvestigation(c *gin.Context) {
	id, p, ok := crashWorkScope(c)
	if !ok {
		return
	}
	var in models.AppendCrashWorkInput
	if !crashJSON(c, &in) {
		return
	}
	actor := "admin"
	if strings.HasPrefix(c.FullPath(), "/internal/mcp/") {
		actor = "mcp"
	}
	note, replay, e := models.AppendCrashWork(mcpDB(c), id, p, actor, in)
	if e != nil {
		crashWorkError(c, e)
		return
	}
	c.JSON(200, gin.H{"note": note, "replayed": replay, "resolution_changed": false, "unit_type": "crash_group"})
}
func registerCrashWorkRoutes(g *gin.RouterGroup) {
	g.GET("/crash-investigations", listCrashInvestigations)
	g.GET("/crash-investigation/:id", getCrashInvestigation)
	g.POST("/crash-investigation/:id/notes", appendCrashInvestigation)
}
