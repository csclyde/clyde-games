package endpoints

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func GetProjects(c *gin.Context) {
	if c.Query("view") == "navigation" {
		projects, err := models.SelectProjectNavigation()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, projects)
		return
	}

	if c.Query("view") == "metadata" {
		projects, err := models.SelectProjectMetadata()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if e := models.AttachDiscordSources(projects); e != nil {
			c.JSON(500, gin.H{"error": "source configuration unavailable"})
			return
		}
		c.JSON(http.StatusOK, projects)
		return
	}
	projects, err := models.SelectProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if e := attachProjectStatsDiscord(projects); e != nil {
		c.JSON(500, gin.H{"error": "source configuration unavailable"})
		return
	}
	attachSteamReviewSummaries(projects)
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Name == projects[j].Name {
			return projects[i].ID < projects[j].ID
		}
		return projects[i].Name < projects[j].Name
	})
	c.JSON(http.StatusOK, projects)
}

func attachSteamReviewSummaries(projects []models.ProjectStats) {
	client := http.Client{Timeout: 6 * time.Second}
	var wg sync.WaitGroup
	for index := range projects {
		if strings.TrimSpace(projects[index].SteamAppID) == "" {
			continue
		}

		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			summary, err := projectReviewCache.get(projects[index].SteamAppID, func() (models.SteamReviewStats, error) {
				return FetchSteamReviewSummary(client, projects[index].SteamAppID)
			})
			if err != nil {
				projects[index].SteamReviews = &models.SteamReviewStats{UnavailableReason: err.Error()}
				return
			}
			projects[index].SteamReviews = &summary
		}(index)
	}
	wg.Wait()
}

type updateProjectRequest struct {
	SteamSyncEnabled *bool                       `json:"steamSyncEnabled"`
	DiscordSources   *[]models.DiscordSyncSource `json:"discordSources"`
	Name             string                      `json:"name"`
	SteamAppID       string                      `json:"steamAppId"`
}

func UpdateProject(c *gin.Context) {
	cutoff := time.Now().UTC()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	var request updateProjectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.DiscordSources != nil {
		verified, e := validateDiscordSources(c, *request.DiscordSources)
		if e != nil {
			c.JSON(400, gin.H{"error": e.Error()})
			return
		}
		request.DiscordSources = &verified
	}
	project, err := models.UpdateProjectWithSyncs(c.Param("id"), request.Name, request.SteamAppID, request.SteamSyncEnabled, request.DiscordSources, cutoff)
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
