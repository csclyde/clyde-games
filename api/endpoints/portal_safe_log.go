package endpoints

import (
	"github.com/gin-gonic/gin"
	"log"
	"time"
)

// Credentials belong in neither normal request logs nor panic request dumps.
// Route templates exclude usernames/IDs, all query strings, cookies and bodies.
func SafePortalRequestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("api method=%s route=%s status=%d duration=%s", c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(start))
	}
}
func SafePortalRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		panicked := true
		defer func() {
			if panicked {
				_ = recover()
				log.Printf("api panic route=%s", c.FullPath())
				c.AbortWithStatusJSON(500, gin.H{"error": "request failed"})
			}
		}()
		c.Next()
		panicked = false
	}
}
