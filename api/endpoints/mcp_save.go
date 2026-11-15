package endpoints

import (
	"encoding/base64"
	"net/http"
	"path"
	"strings"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

// Deliberate single-attachment retrieval, scoped to feedback rather than an
// arbitrary path or URL. Never execute save contents; they are untrusted input.
func mcpFeedbackSave(c *gin.Context) {
	id, ok := mcpID(c)
	if !ok {
		return
	}
	var save models.Savegame
	// Check metadata before loading the blob so historical oversized rows cannot
	// force an unbounded allocation. OCTET_LENGTH verifies the actual blob size.
	err := mcpDB(c).Model(&models.Savegame{}).Select("id", "filename", "content_type", "size", "hash").Where("feedback_id = ?", id).Order("created_at DESC, id DESC").First(&save).Error
	if err != nil {
		mcpDBError(c, err)
		return
	}
	var size int64
	if err := mcpDB(c).Model(&models.Savegame{}).Select("OCTET_LENGTH(data)").Where("id = ?", save.ID).Scan(&size).Error; err != nil {
		mcpDBError(c, err)
		return
	}
	if size > maxSavegameBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "save exceeds 1 MiB MCP investigation limit"})
		return
	}
	var data []byte
	if err := mcpDB(c).Model(&models.Savegame{}).Select("data").Where("id = ? AND OCTET_LENGTH(data) <= ?", save.ID, maxSavegameBytes).Row().Scan(&data); err != nil {
		mcpDBError(c, err)
		return
	}
	filename := path.Base(strings.ReplaceAll(save.Filename, "\\", "/"))
	filename, _ = mcpBound(filename, 255)
	c.JSON(http.StatusOK, gin.H{"feedback_id": id, "save_id": save.ID, "filename": filename, "size": len(data), "sha256": save.Hash, "content_type": save.ContentType, "encoding": "base64", "data": base64.StdEncoding.EncodeToString(data), "untrusted": true})
}
