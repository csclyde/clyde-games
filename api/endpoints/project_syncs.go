package endpoints

import (
	"api.clyde.games/models"
	"fmt"
	"github.com/gin-gonic/gin"
)

type discordCatalogItem struct {
	ID       string `json:"id"`
	GuildID  string `json:"guildId"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	CanRead  bool   `json:"canRead"`
	CanReact bool   `json:"canReact"`
}

func getDiscordCatalog(c *gin.Context) ([]discordCatalogItem, error) {
	var out struct {
		Items []discordCatalogItem `json:"items"`
	}
	e := discordBotCall(c.Request.Context(), "GET", "/source-catalog", nil, &out)
	return out.Items, e
}
func GetDiscordSourceCatalog(c *gin.Context) {
	items, e := getDiscordCatalog(c)
	if e != nil {
		c.JSON(502, gin.H{"error": "Little Clyde source catalog unavailable"})
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func validateDiscordSources(c *gin.Context, requested []models.DiscordSyncSource) ([]models.DiscordSyncSource, error) {
	if len(requested) > 20 {
		return nil, fmt.Errorf("at most 20 Discord sources per project")
	}
	if len(requested) == 0 {
		return []models.DiscordSyncSource{}, nil
	}
	catalog, e := getDiscordCatalog(c)
	if e != nil {
		return nil, fmt.Errorf("source permissions could not be verified")
	}
	return validateDiscordCatalogItems(requested, catalog)
}
func validateDiscordCatalogItems(requested []models.DiscordSyncSource, catalog []discordCatalogItem) ([]models.DiscordSyncSource, error) {
	seen := map[string]bool{}
	verified := []models.DiscordSyncSource{}
	for _, r := range requested {
		if !validSnowflake(r.ID) || seen[r.ID] {
			return nil, fmt.Errorf("duplicate or invalid Discord source ID")
		}
		seen[r.ID] = true
		found := false
		for _, v := range catalog {
			if v.ID != r.ID {
				continue
			}
			if v.Kind != r.Kind {
				return nil, fmt.Errorf("source type does not match Discord")
			}
			if r.Enabled && (!v.CanRead || !v.CanReact) {
				return nil, fmt.Errorf("Little Clyde needs view, history and reaction permissions")
			}
			verified = append(verified, models.DiscordSyncSource{ID: v.ID, GuildID: v.GuildID, Kind: v.Kind, Name: v.Name, Enabled: r.Enabled})
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("Discord source is unavailable to Little Clyde")
		}
	}
	return verified, nil
}
func attachProjectStatsDiscord(projects []models.ProjectStats) error {
	all, e := models.SelectDiscordSyncSources("", false)
	if e != nil {
		return e
	}
	for i := range projects {
		projects[i].DiscordSources = []models.DiscordSyncSource{}
		for _, s := range all {
			if s.ProjectID == projects[i].ID {
				projects[i].DiscordSources = append(projects[i].DiscordSources, s)
			}
		}
	}
	return nil
}
