package endpoints

import (
	"api.clyde.games/models"
	"errors"
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"
)

var errDiscordManualProjectRequired = errors.New("manual Discord intake needs an explicitly configured default project for this channel")

// Manual flags and passive monitoring have deliberately separate authorization.
// The narrow bot credential remains required. No new guild, project or monitored
// source is created here; channel content is evidence, never tool authority.
func manualSourceForSnapshot(s discordSnapshot) (*models.DiscordSyncSource, error) {
	if s.GuildID != models.LegacyDiscordGuild || !validSnowflake(s.ID) || !validSnowflake(s.ChannelID) || s.Timestamp.IsZero() {
		return nil, fmt.Errorf("invalid manual Discord guild or identity")
	}
	scope, kind := s.ChannelID, "discord_text"
	switch s.Kind {
	case "discord_text":
	case "forum_thread":
		if s.ID != s.ChannelID || !validSnowflake(s.ParentID) {
			return nil, fmt.Errorf("invalid manual forum identity")
		}
		scope, kind = s.ParentID, "discord_forum"
	default:
		return nil, fmt.Errorf("invalid manual Discord kind")
	}
	if s.ScopeID != "" && s.ScopeID != scope {
		return nil, fmt.Errorf("manual scope identity mismatch")
	}
	if s.Trigger.ID != "" && !validSnowflake(s.Trigger.ID) {
		return nil, fmt.Errorf("invalid manual trigger identity")
	}
	project := ""
	// Preserve the original association across retries/default or monitoring changes.
	var pending models.DiscordPendingMessage
	e := models.AnalyticsDB.Where("id = ?", s.ID).Take(&pending).Error
	if e == nil {
		if pending.GuildID != s.GuildID || pending.ChannelID != s.ChannelID || pending.ScopeID != scope || pending.Kind != s.Kind {
			return nil, fmt.Errorf("manual source identity conflict")
		}
		project = pending.Project
	} else if !errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, e
	}
	if project == "" {
		var existing []string
		if e := models.AnalyticsDB.Unscoped().Model(&models.Feedback{}).Where("source = ? AND source_id = ?", "discord", s.GuildID+":"+s.ChannelID+":"+s.ID).Distinct("project").Pluck("project", &existing).Error; e != nil {
			return nil, e
		}
		if len(existing) > 1 {
			return nil, fmt.Errorf("ambiguous existing Discord project")
		}
		if len(existing) == 1 {
			project = existing[0]
		}
	}
	if project == "" {
		var known models.DiscordSyncSource
		e := models.AnalyticsDB.Where("id = ? AND guild_id = ? AND kind = ?", scope, s.GuildID, kind).Take(&known).Error
		if e == nil {
			project = known.ProjectID
		} else if !errors.Is(e, gorm.ErrRecordNotFound) {
			return nil, e
		}
	}
	if project == "" {
		project = models.NormalizeProjectID(strings.TrimSpace(os.Getenv("DISCORD_MANUAL_DEFAULT_PROJECT")))
	}
	if project == "" {
		return nil, errDiscordManualProjectRequired
	}
	var n int64
	if e := models.AnalyticsDB.Model(&models.Project{}).Where("id = ?", project).Count(&n).Error; e != nil {
		return nil, e
	}
	if n != 1 {
		return nil, errDiscordManualProjectRequired
	}
	return &models.DiscordSyncSource{ID: scope, GuildID: s.GuildID, ProjectID: project, Kind: kind}, nil
}

func discordQueueScope(db *gorm.DB) *gorm.DB {
	return db.Where("(scope_id IN (?) OR (manual_trigger_id <> '' AND guild_id = ?))", db.Session(&gorm.Session{NewDB: true}).Model(&models.DiscordSyncSource{}).Select("id").Where("enabled = ?", true), models.LegacyDiscordGuild)
}
