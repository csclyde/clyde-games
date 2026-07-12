package endpoints

import (
	"net/http"
	"sort"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func GetProjects(c *gin.Context) {
	projects, err := models.SelectProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	c.JSON(http.StatusOK, projects)
}
