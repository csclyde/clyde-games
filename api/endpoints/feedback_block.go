package endpoints

import (
	"errors"
	"net/http"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func BlockFeedbackUser(c *gin.Context)   { setFeedbackBlock(c, true) }
func UnblockFeedbackUser(c *gin.Context) { setFeedbackBlock(c, false) }

func setFeedbackBlock(c *gin.Context, blocked bool) {
	if err := models.SetFeedbackBlock(c.Param("id"), blocked); err != nil {
		if errors.Is(err, models.ErrInvalidFeedbackPlayerID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update feedback block"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"feedbackBlocked": blocked})
}

func GetFeedbackBlockedUsers(c *gin.Context) {
	blocks, err := models.SelectFeedbackBlocks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load feedback blocks"})
		return
	}
	c.JSON(http.StatusOK, blocks)
}
