package main

// import "net/http"
import (
	"api.clyde.games/endpoints"
	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func main() {
	if err := loadAvailableDotEnv(); err != nil {
		panic(err)
	}

	router := gin.Default()
	router.Use(CORSMiddleware())
	router.GET("/feedback", endpoints.GetFeedback)
	router.POST("/feedback", endpoints.AddFeedback)
	router.GET("/resolvefeedback", endpoints.ResolveFeedback)
	router.GET("/planka/hierarchy", endpoints.GetPlankaHierarchy)
	router.POST("/planka/tickets", endpoints.CreatePlankaTicket)
	router.POST("/steamreviews/import", endpoints.ImportSteamReviewsNow)
	router.POST("/reddit/import", endpoints.ImportRedditPostsNow)
	router.GET("/event", endpoints.GetEvent)
	router.GET("/event/builds", endpoints.GetEventBuilds)
	router.GET("/event/settings", endpoints.GetEventSettings)
	router.PUT("/event/settings", endpoints.UpdateEventSettings)
	router.POST("/event", endpoints.AddEvent)
	router.DELETE("/event/build", endpoints.DeleteEventsByBuild)
	router.GET("/crash", endpoints.GetCrash)
	router.GET("/crash/accessviolations", endpoints.GetAccessViolationCrashes)
	router.GET("/resolvecrash", endpoints.ResolveCrash)
	router.POST("/crash", endpoints.AddCrash)
	router.GET("/savegame", endpoints.GetSavegames)
	router.GET("/savegame/builds", endpoints.GetSavegameBuilds)
	router.GET("/system/stats", endpoints.GetSystemStats)
	router.POST("/savegame", endpoints.AddSavegame)
	router.GET("/savegame/download", endpoints.DownloadSavegame)
	router.DELETE("/savegame", endpoints.DeleteSavegame)
	router.DELETE("/savegame/build", endpoints.DeleteSavegamesByBuild)
	router.GET("/user/:id", endpoints.GetUserHistory)

	router.POST("/words/analyze", endpoints.AnalyzeWords)
	router.POST("/words/unknown", endpoints.GetUnknownWords)
	router.POST("/words/update", endpoints.UpdateWords)
	router.POST("/words/stats", endpoints.GetAllWordStats)

	var err error
	models.AnalyticsDB, err = models.GetDB("analytics")
	if err != nil {
		panic(err)
	}
	if err = models.MigrateAnalyticsDB(); err != nil {
		panic(err)
	}

	models.EtymologyDB, err = models.GetDB("etymology")
	if err != nil {
		panic(err)
	}
	if err = models.MigrateEtymologyDB(); err != nil {
		panic(err)
	}

	endpoints.StartSteamReviewImporter()
	endpoints.StartRedditPostImporter()

	router.Run(":9990")
}
