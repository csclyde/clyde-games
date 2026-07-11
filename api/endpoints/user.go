package endpoints

import (
	"net/http"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func GetUserHistory(c *gin.Context) {
	history, err := models.SelectUserHistory(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, history)
}
