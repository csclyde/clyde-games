package models

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

// The initial IDs are migration evidence only. Runtime routing reads this table.
const LegacyDiscordGuild = "1169710427387744297"
const LegacyDiscordText = "1406020405231353937"
const LegacyDiscordForum = "1513944000158240768"

var OwnerFeedbackBaseline = time.Date(2026, 11, 16, 5, 58, 21, 0, time.UTC)

type DiscordSyncSource struct {
	ID         string    `json:"id" gorm:"primaryKey;type:varchar(32)"`
	ProjectID  string    `json:"projectId" gorm:"type:varchar(64);index"`
	GuildID    string    `json:"guildId" gorm:"type:varchar(32)"`
	Kind       string    `json:"kind" gorm:"type:varchar(32)"`
	Name       string    `json:"name" gorm:"type:varchar(255)"`
	Enabled    bool      `json:"enabled"`
	BaselineAt time.Time `json:"baselineAt"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func DiscordSyncName(s DiscordSyncSource) string { return "discord_source_" + s.ID + "_" + s.ProjectID }
func SelectDiscordSyncSources(project string, enabled bool) ([]DiscordSyncSource, error) {
	out := []DiscordSyncSource{}
	q := AnalyticsDB.Order("project_id,id")
	if project != "" {
		q = q.Where("project_id = ?", project)
	}
	if enabled {
		q = q.Where("enabled = ?", true)
	}
	e := q.Find(&out).Error
	return out, e
}
func AttachDiscordSources(projects []Project) error {
	all, e := SelectDiscordSyncSources("", false)
	if e != nil {
		return e
	}
	for i := range projects {
		projects[i].DiscordSources = []DiscordSyncSource{}
		for _, s := range all {
			if s.ProjectID == projects[i].ID {
				projects[i].DiscordSources = append(projects[i].DiscordSources, s)
			}
		}
	}
	return nil
}
func BaselineSync(tx *gorm.DB, name string, at time.Time) error {
	var old SyncState
	e := tx.Where("name = ?", name).Take(&old).Error
	if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
		return e
	}
	if old.ID == 0 {
		old = SyncState{Name: name, LastSuccessAt: at, BaselineAt: &at, Status: "baselined"}
		return tx.Create(&old).Error
	}
	updates := map[string]interface{}{"baseline_at": at, "window_from": at, "status": "baselined", "last_error": ""}
	if old.LastSuccessAt.Before(at) {
		updates["last_success_at"] = at
	}
	return tx.Model(&old).Updates(updates).Error
}
func MigrateFeedbackSyncConfiguration() error {
	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		const marker = "feedback_sync_configuration_v2"
		var n int64
		if e := tx.Model(&SyncState{}).Where("name = ?", marker).Count(&n).Error; e != nil {
			return e
		}
		if n > 0 {
			return nil
		}
		for _, s := range []DiscordSyncSource{{ID: LegacyDiscordText, ProjectID: "cursemark", GuildID: LegacyDiscordGuild, Kind: "discord_text", Name: "cursemark-feedback", Enabled: true, BaselineAt: OwnerFeedbackBaseline}, {ID: LegacyDiscordForum, ProjectID: "cursemark", GuildID: LegacyDiscordGuild, Kind: "discord_forum", Name: "bugs-and-feedback", Enabled: true, BaselineAt: OwnerFeedbackBaseline}} {
			if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&s).Error; e != nil {
				return e
			}
			if e := BaselineSync(tx, DiscordSyncName(s), OwnerFeedbackBaseline); e != nil {
				return e
			}
		}
		var projects []Project
		if e := tx.Find(&projects).Error; e != nil {
			return e
		}
		for _, p := range projects {
			if e := tx.Model(&Project{}).Where("id = ?", p.ID).Updates(map[string]interface{}{"steam_sync_enabled": p.SteamAppID != ""}).Error; e != nil {
				return e
			}
			names := []string{}
			if p.SteamAppID != "" {
				names = append(names, "steam_reviews_"+p.ID, "steam_discussions_"+p.ID)
			}
			for _, name := range names {
				if e := BaselineSync(tx, name, OwnerFeedbackBaseline); e != nil {
					return e
				}
			}
		}
		if e := tx.Model(&DiscordPendingMessage{}).Where("channel_id = ? AND (scope_id = ? OR scope_id IS NULL)", LegacyDiscordText, "").Updates(map[string]interface{}{"scope_id": LegacyDiscordText, "kind": "discord_text", "original_snapshot": gorm.Expr("snapshot")}).Error; e != nil {
			return e
		}
		if e := tx.Model(&DiscordPendingMessage{}).Where("feedback_id = 0 AND source_created_at < ?", OwnerFeedbackBaseline).Updates(map[string]interface{}{"disposition": "handled", "needs_review": false, "mark_suppressed": true}).Error; e != nil {
			return e
		}
		return tx.Create(&SyncState{Name: marker, LastSuccessAt: OwnerFeedbackBaseline, Status: "baselined"}).Error
	})
}
func validateProjectSyncFields(name, app string) error {
	if strings.TrimSpace(name) == "" || len(name) > 255 {
		return errors.New("project name is required, at most 255 characters")
	}
	if len(app) > 32 {
		return errors.New("invalid Steam app ID")
	}
	for _, c := range app {
		if c < '0' || c > '9' {
			return errors.New("Steam app ID must contain only numbers")
		}
	}
	return nil
}
func UpdateProjectWithSyncs(id, name, app string, steam *bool, sources *[]DiscordSyncSource, at time.Time) (*Project, error) {
	id = NormalizeProjectID(id)
	name = strings.TrimSpace(name)
	app = strings.TrimSpace(app)
	if e := validateProjectSyncFields(name, app); e != nil {
		return nil, e
	}
	var project Project
	e := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, "id = ?", id).Error; e != nil {
			return e
		}
		old := project
		project.Name = name
		project.SteamAppID = app
		if steam == nil && app == "" {
			project.SteamSyncEnabled = false
		}
		if steam != nil {
			project.SteamSyncEnabled = *steam
		}
		if project.SteamSyncEnabled && app == "" {
			return errors.New("enabled Steam sync needs an app ID")
		}
		if e := tx.Model(&Project{}).Where("id = ?", id).Updates(map[string]interface{}{"name": name, "steam_app_id": app, "steam_sync_enabled": project.SteamSyncEnabled}).Error; e != nil {
			return e
		}
		if project.SteamSyncEnabled && (!old.SteamSyncEnabled || old.SteamAppID != app) {
			for _, n := range []string{"steam_reviews_", "steam_discussions_"} {
				if e := BaselineSync(tx, n+id, at); e != nil {
					return e
				}
			}
		}
		if sources != nil {
			if len(*sources) > 20 {
				return errors.New("at most 20 Discord sources per project")
			}
			seen := map[string]bool{}
			for _, s := range *sources {
				if seen[s.ID] {
					return errors.New("duplicate Discord source")
				}
				seen[s.ID] = true
				var existing DiscordSyncSource
				e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", s.ID).Take(&existing).Error
				if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
					return e
				}
				if s.Kind != "discord_text" && s.Kind != "discord_forum" {
					return errors.New("invalid Discord source kind")
				}
				if existing.ID != "" && existing.Kind != s.Kind {
					return errors.New("Discord source type cannot change")
				}
				if existing.ID != "" && existing.ProjectID != id {
					return fmt.Errorf("Discord source %s belongs to another project", s.ID)
				}
				s.ProjectID = id
				s.BaselineAt = existing.BaselineAt
				if existing.ID == "" || (!existing.Enabled && s.Enabled) {
					s.BaselineAt = at
					if e := BaselineSync(tx, DiscordSyncName(s), at); e != nil {
						return e
					}
					if e := tx.Model(&DiscordPendingMessage{}).Where("scope_id = ? AND feedback_id = 0 AND source_created_at < ?", s.ID, at).Updates(map[string]interface{}{"disposition": "handled", "needs_review": false}).Error; e != nil {
						return e
					}
				}
				if e := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&s).Error; e != nil {
					return e
				}
			}
			if e := tx.Model(&DiscordSyncSource{}).Where("project_id = ?", id).Where("id NOT IN ?", append([]string{""}, keys(seen)...)).Update("enabled", false).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	project.DiscordSources, e = SelectDiscordSyncSources(id, false)
	return &project, e
}
func keys(m map[string]bool) []string {
	a := []string{}
	for k := range m {
		a = append(a, k)
	}
	return a
}
