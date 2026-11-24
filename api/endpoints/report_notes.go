package endpoints

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func RegisterReportNoteRoutes(router *gin.Engine) {
	registerReportNoteRoutes(router.Group("", AdminAuth))
	registerCrashWorkRoutes(router.Group("", AdminAuth))
}
func registerReportNoteRoutes(g *gin.RouterGroup) {
	g.GET("/report-investigations", getInvestigationStatuses)
	g.GET("/report-notes", reportNoteIndex)
	g.GET("/report-notes/:kind/:id", getReportNotes)
	g.POST("/report-notes/:kind/:id", appendReportNote)
}
func noteProject(c *gin.Context) (string, bool) {
	values, present := c.Request.URL.Query()["project"]
	if !present || len(values) != 1 || len(values[0]) > 64 || values[0] != models.NormalizeProjectID(values[0]) {
		return "", false
	}
	return values[0], true // Explicit empty project is a legitimate untagged legacy scope.
}
func noteScope(c *gin.Context) (string, string, bool) {
	project, ok := noteProject(c)
	kind := c.Param("kind")
	if kind == "" {
		kind = c.Query("kind")
	}
	if !ok || len(project) > 64 || project != models.NormalizeProjectID(project) || (kind != "feedback" && kind != "crash") {
		c.JSON(400, gin.H{"error": "explicit canonical project and kind feedback/crash required"})
		return "", "", false
	}
	return project, kind, true
}
func noteError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, models.ErrCrashGroupRequired):
		c.JSON(409, gin.H{"error": err.Error(), "code": "crash_group_required", "next": "get_crash_investigation then claim_crash_investigation; append group notes with append_crash_investigation_note"})
	case errors.Is(err, models.ErrNoteInput):
		c.JSON(400, gin.H{"error": "invalid note: concise text, request_key and allowed state/release required; evidence must match SHA256"})
	case errors.Is(err, models.ErrNoteConflict):
		c.JSON(409, gin.H{"error": err.Error()})
	default:
		mcpDBError(c, err)
	}
}
func noteCursor(c *gin.Context, name string) (uint64, bool) {
	s := c.DefaultQuery(name, "0")
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid " + name})
		return 0, false
	}
	return v, true
}
func noteLimit(c *gin.Context) (int, bool) {
	n, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || n < 1 || n > 50 {
		c.JSON(400, gin.H{"error": "limit must be 1..50"})
		return 0, false
	}
	return n, true
}
func getReportNotes(c *gin.Context) {
	project, kind, ok := noteScope(c)
	if !ok {
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
	db := mcpDB(c)
	row, err := models.FindNoteReport(db, kind, uint(id), project, false)
	if err != nil {
		noteError(c, err)
		return
	}
	var frontier uint64
	q := db.Model(&models.ReportNote{}).Where("kind = ? AND report_id = ? AND project = ?", kind, id, project)
	if err = q.Select("COALESCE(MAX(id),0)").Scan(&frontier).Error; err != nil {
		noteError(c, err)
		return
	}
	items := []models.ReportNote{}
	err = db.Where("kind = ? AND report_id = ? AND project = ? AND id > ? AND id <= ?", kind, id, project, after, frontier).Order("id ASC").Limit(limit + 1).Find(&items).Error
	if err != nil {
		noteError(c, err)
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := after
	if len(items) > 0 {
		next = uint64(items[len(items)-1].ID)
	}
	var investigation any
	if kind == "crash" {
		view, e := models.GetCrashWork(db, uint(id), project)
		if e != nil {
			crashWorkError(c, e)
			return
		}
		investigation = view
	} else {
		statuses, e := models.ReadInvestigationStatuses(db, kind, project, []uint{uint(id)}, &frontier)
		if e != nil {
			noteError(c, e)
			return
		}
		investigation = statuses[0]
	}

	row.Message = ""
	c.JSON(200, gin.H{"report": row, "investigation": investigation, "legacy_history_only": kind == "crash", "kind": kind, "items": items, "has_more": more, "next_after_id": next, "frontier_id": frontier, "limit": limit, "resolution_policy": "Notes never change resolution. The owner closes reports manually. local means not deployed or released."})
}
func appendReportNote(c *gin.Context) {
	project, kind, ok := noteScope(c)
	if !ok {
		return
	}
	id, ok := mcpID(c)
	if !ok {
		return
	}
	var in models.AppendReportNoteInput
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 48<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		c.JSON(400, gin.H{"error": "invalid note JSON"})
		return
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		c.JSON(400, gin.H{"error": "one JSON object required"})
		return
	}
	actor := "admin"
	if strings.HasPrefix(c.FullPath(), "/internal/mcp/") {
		actor = "mcp"
	}
	note, replay, err := models.AppendReportNote(mcpDB(c), kind, uint(id), project, actor, in)
	if err != nil {
		noteError(c, err)
		return
	}
	c.JSON(200, gin.H{"note": note, "replayed": replay, "resolution_changed": false})
}

// Bounded annotated-record index includes resolved records so their history remains discoverable.
func reportNoteIndex(c *gin.Context) {
	project, kind, ok := noteScope(c)
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
	table := "feedbacks"
	if kind == "crash" {
		table = "crashes"
	}
	type item struct {
		ID       uint   `json:"id"`
		Project  string `json:"project"`
		Resolved bool   `json:"resolved"`
		Message  string `json:"message"`
		Notes    int    `json:"notes"`
	}
	items := []item{}
	err := mcpDB(c).Table(table+" r").Select("r.id, r.project, r.resolved, r.message, COUNT(n.id) AS notes").Joins("JOIN report_notes n ON n.report_id = r.id AND n.kind = ? AND BINARY n.project = ?", kind, project).Where("r.deleted_at IS NULL AND r.id > ? AND (BINARY COALESCE(TRIM(r.project),'') = ? OR (? = 'cursemark' AND LOWER(TRIM(r.project)) = 'cursemark'))", after, project, project).Group("r.id").Order("r.id ASC").Limit(limit + 1).Scan(&items).Error
	if err != nil {
		noteError(c, err)
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := after
	for i := range items {
		items[i].Message, _ = mcpBound(items[i].Message, 240)
		next = uint64(items[i].ID)
	}
	c.JSON(200, gin.H{"items": items, "kind": kind, "project": project, "legacy_history_only": kind == "crash", "has_more": more, "next_after_id": next, "limit": limit})
}
