package endpoints

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const mcpListLimit = 200

// RegisterMCPRoutes is deliberately separate from the game's unauthenticated
// ingestion endpoints. Never expose arbitrary API paths through this surface.
func RegisterMCPRoutes(router *gin.Engine) {
	bot := router.Group("/internal/discord", DiscordBotAuth)
	bot.POST("/manual", mcpDiscordManual)
	bot.GET("/sources", botSyncSources)
	bot.POST("/messages/:id/mark-result", botMarkResult)
	bot.GET("/messages/:id/status", mcpDiscordIntakeStatus)
	g := router.Group("/internal/mcp", MCPAuth)
	registerMarketingHandlers(g.Group("/marketing/:project", func(c *gin.Context) {
		c.Set("marketing_mcp", true)
		c.Next()
	}, marketingScope))
	registerReportNoteRoutes(g)
	registerCrashWorkRoutes(g)
	g.POST("/crash-investigation/:id/claim", claimCrashInvestigation)
	g.GET("/projects", mcpProjects)
	g.GET("/feedback-ingestion", mcpFeedbackIngestionStatus)
	g.POST("/feedback/:id/triage", mcpTriageFeedback)
	registerMCPDiscord(g)
	registerMCPPlanka(g)
	g.GET("/feedback", mcpFeedbackList)
	g.GET("/feedback/:id", mcpFeedbackDetail)
	g.GET("/feedback/:id/save", mcpFeedbackSave)
	g.POST("/feedback/:id/resolve", mcpResolveFeedback)
	g.GET("/crashes", mcpCrashList)
	g.GET("/crashes/:id", mcpCrashDetail)
	g.POST("/crashes/:id/resolve", mcpResolveCrash)
	g.GET("/metric-query/:id", mcpFilteredMetric)
	g.GET("/crash-health", mcpCrashHealth)
	g.GET("/metrics", explicitMCPMetricProject, GetMetrics)
	g.GET("/metrics/:id", explicitMCPMetricProject, GetMetric)
}

func explicitMCPMetricProject(c *gin.Context) {
	if len(c.Request.URL.Query()["project"]) != 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "choose cursemark, necrovale, or farm-builder explicitly"})
		return
	}
	if _, err := (models.MetricFilters{Project: c.Query("project")}).Normalize(); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func MCPAuth(c *gin.Context) {
	token := os.Getenv("MCP_UPSTREAM_TOKEN")
	supplied := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if len(token) < 32 || !strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") || subtle.ConstantTimeCompare([]byte(token), []byte(supplied)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Header("Cache-Control", "no-store")
	timeout := 15 * time.Second
	if c.FullPath() == "/internal/mcp/metric-query/:id" || c.FullPath() == "/internal/mcp/metrics/:id" || c.FullPath() == "/internal/mcp/discord/messages/:id/intake" {
		timeout = 55 * time.Second // Existing metric queries have a 45-second execution budget.
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

type mcpReport struct {
	TriageState       string     `json:"triage_state,omitempty"`
	TriageNote        string     `json:"triage_note,omitempty"`
	TriageAt          *time.Time `json:"triage_at,omitempty"`
	Source            string     `json:"source,omitempty"`
	SourceID          string     `json:"source_id,omitempty"`
	SourceURL         string     `json:"source_url,omitempty"`
	ID                uint       `json:"id"`
	Project           string     `json:"project"`
	Message           string     `json:"message"`
	Translated        string     `json:"translated,omitempty"`
	Stack             string     `json:"stack,omitempty"`
	Rating            uint8      `json:"rating,omitempty"`
	Category          string     `json:"category,omitempty"`
	Platform          string     `json:"platform,omitempty"`
	Build             string     `json:"build,omitempty"`
	Commit            string     `json:"commit,omitempty"`
	Count             uint       `json:"count,omitempty"`
	Resolved          bool       `json:"resolved"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	TextTruncated     bool       `json:"text_truncated" gorm:"-"`
	GroupKey          string     `json:"group_key,omitempty"`
	GroupIdentity     string     `json:"group_identity,omitempty"`
	OccurrenceVersion uint       `json:"occurrence_version"`
	IngestedAt        *time.Time `json:"ingested_at,omitempty"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
}

func mcpBound(s string, n int) (string, bool) {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]), true
	}
	return s, false
}
func mcpBoundReport(r *mcpReport, detail bool) {
	limit := 512
	if detail {
		limit = 16384
	}
	var truncated bool
	for _, p := range []*string{&r.Message, &r.Translated, &r.Stack, &r.GroupIdentity, &r.TriageNote} {
		*p, truncated = mcpBound(*p, limit)
		r.TextTruncated = r.TextTruncated || truncated
	}
	for _, p := range []*string{&r.Project, &r.Category, &r.Platform, &r.Build, &r.Commit} {
		*p, truncated = mcpBound(*p, 255)
		r.TextTruncated = r.TextTruncated || truncated
	}
	// Raw player ID and Env fields are intentionally omitted. Free-form text remains untrusted.
}
func mcpDB(c *gin.Context) *gorm.DB { return models.AnalyticsDB.WithContext(c.Request.Context()) }
func mcpDBError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"error": "report not found"})
		return
	}
	c.JSON(500, gin.H{"error": "database operation failed"})
}
func mcpID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"error": "id must be a positive integer"})
		return 0, false
	}
	return id, true
}
func mcpProjects(c *gin.Context) {
	items := []models.Project{}
	if err := mcpDB(c).Order("id ASC").Limit(mcpListLimit + 1).Find(&items).Error; err != nil {
		mcpDBError(c, err)
		return
	}
	truncated := len(items) > mcpListLimit
	if truncated {
		items = items[:mcpListLimit]
	}
	c.JSON(200, gin.H{"items": items, "truncated": truncated, "limit": mcpListLimit})
}
func mcpQuery(c *gin.Context, crash bool) *gorm.DB {
	fields := []string{"id", "project", "message", "category", "platform", "build", "commit", "resolved", "created_at", "updated_at"}
	table := "feedbacks"
	if crash {
		table = "crashes"
		fields = append(fields, "stack", "count", "group_key", "group_identity", "occurrence_version", "ingested_at", "resolved_at")
	} else {
		fields = append(fields, "translated", "rating", "source", "source_id", "source_url", "ingested_at", "triage_state", "triage_note", "triage_at")
	}
	return mcpDB(c).Table(table).Select(fields).Where("deleted_at IS NULL")
}

