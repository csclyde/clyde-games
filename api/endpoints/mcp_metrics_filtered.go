package endpoints

import (
	"api.clyde.games/models"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var runFilteredMetric = models.RunFilteredMetric

func mcpFilteredMetric(c *gin.Context) {
	id, e := strconv.Atoi(c.Param("id"))
	if e != nil || !models.GameplayMetricAllowed(id) {
		c.JSON(404, gin.H{"error": "unknown gameplay aggregate; use dedicated crash/feedback tools for report cards"})
		return
	}
	allowed := map[string]bool{"project": true, "created_from": true, "created_to": true, "build": true, "environment": true, "event_type": true, "level": true}
	for key, values := range c.Request.URL.Query() {
		if !allowed[key] || len(values) != 1 {
			c.JSON(400, gin.H{"error": "unknown or repeated metric filter"})
			return
		}
	}
	f, e := (models.MetricFilters{Project: c.Query("project"), CreatedFrom: c.Query("created_from"), CreatedTo: c.Query("created_to"), Build: c.Query("build"), Environment: c.Query("environment"), EventType: c.Query("event_type"), Level: c.Query("level")}).Normalize()
	if e != nil {
		c.JSON(400, gin.H{"error": e.Error()})
		return
	}
	raw, _ := json.Marshal(f)
	key := "filtered:" + strconv.Itoa(id) + ":" + string(raw)
	now := time.Now()
	metricCache.Lock()
	entry := metricCache.entries[key]
	if entry != nil && !entry.expires.After(now) {
		delete(metricCache.entries, key)
		entry = nil
	}
	if entry == nil {
		if len(metricCache.entries) >= 128 {
			for k, v := range metricCache.entries {
				if !v.expires.After(now) {
					delete(metricCache.entries, k)
				}
			}
		}
		if len(metricCache.entries) >= 128 {
			metricCache.Unlock()
			c.JSON(503, gin.H{"error": "metrics are busy; retry shortly"})
			return
		}
		entry = &metricCacheEntry{ready: make(chan struct{}), expires: now.Add(5 * time.Minute)}
		metricCache.entries[key] = entry
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			select {
			case metricSlots <- struct{}{}:
				entry.result, entry.err = runFilteredMetric(ctx, id, f)
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
		c.JSON(504, gin.H{"error": "metric request canceled or timed out"})
		return
	case <-entry.ready:
	}
	if entry.err != nil {
		c.JSON(500, gin.H{"error": "unable to load filtered metric"})
		return
	}
	response := struct {
		*models.MetricResult
		MetricID int                  `json:"metric_id"`
		Filters  models.MetricFilters `json:"filters"`
		Complete bool                 `json:"complete"`
		Cohort   string               `json:"cohort_note"`
	}{entry.result, id, f, !entry.result.Truncated, "Retained non-deleted source events matching all filters, including metric baselines. Date windows can contain partial sessions/runs. Player labels in legacy formulas refer to client PIDs, not verified people or active-installation totals."}
	data, e := json.Marshal(response)
	if e != nil || len(data) > 1<<20 {
		c.JSON(413, gin.H{"error": "metric result exceeds response budget; narrow the filters", "complete": false})
		return
	}
	c.Data(200, "application/json; charset=utf-8", data)
}
func mcpCrashHealth(c *gin.Context) {
	for key, values := range c.Request.URL.Query() {
		if (key != "project" && key != "days") || len(values) != 1 {
			c.JSON(400, gin.H{"error": "unknown or repeated health filter"})
			return
		}
	}
	project := strings.TrimSpace(c.Query("project"))
	if project == "" || len(project) > 64 || strings.IndexFunc(project, unicode.IsControl) >= 0 {
		c.JSON(400, gin.H{"error": "choose a project or all"})
		return
	}
	days, e := strconv.Atoi(c.DefaultQuery("days", "30"))
	if e != nil || (days != 7 && days != 30 && days != 90) {
		c.JSON(400, gin.H{"error": "days must be 7, 30 or 90"})
		return
	}
	h, e := models.SelectCrashHealth(mcpDB(c), project, days, time.Now().UTC())
	if e != nil {
		code := 500
		if errors.Is(e, models.ErrCrashHealthLimit) {
			code = 413
		}
		c.JSON(code, gin.H{"error": "complete crash health unavailable for this scope", "complete": false})
		return
	}
	// AI analysis needs aggregate evidence, not a copy of closed issue metadata.
	h.Groups = nil
	h.Families = nil
	raw, e := json.Marshal(h)
	if e != nil || len(raw) > 1<<20 {
		c.JSON(413, gin.H{"error": "health result exceeds response budget; choose one project", "complete": false})
		return
	}
	c.Data(200, "application/json; charset=utf-8", raw)
}
