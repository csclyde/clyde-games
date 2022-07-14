package main

// import "net/http"
import "github.com/gin-gonic/gin"

func main() {
	router := gin.Default()
	router.GET("/api/feedback", get_feedback)
	router.POST("/api/feedback", add_feedback)

	router.Run(":9990")
}
