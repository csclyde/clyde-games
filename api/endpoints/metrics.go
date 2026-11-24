package endpoints

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

// Nginx authenticates the admin and injects the token. Direct public API calls
// cannot obtain analytics by bypassing the portal's HTTP basic authentication.
func MetricsAuth(c *gin.Context) {
	if PortalAuthMode() == "invalid" {
		c.AbortWithStatusJSON(503, gin.H{"error": "authentication configuration unavailable"})
		return
	}
	if PortalAuthMode() == "session" && isPortalBrowser(c) {
		AdminAuth(c)
		return
	}

	token := os.Getenv("PACKRAT_DEPLOY_TOKEN")
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(c.GetHeader("X-Packrat-Deploy-Token"))) != 1 {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Next()
}

func GetMetrics(c *gin.Context) {
	project := strings.ToLower(strings.TrimSpace(c.Query("project")))
	if project == "" { project = "cursemark" }
	cards := make([]models.MetricDefinition, 0)
	for _, card := range models.MetricCatalog() {
		if card.Project == project || (card.Project == "" && project == "cursemark") { cards = append(cards, card) }
	}
	c.JSON(http.StatusOK, cards)
}

type metricCacheEntry struct {
	ready   chan struct{}
	result  *models.MetricResult
	err     error
	expires time.Time
}

var metricCache = struct {
	sync.Mutex
	entries map[string]*metricCacheEntry
}{entries: make(map[string]*metricCacheEntry)}
var metricSlots = make(chan struct{}, 2)

func GetMetric(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	var definition *models.MetricDefinition
	for _, card := range models.MetricCatalog() {
		if card.ID == id {
			definition = &card
			break
		}
	}
	if err != nil || definition == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown metric"})
		return
	}
	project := strings.ToLower(strings.TrimSpace(c.Query("project")))
	if project == "" {
		project = "cursemark"
	}
	if (definition.Project == "" && project != "cursemark") || (definition.Project != "" && project != definition.Project) {
		c.JSON(http.StatusNotFound, gin.H{"error": "No gameplay metrics are configured for this project"})
		return
	}
	if len(project) > 128 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project"})
		return
	}
	key := strconv.Itoa(id) + ":" + project
	now := time.Now()
	metricCache.Lock()
	entry := metricCache.entries[key]
	if entry != nil && !entry.expires.After(now) {
		delete(metricCache.entries, key)
		entry = nil
	}
	if entry == nil {
		// Bound cache size even if many distinct project values are requested.
		if len(metricCache.entries) >= 128 {
			for k, item := range metricCache.entries {
				if !item.expires.After(now) {
					delete(metricCache.entries, k)
				}
			}
		}
		if len(metricCache.entries) >= 128 {
			metricCache.Unlock()
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Metrics are busy. Please retry shortly."})
			return
		}
		entry = &metricCacheEntry{ready: make(chan struct{}), expires: now.Add(5 * time.Minute)}
		metricCache.entries[key] = entry
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			select {
			case metricSlots <- struct{}{}:
				entry.result, entry.err = models.RunMetric(ctx, id, project)
				<-metricSlots
			case <-ctx.Done():
				entry.err = ctx.Err()
			}
			metricCache.Lock()
			if entry.err != nil {
				delete(metricCache.entries, key)
			}
			close(entry.ready)
			metricCache.Unlock()
		}()
	}
	metricCache.Unlock()
	select {
	case <-c.Request.Context().Done():
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Metric request canceled or timed out. Please retry."})
		return
	case <-entry.ready:
	}
	if entry.err != nil {
		log.Printf("metric %d failed: %v", id, entry.err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load this metric. Please retry."})
		return
	}
	c.JSON(http.StatusOK, entry.result)
}
