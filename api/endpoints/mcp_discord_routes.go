package endpoints

import "github.com/gin-gonic/gin"

func registerMCPDiscord(g *gin.RouterGroup) {
	g.GET("/discord/batch", mcpDiscordBatch)
	g.GET("/discord/messages/:id", mcpDiscordDetail)
	g.POST("/discord/messages/:id/intake", mcpDiscordIntake)
	g.POST("/discord/messages/:id/disposition", mcpDiscordDisposition)

}
