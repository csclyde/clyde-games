package endpoints

import (
	"strconv"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const marketingPageLimit = 50

func marketingMCP(c *gin.Context) bool { return c.GetBool("marketing_mcp") }

func marketingListQuery(c *gin.Context, portalOrder string) *gorm.DB {
	q := marketingDB(c).Where("project = ?", marketingProject)
	if c.Query("include_archived") != "1" {
		q = q.Where("archived_at IS NULL")
	}
	if marketingMCP(c) {
		after, _ := strconv.ParseUint(c.Query("after_id"), 10, 32)
		return q.Where("id > ?", after).Order("id ASC").Limit(marketingPageLimit + 1)
	}
	return q.Order(portalOrder)
}

func marketingListResult[T any](c *gin.Context, key string, items []T) gin.H {
	if !marketingMCP(c) {
		return gin.H{key: items}
	}
	more := len(items) > marketingPageLimit
	if more {
		items = items[:marketingPageLimit]
	}
	return gin.H{key: items, "has_more": more, "limit": marketingPageLimit}
}

func archiveMarketing(c *gin.Context, scope *gorm.DB) {
	var in struct {
		Archived *bool `json:"archived"`
		Version  uint  `json:"version"`
	}
	if !marketingJSON(c, &in) {
		return
	}
	if in.Archived == nil || in.Version == 0 {
		marketingError(c, 400, "Choose archive or restore and supply the current version.")
		return
	}
	var when *time.Time
	if *in.Archived {
		now := time.Now().UTC()
		when = &now
	}
	result := scope.Where("version = ?", in.Version).Updates(map[string]any{"archived_at": when, "version": in.Version + 1})
	if result.Error != nil {
		marketingDBError(c, result.Error)
		return
	}
	if result.RowsAffected != 1 {
		marketingError(c, 409, "This record changed or could not be found. Read it before retrying.")
		return
	}
	c.JSON(200, gin.H{"archived_at": when, "version": in.Version + 1})
}

func archiveMarketingBeat(c *gin.Context) {
	id, ok := marketingID(c, "beat")
	if !ok {
		return
	}
	archiveMarketing(c, marketingDB(c).Model(&models.MarketingBeat{}).Where("project = ? AND id = ?", marketingProject, id))
}
func archiveMarketingTarget(c *gin.Context) {
	id, ok := marketingID(c, "target")
	if !ok {
		return
	}
	archiveMarketing(c, marketingDB(c).Model(&models.MarketingTarget{}).Where("project = ? AND id = ?", marketingProject, id))
}
func archiveMarketingPlacement(c *gin.Context) {
	beat, ok := marketingID(c, "beat")
	if !ok {
		return
	}
	id, ok := marketingID(c, "placement")
	if !ok {
		return
	}
	archiveMarketing(c, marketingDB(c).Model(&models.MarketingPlacement{}).Where("project = ? AND beat_id = ? AND id = ?", marketingProject, beat, id))
}
