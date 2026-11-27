package endpoints

import (
	"api.clyde.games/models"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Legacy constants exist only for migration/test evidence; runtime reads configuration.
const discordFeedbackGuild = models.LegacyDiscordGuild
const discordFeedbackChannel = models.LegacyDiscordText

var discordCollectionLock sync.Mutex

type discordSnapshot struct {
	ID               string                   `json:"id"`
	GuildID          string                   `json:"guild_id"`
	ChannelID        string                   `json:"channel_id"`
	ParentID         string                   `json:"parent_id"`
	ScopeID          string                   `json:"scope_id"`
	Kind             string                   `json:"kind"`
	Timestamp        time.Time                `json:"timestamp"`
	ContextTimestamp time.Time                `json:"context_timestamp"`
	Handled          bool                     `json:"handled"`
	ContextOnly      bool                     `json:"context_only"`
	Historical       bool                     `json:"historical_identity_only"`
	Payload          models.Feedback          `json:"feedback"`
	Attachments      []discordImageAttachment `json:"attachments"`
	ThreadContext    []discordImageContext    `json:"thread_context"`
	Trigger          discordImageContext      `json:"trigger_message"`
}

func discordBotCall(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://127.0.0.1:9992"+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	token := os.Getenv("DISCORD_INGEST_TOKEN")
	if len(token) < 32 {
		return fmt.Errorf("Discord bridge token unavailable")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 120 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Little Clyde bridge unavailable")
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20+1))
	if err != nil || len(data) > 8<<20 {
		return fmt.Errorf("Little Clyde response exceeded limit")
	}
	if out != nil && json.Unmarshal(data, out) != nil {
		return fmt.Errorf("invalid Little Clyde response")
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("Little Clyde returned HTTP %d", res.StatusCode)
	}
	return nil
}

