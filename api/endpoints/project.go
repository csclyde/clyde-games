package endpoints

import (
	"net/http"
	"sort"
	"strings"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func GetProjects(c *gin.Context) {
	projects, err := models.SelectProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Name == projects[j].Name {
			return projects[i].ID < projects[j].ID
		}
		return projects[i].Name < projects[j].Name
	})
	c.JSON(http.StatusOK, projects)
}

type updateProjectRequest struct {
	Name         string `json:"name"`
	SteamAppID   string `json:"steamAppId"`
	SubredditURL string `json:"subredditUrl"`
}

func UpdateProject(c *gin.Context) {
	var request updateProjectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	project, err := models.UpdateProject(c.Param("id"), request.Name, request.SteamAppID, request.SubredditURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}
	c.JSON(http.StatusOK, project)
}