func mcpReportProject(c *gin.Context) (*string, bool) {
	if _, present := c.Request.URL.Query()["project"]; !present {
		return nil, true
	}
	project, ok := noteProject(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "choose one canonical project"})
		return nil, false
	}
	if project == "all" {
		return nil, true
	}
	return &project, true
}

func scopeMCPReports(query *gorm.DB, project *string) *gorm.DB {
	if project == nil {
		return query
	}
	return query.Where("(BINARY COALESCE(TRIM(project),'') = ? OR (? = 'cursemark' AND LOWER(TRIM(project)) = 'cursemark'))", *project, *project)
}

func mcpList(c *gin.Context, crash bool) {
	// Return a complete compact work index even when only the first 200 reports
	// fit as summaries. The caller can retrieve any later report directly by ID.
	project, ok := mcpReportProject(c)
	if !ok {
		return
	}
	const maxActiveIDs = 100000
	ids := []uint{}
	queue := scopeMCPReports(mcpQuery(c, crash), project)
	if !crash {
		queue = queue.Where("parent_feedback_id IS NULL")
	}
	if err := queue.Select("id").Where("resolved = ?", false).Order("created_at ASC, id ASC").Limit(maxActiveIDs+1).Pluck("id", &ids).Error; err != nil {
		mcpDBError(c, err)
		return
	}
	if len(ids) > maxActiveIDs {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Active queue exceeds the complete-index safety limit; use the admin portal to review it. No partial index returned.", "limit": maxActiveIDs, "index_complete": false})
		return
	}
	items := []mcpReport{}
	summaryIDs := ids
	truncated := len(ids) > mcpListLimit
	if truncated {
		summaryIDs = ids[:mcpListLimit]
	}
	if len(summaryIDs) > 0 {
		if err := scopeMCPReports(mcpQuery(c, crash), project).Where("id IN ?", summaryIDs).Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
			mcpDBError(c, err)
			return
		}
	}
	for i := range items {
		mcpBoundReport(&items[i], false)
		items[i].Stack = ""
		items[i].GroupIdentity = ""
	}
	c.JSON(200, gin.H{"items": items, "active_ids": ids, "active_count": len(ids), "index_complete": true, "truncated": truncated, "limit": mcpListLimit})
}

