package main

// import "net/http"
import (
	"api.clyde.games/endpoints"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func main() {
	router := gin.Default()
	router.Use(CORSMiddleware())
	router.GET("/feedback", endpoints.GetFeedback)
	router.POST("/feedback", endpoints.AddFeedback)
	router.GET("/event", endpoints.GetEvent)
	router.POST("/event", endpoints.AddEvent)

	router.POST("/words/analyze", endpoints.AnalyzeWords)

	router.Run(":9990")
}
