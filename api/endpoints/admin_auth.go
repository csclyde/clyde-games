package endpoints

import (
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// AdminAuth protects legacy portal routes independently of the Basic Auth HTML
// shell. Nginx injects this dedicated secret only after authenticating the user.
// Never put it into frontend JavaScript or game clients.
func AdminAuth(c *gin.Context) {
	if PortalAuthMode() == "invalid" {
		c.AbortWithStatusJSON(503, gin.H{"error": "authentication configuration unavailable"})
		return
	}
	if PortalAuthMode() == "session" && isPortalBrowser(c) {
		if !portalSessionAuth(c, false) {
			return
		}
		u, _ := portalAccount(c)
		if u.Role != "admin" && c.Request.Method != "GET" && c.Request.Method != "HEAD" {
			c.AbortWithStatusJSON(403, gin.H{"error": "read-only account"})
			return
		}
		c.Next()
		return
	}

	token := os.Getenv("ADMIN_API_TOKEN")
	if len(token) < 32 || subtle.ConstantTimeCompare([]byte(token), []byte(c.GetHeader("X-Admin-API-Token"))) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	// Browser credentials are ambient. Even a token injected by the proxy must
	// not authorize cross-origin requests. Non-browser clients with the explicit
	// server secret may omit Origin; the proxy also enforces this boundary.
	origin := c.GetHeader("Origin")
	if (origin != "" && origin != "https://admin.clyde.games") || c.GetHeader("Sec-Fetch-Site") == "cross-site" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-origin admin request denied"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Next()
}