type mcpSaveMetadata struct {
	ID          uint   `json:"id"`
	Filename    string `json:"filename"`
	Size        uint32 `json:"size"`
	Hash        string `json:"hash"`
	ContentType string `json:"content_type"`
}

func mcpDetail(c *gin.Context, crash bool) {
	id, ok := mcpID(c)
	if !ok {
		return
	}
	project, ok := mcpReportProject(c)
	if !ok {
		return
	}
	var item mcpReport
	if err := scopeMCPReports(mcpQuery(c, crash), project).Where("id = ?", id).Take(&item).Error; err != nil {
		mcpDBError(c, err)
		return
	}
	mcpBoundReport(&item, true)
	response := gin.H{"item": item}
	if crash {
		// Return a bounded, complete membership index; inspect any report ID
		// independently. Never embed an unbounded collection of stacks.
		ids := []uint{}
		if item.GroupKey != "" {
			if err := mcpDB(c).Model(&models.Crash{}).Where("group_key = ?", item.GroupKey).Order("id ASC").Limit(100001).Pluck("id", &ids).Error; err != nil {
				mcpDBError(c, err)
				return
			}
			if len(ids) > 100000 {
				c.JSON(413, gin.H{"error": "crash group exceeds membership index limit", "index_complete": false})
				return
			}
		}
		response["group_report_ids"] = ids
		response["group_index_complete"] = true
	}
	if !crash && (item.Source == "steam_discussions" || item.Category == "Steam Discussion" || item.Category == "Steam Announcement Discussion") {
		var replies []mcpReport
		const limit = 50
		if err := mcpQuery(c, false).Where("parent_feedback_id = ?", id).Order("created_at DESC, id DESC").Limit(limit + 1).Find(&replies).Error; err != nil {
			mcpDBError(c, err)
			return
		}
		response["replies_truncated"] = len(replies) > limit
		if len(replies) > limit {
			replies = replies[:limit]
		}
		for i, j := 0, len(replies)-1; i < j; i, j = i+1, j-1 {
			replies[i], replies[j] = replies[j], replies[i]
		}
		for i := range replies {
			mcpBoundReport(&replies[i], false)
		}
		response["replies"] = replies
	}
	if !crash {
		var save mcpSaveMetadata
		err := mcpDB(c).Table("savegames").Select([]string{"id", "filename", "size", "hash", "content_type"}).Where("feedback_id = ? AND deleted_at IS NULL", id).Order("created_at DESC, id DESC").Take(&save).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			mcpDBError(c, err)
			return
		}
		if err == nil {
			save.Filename, _ = mcpBound(save.Filename, 255)
			save.Hash, _ = mcpBound(save.Hash, 64)
			save.ContentType, _ = mcpBound(save.ContentType, 128)
			response["save"] = save
		}
	}
	c.JSON(200, response)
}
func mcpFeedbackList(c *gin.Context)   { mcpList(c, false) }
func mcpCrashList(c *gin.Context)      { listCrashInvestigations(c) }
func mcpFeedbackDetail(c *gin.Context) { mcpDetail(c, false) }
func mcpCrashDetail(c *gin.Context)    { mcpDetail(c, true) }
func mcpResolveFeedback(c *gin.Context) {
	id, ok := mcpID(c)
	if !ok {
		return
	}
	err := mcpDB(c).Transaction(func(tx *gorm.DB) error {
		var item models.Feedback
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&item).Update("resolved", true).Error; err != nil {
			return err
		}
		return tx.Where("feedback_id = ?", id).Delete(&models.Savegame{}).Error
	})
	if err != nil {
		mcpDBError(c, err)
		return
	}
	c.JSON(200, gin.H{"id": id, "resolved": true, "savegames_soft_deleted": true})
}
func mcpResolveCrash(c *gin.Context) {
	id, ok := mcpID(c)
	if !ok {
		return
	}
	item, err := models.ResolveCrashID(mcpDB(c), id)
	if err != nil {
		mcpDBError(c, err)
		return
	}
	c.JSON(200, gin.H{"id": id, "group_key": item.GroupKey, "resolved": true})
}
