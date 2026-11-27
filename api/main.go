package main

// import "net/http"
import (
	"fmt"
	"os"

	"api.clyde.games/endpoints"
	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, "+models.CrashReportIDHeader+", X-Clyde-Event-ID")
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

	router := gin.New()
	router.Use(endpoints.SafePortalRequestLog(), endpoints.SafePortalRecovery())
	router.Use(CORSMiddleware())
	registerRoutes(router)

	var err error
	shared, cursemark, necrovale, farmBuilder := "analytics", "cursemark_metrics", "necrovale_metrics", "farm_builder_metrics"
	action := ""
	if len(os.Args) > 1 {
		if len(os.Args) >= 3 && os.Args[1] == "metrics-storage" {
			action = os.Args[2]
		}
		switch {
		case (action == "split" || action == "rollback") && len(os.Args) == 3:
		case (action == "split" || action == "rollback") && len(os.Args) == 6:
			shared, cursemark, necrovale = os.Args[3], os.Args[4], os.Args[5]
		case action == "add-farm-builder" && len(os.Args) == 3:
		case action == "add-farm-builder" && len(os.Args) == 5:
			shared, farmBuilder = os.Args[3], os.Args[4]
		default:
			fmt.Fprintln(os.Stderr, "Usage: api metrics-storage split|rollback [shared_schema cursemark_schema necrovale_schema]")
			fmt.Fprintln(os.Stderr, "       api metrics-storage add-farm-builder [shared_schema farm_builder_schema]")
			os.Exit(2)
		}
	}
	models.AnalyticsDB, err = models.GetDB(shared)
	if err != nil {
		panic(err)
	}
	if action != "" {
		if action == "add-farm-builder" {
			err = models.ProvisionFarmBuilderMetrics(models.AnalyticsDB, farmBuilder)
		} else {
			err = models.MigrateGameMetrics(models.AnalyticsDB, cursemark, necrovale, action == "rollback", os.Stdout)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if _, err = models.ConfigureGameMetrics(cursemark, necrovale); err != nil {
		panic(err)
	}
	if _, err = models.ConfigureFarmBuilderMetrics(farmBuilder); err != nil {
		panic(err)
	}
	if err = models.MigrateAnalyticsDB(); err != nil {
		panic(err)
	}

	models.EtymologyDB, err = models.GetDB("etymology")
	if err != nil {
		panic(err)
	}
	if err = models.MigrateReportNotes(models.AnalyticsDB); err != nil {
		panic(err)
	}
	if err = models.MigrateCrashInvestigations(models.AnalyticsDB); err != nil {
		panic("crash investigation migration failed")
	}
	if err = models.MigratePortalAuth(models.AnalyticsDB); err != nil {
		panic("portal authentication schema migration failed")
	}
	if err = models.MigrateMarketing(models.AnalyticsDB); err != nil {
		panic("marketing schema migration failed")
	}
	if err = models.MigrateEtymologyDB(); err != nil {
		panic(err)
	}

	endpoints.StartSteamReviewImporter()
	endpoints.StartDiscordFeedbackImporter()
	endpoints.StartDiscordCompletionWorker()

	router.Run(":9990")
}

// registerRoutes keeps routing testable without connecting production databases.
func registerRoutes(router *gin.Engine) {
	endpoints.RegisterPortalAuthRoutes(router)
	endpoints.RegisterMarketingRoutes(router)
	endpoints.RegisterReportNoteRoutes(router)
	endpoints.RegisterMCPRoutes(router)
	router.GET("/metrics", endpoints.MetricsAuth, endpoints.GetMetrics)
	router.GET("/metrics/:id", endpoints.MetricsAuth, endpoints.GetMetric)
	router.GET("/feedback", endpoints.AdminAuth, endpoints.GetFeedback)
	router.POST("/feedback", endpoints.AddFeedback)
	router.GET("/feedback/image", endpoints.AdminAuth, endpoints.GetFeedbackImage)
	router.PUT("/feedback/translation", endpoints.AdminAuth, endpoints.UpdateFeedbackTranslation)
	router.POST("/resolvefeedback", endpoints.AdminAuth, endpoints.ResolveFeedback)
	router.POST("/feedback/close", endpoints.AdminAuth, endpoints.CloseFeedback)
	router.GET("/planka/hierarchy", endpoints.AdminAuth, endpoints.GetPlankaHierarchy)
	router.POST("/planka/tickets", endpoints.AdminAuth, endpoints.CreatePlankaTicket)
	router.POST("/steamreviews/import", endpoints.AdminAuth, endpoints.ImportSteamReviewsNow)
	router.GET("/event", endpoints.AdminAuth, endpoints.GetEvent)
	router.GET("/event/builds", endpoints.AdminAuth, endpoints.GetEventBuilds)
	router.GET("/event/settings", endpoints.AdminAuth, endpoints.GetEventSettings)
	router.PUT("/event/settings", endpoints.AdminAuth, endpoints.UpdateEventSettings)
	router.POST("/event", endpoints.AddEvent)
	router.DELETE("/event/build", endpoints.AdminAuth, endpoints.DeleteEventsByBuild)
	router.GET("/crash", endpoints.AdminAuth, endpoints.GetCrash)
	router.GET("/crash/accessviolations", endpoints.AdminAuth, endpoints.GetAccessViolationCrashes)
	router.GET("/crash/health", endpoints.AdminAuth, endpoints.GetCrashHealth)
	router.GET("/crash/builds", endpoints.AdminAuth, endpoints.GetCrashBuilds)
	router.GET("/crash/settings", endpoints.AdminAuth, endpoints.GetCrashSettings)
	router.PUT("/crash/settings", endpoints.AdminAuth, endpoints.UpdateCrashSettings)
	router.POST("/resolvecrash", endpoints.AdminAuth, endpoints.ResolveCrash)
	router.POST("/crash/ignore", endpoints.AdminAuth, endpoints.IgnoreCrash)
	router.POST("/crash", endpoints.AddCrash)
	router.GET("/savegame", endpoints.AdminAuth, endpoints.GetSavegames)
	router.GET("/savegame/download-all", endpoints.AdminAuth, endpoints.DownloadAllSavegames)
	router.GET("/savegame/builds", endpoints.AdminAuth, endpoints.GetSavegameBuilds)
	router.GET("/system/stats", endpoints.AdminAuth, endpoints.GetSystemStats)
	router.GET("/packrat/version", endpoints.GetPackratVersion)
	router.GET("/packrat/version/settings", endpoints.AdminAuth, endpoints.GetPackratVersionSettings)
	router.PUT("/packrat/version/settings", endpoints.AdminAuth, endpoints.UpdatePackratVersionSettings)
	router.POST("/packrat/rebuild", endpoints.PortalPrivilegedBridge, endpoints.RebuildPackratDocs)
	router.POST("/savegame", endpoints.AddSavegame)
	router.GET("/savegame/download", endpoints.AdminAuth, endpoints.DownloadSavegame)
	router.DELETE("/savegame", endpoints.AdminAuth, endpoints.DeleteSavegame)
	router.DELETE("/savegame/build", endpoints.AdminAuth, endpoints.DeleteSavegamesByBuild)
	router.GET("/user/:id", endpoints.AdminAuth, endpoints.GetUserHistory)
	router.POST("/user/:id/block", endpoints.AdminAuth, endpoints.BlockUser)
	router.PUT("/user/:id/feedback-block", endpoints.AdminAuth, endpoints.BlockFeedbackUser)
	router.DELETE("/user/:id/feedback-block", endpoints.AdminAuth, endpoints.UnblockFeedbackUser)
	router.GET("/users/feedback-blocked", endpoints.AdminAuth, endpoints.GetFeedbackBlockedUsers)
	router.GET("/users/blocked", endpoints.AdminAuth, endpoints.GetBlockedUsers)
	router.DELETE("/users/blocked/:id", endpoints.AdminAuth, endpoints.UnblockUser)
	router.GET("/projects/discord-sources", endpoints.AdminAuth, endpoints.GetDiscordSourceCatalog)
	router.GET("/projects", endpoints.AdminAuth, endpoints.GetProjects)
	router.PUT("/projects/:id", endpoints.AdminAuth, endpoints.UpdateProject)

	router.POST("/words/analyze", endpoints.AnalyzeWords)
	router.POST("/words/unknown", endpoints.GetUnknownWords)
	router.POST("/words/update", endpoints.UpdateWords)
	router.POST("/words/stats", endpoints.GetAllWordStats)

}
