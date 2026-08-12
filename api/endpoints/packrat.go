package endpoints

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

const packratDeployScript = "/root/Packrat/scripts/deploy-docs.sh"

var packratDeployMu sync.Mutex

// RebuildPackratDocs pulls the Packrat repository and publishes its VitePress
// site through the repository's deployment script.
func RebuildPackratDocs(c *gin.Context) {
	configuredToken := os.Getenv("PACKRAT_DEPLOY_TOKEN")
	providedToken := c.GetHeader("X-Packrat-Deploy-Token")
	if configuredToken == "" || subtle.ConstantTimeCompare([]byte(providedToken), []byte(configuredToken)) != 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if !packratDeployMu.TryLock() {
		c.JSON(http.StatusConflict, gin.H{"error": "a Packrat documentation deployment is already running"})
		return
	}
	defer packratDeployMu.Unlock()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Minute)
	defer cancel()

	command := exec.CommandContext(ctx, packratDeployScript)
	command.Dir = "/root/Packrat"
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Packrat documentation deployment timed out", "output": trimCommandOutput(output)})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Packrat documentation deployment failed", "output": trimCommandOutput(output)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Packrat documentation deployed", "output": trimCommandOutput(output)})
}

func GetPackratVersion(c *gin.Context) {
	check, err := models.CheckPackratVersion(c.Query("current"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, check)
}

func GetPackratVersionSettings(c *gin.Context) {
	settings, err := models.GetPackratVersionSettings()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

func UpdatePackratVersionSettings(c *gin.Context) {
	var settings models.PackratVersionSettings
	if err := c.BindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := models.SavePackratLatestVersion(settings.LatestVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func trimCommandOutput(output []byte) string {
	const maxOutputLength = 12000
	text := strings.TrimSpace(string(output))
	if len(text) <= maxOutputLength {
		return text
	}
	return text[len(text)-maxOutputLength:]
}