func validSnowflake(id string) bool {
	if len(id) < 1 || len(id) > 22 {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func StartDiscordFeedbackImporter() {
	startFeedbackSchedule("discord", func() {
		discordCollectionLock.Lock()
		defer discordCollectionLock.Unlock()
		sources, e := models.SelectDiscordSyncSources("", true)
		if e != nil {
			fmt.Printf("Discord source lookup failed: %v\n", e)
			return
		}
		for _, source := range sources {
			state := models.SyncState{Name: models.DiscordSyncName(source), LastSuccessAt: source.BaselineAt, BaselineAt: &source.BaselineAt, Status: "baselined"}
			if e := models.AnalyticsDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; e != nil {
				continue
			}
			_, _, _, _, e := runFeedbackSource(models.Project{ID: source.ProjectID}, "discord_source_"+source.ID, "", func(from, to time.Time) (int, int, error) { return collectConfiguredDiscord(source, from, to) })
			if e != nil {
				fmt.Printf("Discord source %s failed: %v\n", source.ID, e)
			}
		}
	})
}
func collectConfiguredDiscord(source models.DiscordSyncSource, from, to time.Time) (int, int, error) {
	var batch struct {
		Messages []json.RawMessage `json:"messages"`
		Complete bool              `json:"complete"`
		SourceID string            `json:"source_id"`
	}
	path := "/batch?source_id=" + source.ID + "&from=" + from.Format(time.RFC3339Nano) + "&to=" + to.Format(time.RFC3339Nano)
	if e := discordBotCall(context.Background(), "GET", path, nil, &batch); e != nil {
		return 0, 0, e
	}
	if !batch.Complete || batch.SourceID != source.ID {
		return 0, 0, fmt.Errorf("incomplete or mismatched Discord batch")
	}
	added, skipped := 0, 0
	for _, raw := range batch.Messages {
		changed, e := storeDiscordSnapshot(source, raw, false)
		if e != nil {
			return added, skipped, e
		}
		if changed {
			added++
		} else {
			skipped++
		}
	}
	return added, skipped, nil
}
func sourceForSnapshot(s discordSnapshot) (*models.DiscordSyncSource, error) {
	id := s.ChannelID
	if s.Kind == "forum_thread" {
		id = s.ParentID
	}
	var source models.DiscordSyncSource
	e := models.AnalyticsDB.Where("id = ? AND enabled = ?", id, true).Take(&source).Error
	if e != nil {
		return nil, e
	}
	if s.GuildID != source.GuildID || !validSnowflake(s.ID) || !validSnowflake(s.ChannelID) || s.Timestamp.IsZero() {
		return nil, fmt.Errorf("invalid Discord source identity")
	}
	if source.Kind == "discord_forum" {
		if s.Kind != "forum_thread" || s.ID != s.ChannelID || s.ParentID != source.ID {
			return nil, fmt.Errorf("forum starter identity mismatch")
		}
	} else if s.ChannelID != source.ID || s.Kind == "forum_thread" {
		return nil, fmt.Errorf("text source identity mismatch")
	}
	return &source, nil
}
func discordPending(c *gin.Context) (*models.DiscordPendingMessage, bool) {
	if !validSnowflake(c.Param("id")) {
		c.JSON(400, gin.H{"error": "invalid message id"})
		return nil, false
	}
	var item models.DiscordPendingMessage
	if e := mcpDB(c).Where("id = ?", c.Param("id")).Take(&item).Error; e != nil {
		mcpDBError(c, e)
		return nil, false
	}
	if item.ManualTriggerID != "" {
		var snapshot discordSnapshot
		if json.Unmarshal([]byte(item.Snapshot), &snapshot) != nil {
			c.JSON(500, gin.H{"error": "invalid stored manual evidence"})
			return nil, false
		}
		source, e := manualSourceForSnapshot(snapshot)
		if e != nil || source.ProjectID != item.Project {
			c.JSON(409, gin.H{"error": "manual association unavailable"})
			return nil, false
		}
		return &item, true
	}
	var source models.DiscordSyncSource
	if e := mcpDB(c).Where("id = ? AND project_id = ? AND enabled = ?", item.ScopeID, item.Project, true).Take(&source).Error; e != nil {
		mcpDBError(c, e)
		return nil, false
	}
	return &item, true
}
func discordIntake(s discordSnapshot, manual ...bool) (*models.Feedback, error) {
	resolver := sourceForSnapshot
	if len(manual) > 0 && manual[0] {
		resolver = manualSourceForSnapshot
	}
	source, e := resolver(s)
	if e != nil {
		return nil, e
	}
	url := "https://discord.com/channels/" + s.GuildID + "/" + s.ChannelID + "/" + s.ID
	fb := s.Payload
	fb.ID = 0
	fb.Resolved = false
	fb.ClosureDisposition = ""
	fb.CompletionReactionDue, fb.CompletionReactionSentAt = nil, nil
	fb.DeletedAt = gorm.DeletedAt{}
	fb.Translated = ""
	fb.SourceKey = nil
	fb.Project = source.ProjectID
	fb.Platform = "Discord"
	fb.Category = "Discord"
	fb.Env = "production"
	fb.CreatedAt = s.Timestamp
	fb.UpdatedAt = time.Time{}
	fb.TriageState = ""
	fb.TriageNote = ""
	fb.TriageAt = nil
	if fb.PID == "" || fb.Message == "" {
		return nil, fmt.Errorf("source evidence missing")
	}
	external := s.GuildID + ":" + s.ChannelID + ":" + s.ID
	if _, e := models.AddSourceFeedback(fb, "discord", external, url); e != nil {
		return nil, e
	}
	var saved models.Feedback
	e = models.AnalyticsDB.Unscoped().Where("project = ? AND source = ? AND source_id = ?", source.ProjectID, "discord", external).Take(&saved).Error
	return &saved, e
}
func mcpDiscordManual(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
	var raw json.RawMessage
	var s discordSnapshot
	if c.ShouldBindJSON(&raw) != nil || json.Unmarshal(raw, &s) != nil {
		c.JSON(400, gin.H{"error": "invalid snapshot"})
		return
	}
	source, e := manualSourceForSnapshot(s)
	if errors.Is(e, errDiscordManualProjectRequired) {
		c.JSON(409, gin.H{"error": e.Error()})
		return
	}
	if e != nil {
		mcpDBError(c, e)
		return
	}
	if _, e = storeDiscordSnapshot(*source, raw, true); e != nil {
		mcpDBError(c, e)
		return
	}
	saved, e := discordIntake(s, true)
	if e == nil && !saved.HasImage && !saved.Resolved && !saved.DeletedAt.Valid {
		if data := fetchDiscordScreenshot(c.Request.Context(), s); len(data) > 0 {
			// Text is already durable. A failed image import must not fail the flag.
			_ = models.AttachDiscordFeedbackImage(saved.ID, saved.Project, data)
		}
	}
	if e != nil {
		mcpDBError(c, e)
		return
	}
	if e := mcpDB(c).Model(&models.DiscordPendingMessage{}).Where("id = ?", s.ID).Updates(map[string]interface{}{"feedback_id": saved.ID, "disposition": "imported", "mark_suppressed": false}).Error; e != nil {
		mcpDBError(c, e)
		return
	}
	c.JSON(200, gin.H{"id": s.ID, "feedback_id": saved.ID, "durable": true})
}
func mcpDiscordBatch(c *gin.Context) {
	ids := []string{}
	q := discordQueueScope(mcpDB(c).Model(&models.DiscordPendingMessage{}))
	if e := q.Where("needs_review = ? OR disposition = ? OR (disposition = ? AND check_marked = ? AND mark_suppressed = ?)", true, "pending", "imported", false, false).Order("source_created_at,id").Limit(10001).Pluck("id", &ids).Error; e != nil {
		mcpDBError(c, e)
		return
	}
	if len(ids) > 10000 {
		c.JSON(413, gin.H{"error": "queue index limit", "index_complete": false})
		return
	}
	items := []models.DiscordPendingMessage{}
	summary := ids
	if len(summary) > 100 {
		summary = summary[:100]
	}
	if len(summary) > 0 {
		if e := mcpDB(c).Where("id IN ?", summary).Order("source_created_at,id").Find(&items).Error; e != nil {
			mcpDBError(c, e)
			return
		}
	}
	snapshots := []json.RawMessage{}
	snapshotIDs := []string{}
	size := 0
	for _, item := range items {
		if size+len(item.Snapshot) > 512<<10 {
			break
		}
		snapshots = append(snapshots, json.RawMessage(item.Snapshot))
		snapshotIDs = append(snapshotIDs, item.ID)
		size += len(item.Snapshot)
	}
	noise := []string{}
	if e := discordQueueScope(mcpDB(c).Model(&models.DiscordPendingMessage{})).Where("disposition = ?", "noise").Limit(10001).Pluck("id", &noise).Error; e != nil {
		mcpDBError(c, e)
		return
	}
	if len(noise) > 10000 {
		c.JSON(413, gin.H{"error": "noise index limit", "index_complete": false})
		return
	}
	sources, e := models.SelectDiscordSyncSources("", true)
	if e != nil {
		mcpDBError(c, e)
		return
	}
	c.JSON(200, gin.H{"sources": sources, "pending_ids": ids, "items": items, "snapshots": snapshots, "snapshot_ids": snapshotIDs, "snapshot_summaries_complete": len(snapshots) == len(ids), "noise_ids": noise, "index_complete": true, "truncated": len(ids) > 100, "limit": 100, "coverage": "Configured enabled sources only. Forum starters are one issue; replies/revisions are context. Handled/context-only items cannot become new issues automatically."})
}
func mcpDiscordDetail(c *gin.Context) {
	item, ok := discordPending(c)
	if !ok {
		return
	}
	revisions := []models.DiscordSnapshotRevision{}
	if e := mcpDB(c).Where("message_id = ?", item.ID).Select("id,message_id,revision,created_at").Order("revision DESC").Limit(101).Find(&revisions).Error; e != nil {
		mcpDBError(c, e)
		return
	}
	c.JSON(200, gin.H{"item": item, "snapshot": json.RawMessage(item.Snapshot), "original_snapshot": json.RawMessage(item.OriginalSnapshot), "revisions": revisions, "revision_index_truncated": len(revisions) > 100, "untrusted_content": true})
}
func mcpDiscordDisposition(c *gin.Context) {
	item, ok := discordPending(c)
	if !ok {
		return
	}
	var r struct {
		Disposition string `json:"disposition"`
		Note        string `json:"note"`
		Revision    uint   `json:"revision"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if c.ShouldBindJSON(&r) != nil || (r.Disposition != "noise" && r.Disposition != "pending" && r.Disposition != "reviewed") || len([]rune(r.Note)) > 2000 {
		c.JSON(400, gin.H{"error": "invalid disposition"})
		return
	}
	conflict := errors.New("context changed or intake completed")
	e := mcpDB(c).Transaction(func(tx *gorm.DB) error {
		var current models.DiscordPendingMessage
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&current, "id = ?", item.ID).Error; e != nil {
			return e
		}
		if r.Revision != current.Revision {
			return conflict
		}
		if r.Disposition == "reviewed" {
			return tx.Model(&current).Updates(map[string]interface{}{"needs_review": false, "reviewed_revision": current.Revision, "note": r.Note}).Error
		}
		if current.Disposition == r.Disposition && current.Note == r.Note && !current.NeedsReview {
			return nil
		}
		if current.Disposition == "imported" || current.Disposition == "handled" {
			return conflict
		}
		return tx.Model(&current).Updates(map[string]interface{}{"disposition": r.Disposition, "note": r.Note, "needs_review": r.Disposition == "pending", "reviewed_revision": current.Revision}).Error
	})
	if errors.Is(e, conflict) {
		c.JSON(409, gin.H{"error": e.Error()})
		return
	}
	if e != nil {
		mcpDBError(c, e)
		return
	}
	c.JSON(200, gin.H{"id": item.ID, "disposition": r.Disposition, "reversible": true})
}
func mcpDiscordIntake(c *gin.Context) {
	item, ok := discordPending(c)
	if !ok {
		return
	}
	if item.Disposition == "handled" && item.ManualTriggerID == "" {
		c.JSON(200, gin.H{"id": item.ID, "handled": true, "skipped": true, "context_only": true, "durable": item.FeedbackID > 0, "feedback_id": item.FeedbackID, "retry_needed": false})
		return
	}
	var gate struct {
		Handled bool `json:"handled"`
	}
	if e := discordBotCall(c.Request.Context(), "GET", "/messages/"+item.ID+"/preflight", nil, &gate); e != nil {
		c.JSON(502, gin.H{"error": e.Error(), "durable": item.FeedbackID > 0, "retry_needed": true})
		return
	}
	if gate.Handled {
		if item.FeedbackID == 0 {
			if e := mcpDB(c).Model(item).Updates(map[string]interface{}{"disposition": "handled", "needs_review": false, "mark_suppressed": true}).Error; e != nil {
				mcpDBError(c, e)
				return
			}
		}
		c.JSON(200, gin.H{"id": item.ID, "handled": true, "skipped": true, "durable": item.FeedbackID > 0, "feedback_id": item.FeedbackID, "retry_needed": false})
		return
	}
	var s discordSnapshot
	if json.Unmarshal([]byte(item.Snapshot), &s) != nil {
		c.JSON(500, gin.H{"error": "invalid stored evidence"})
		return
	}
	saved, e := discordIntake(s, item.ManualTriggerID != "")
	if e != nil {
		mcpDBError(c, e)
		return
	}
	if e := mcpDB(c).Model(item).Updates(map[string]interface{}{"disposition": "imported", "feedback_id": saved.ID, "needs_review": gorm.Expr("CASE WHEN revision = ? THEN false ELSE needs_review END", item.Revision), "reviewed_revision": gorm.Expr("GREATEST(reviewed_revision, ?)", item.Revision)}).Error; e != nil {
		mcpDBError(c, e)
		return
	}
	var marked struct {
		Bug     bool   `json:"bug_marked"`
		Check   bool   `json:"check_marked"`
		Handled bool   `json:"handled"`
		Error   string `json:"error"`
	}
	e = discordBotCall(c.Request.Context(), "POST", "/messages/"+item.ID+"/mark", nil, &marked)
	if e != nil {
		marked.Error = e.Error()
	}
	marked.Error, _ = mcpBound(marked.Error, 1000)
	updates := map[string]interface{}{"last_mark_error": marked.Error}
	if marked.Bug {
		updates["bug_marked"] = true
	}
	if marked.Check {
		updates["check_marked"] = true
	}
	if marked.Handled {
		updates["mark_suppressed"] = true
	}
	if e := mcpDB(c).Model(item).Updates(updates).Error; e != nil {
		mcpDBError(c, e)
		return
	}
	c.JSON(200, gin.H{"id": item.ID, "feedback_id": saved.ID, "durable": true, "bug_marked": item.BugMarked || marked.Bug, "check_marked": item.CheckMarked || marked.Check, "handled": marked.Handled, "mark_error": marked.Error, "retry_needed": !marked.Handled && !(item.CheckMarked || marked.Check)})
}
func mcpDiscordIntakeStatus(c *gin.Context) {
	item, ok := discordPending(c)
	if !ok {
		return
	}
	var count int64
	if item.FeedbackID > 0 {
		if e := mcpDB(c).Unscoped().Model(&models.Feedback{}).Where("id = ? AND project = ? AND source = ? AND source_id = ?", item.FeedbackID, item.Project, "discord", item.GuildID+":"+item.ChannelID+":"+item.ID).Count(&count).Error; e != nil {
			mcpDBError(c, e)
			return
		}
	}
	c.JSON(200, gin.H{"id": item.ID, "durable": count == 1, "feedback_id": item.FeedbackID, "channel_id": item.ChannelID, "guild_id": item.GuildID, "scope_id": item.ScopeID, "kind": item.Kind, "project": item.Project, "manual_trigger_id": item.ManualTriggerID, "revision": item.Revision})
}
func botSyncSources(c *gin.Context) {
	items, e := models.SelectDiscordSyncSources("", true)
	if e != nil {
		mcpDBError(c, e)
		return
	}
	all, e := models.SelectDiscordSyncSources("", false)
	if e != nil {
		mcpDBError(c, e)
		return
	}
	manualSources := []models.DiscordSyncSource{}
	for _, source := range all {
		if source.GuildID == models.LegacyDiscordGuild {
			manualSources = append(manualSources, source)
		}
	}
	defaultProject := ""
	configured := models.NormalizeProjectID(strings.TrimSpace(os.Getenv("DISCORD_MANUAL_DEFAULT_PROJECT")))
	if configured != "" {
		var count int64
		if e := mcpDB(c).Model(&models.Project{}).Where("id = ?", configured).Count(&count).Error; e != nil {
			mcpDBError(c, e)
			return
		}
		if count == 1 {
			defaultProject = configured
		}
	}
	c.JSON(200, gin.H{"items": items, "manual": gin.H{"guild_id": models.LegacyDiscordGuild, "default_project": defaultProject, "sources": manualSources}})
}
func botMarkResult(c *gin.Context) {
	item, ok := discordPending(c)
	if !ok {
		return
	}
	if item.FeedbackID == 0 {
		c.JSON(409, gin.H{"error": "no durable intake"})
		return
	}
	var r struct {
		Trigger string `json:"trigger_message_id"`
		Check   bool   `json:"check_marked"`
		Error   string `json:"error"`
	}
	if c.ShouldBindJSON(&r) != nil {
		c.JSON(400, gin.H{"error": "invalid result"})
		return
	}
	if r.Trigger != "" && r.Trigger != item.ManualTriggerID {
		c.JSON(409, gin.H{"error": "manual trigger changed; retry pending"})
		return
	}
	r.Error, _ = mcpBound(r.Error, 1000)
	updates := map[string]interface{}{"last_mark_error": r.Error}
	if r.Check {
		updates["check_marked"] = true
	}
	query := mcpDB(c).Model(item)
	if r.Trigger != "" {
		query = query.Where("manual_trigger_id = ?", r.Trigger)
	}
	changed := query.Updates(updates)
	if changed.Error != nil {
		e := changed.Error
		mcpDBError(c, e)
		return
	}
	if r.Trigger != "" && changed.RowsAffected == 0 {
		var current models.DiscordPendingMessage
		if e := mcpDB(c).Take(&current, "id = ?", item.ID).Error; e != nil {
			mcpDBError(c, e)
			return
		}
		if current.ManualTriggerID != r.Trigger {
			c.JSON(409, gin.H{"error": "manual trigger changed; retry pending"})
			return
		}
	}
	c.JSON(200, gin.H{"id": item.ID, "recorded": true})
}

// The bot credential grants only intake and verification, never admin MCP tools.
func DiscordBotAuth(c *gin.Context) {
	token := os.Getenv("DISCORD_INGEST_TOKEN")
	supplied := c.GetHeader("Authorization")
	if len(token) < 32 || subtle.ConstantTimeCompare([]byte(supplied), []byte("Bearer "+token)) != 1 {
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
		return
	}
	if c.GetHeader("Origin") != "" {
		c.AbortWithStatus(403)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	c.Header("Cache-Control", "no-store")
	c.Next()
}
