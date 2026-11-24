package endpoints

import (
	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

func getInvestigationStatuses(c *gin.Context) {
	project, kind, ok := noteScope(c)
	if !ok {
		return
	}
	parts := strings.Split(c.Query("ids"), ",")
	if len(parts) == 0 || len(parts) > 200 {
		c.JSON(400, gin.H{"error": "ids must contain 1..200 distinct positive report IDs"})
		return
	}
	ids := make([]uint, 0, len(parts))
	seen := map[uint64]bool{}
	for _, p := range parts {
		v, err := strconv.ParseUint(p, 10, 32)
		if err != nil || v == 0 || seen[v] {
			c.JSON(400, gin.H{"error": "ids must contain 1..200 distinct positive report IDs"})
			return
		}
		seen[v] = true
		ids = append(ids, uint(v))
	}
	if kind == "crash" {
		items := []models.CrashWorkView{}
		seen := map[string]bool{}
		for _, id := range ids {
			v, e := models.GetCrashWork(mcpDB(c), id, project)
			if e != nil {
				crashWorkError(c, e)
				return
			}
			if !seen[v.Key] {
				seen[v.Key] = true
				items = append(items, v)
			}
		}
		c.JSON(200, gin.H{"items": items, "kind": kind, "project": project, "unit_type": "crash_group", "complete": true})
		return
	}
	items, err := models.ReadInvestigationStatuses(mcpDB(c), kind, project, ids, nil)
	if err != nil {
		noteError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items, "kind": kind, "project": project, "complete": true, "resolution_policy": "Only the owner closes reports manually. ready_for_review is a fix handoff, not resolution. Concise chronological database notes are canonical; no Markdown/server files are required."})
}
