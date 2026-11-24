package endpoints

import (
	"fmt"
	"log"
	"time"
	_ "time/tzdata"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

const ingestionOverlap = time.Hour

// Each source advances independently. Failure keeps the last successful window.
func runFeedbackSource(project models.Project, source, legacy string, collect func(time.Time, time.Time) (int, int, error)) (int, int, time.Time, time.Time, error) {
	name := source + "_" + project.ID
	state, err := models.GetSyncState(name)
	if err != nil {
		return 0, 0, time.Time{}, time.Time{}, err
	}
	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)
	if state != nil && !state.LastSuccessAt.IsZero() {
		from = state.LastSuccessAt.Add(-ingestionOverlap)
	} else if state != nil && state.WindowFrom != nil {
		from = *state.WindowFrom
	} else if legacy != "" {
		old, e := models.GetSyncState(legacy + project.ID)
		if e != nil {
			return 0, 0, from, to, e
		}
		if old != nil && !old.LastSuccessAt.IsZero() {
			from = old.LastSuccessAt.Add(-ingestionOverlap)
		}
	}
	if state != nil && state.BaselineAt != nil && from.Before(*state.BaselineAt) {
		from = *state.BaselineAt
	}
	if state == nil {
		state = &models.SyncState{Name: name}
	}
	state.LastAttemptAt = &to
	state.WindowFrom = &from
	state.WindowTo = &to
	state.Status = "running"
	state.LastError = ""
	state.Imported = 0
	state.Skipped = 0
	create := models.AnalyticsDB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoUpdates: clause.AssignmentColumns([]string{"last_attempt_at", "window_from", "window_to", "status", "last_error", "imported", "skipped", "updated_at"})})
	if state.LastSuccessAt.IsZero() {
		create = create.Omit("LastSuccessAt")
	}
	if err = create.Create(state).Error; err != nil {
		return 0, 0, from, to, err
	}
	imported, skipped, collectErr := collect(from, to)
	finished := time.Now().UTC()
	state.LastFinishedAt = &finished
	state.Imported = imported
	state.Skipped = skipped
	state.Status = "success"
	if collectErr != nil {
		state.Status = "failed"
		state.LastError, _ = mcpBound(collectErr.Error(), 1000)
	} else {
		state.LastSuccessAt = to
	}
	updates := map[string]interface{}{"last_finished_at": finished, "imported": imported, "skipped": skipped, "status": state.Status, "last_error": state.LastError}
	if collectErr == nil {
		updates["last_success_at"] = to
	}
	finish := models.AnalyticsDB.Model(&models.SyncState{}).Where("name = ?", name)
	if state.BaselineAt == nil {
		finish = finish.Where("baseline_at IS NULL")
	} else {
		finish = finish.Where("baseline_at = ?", *state.BaselineAt)
	}
	result := finish.Updates(updates)
	if result.Error != nil {
		return imported, skipped, from, to, result.Error
	}
	if result.RowsAffected == 0 {
		return imported, skipped, from, to, fmt.Errorf("source configuration changed during collection; checkpoint retained")
	}
	return imported, skipped, from, to, collectErr
}

func startFeedbackSchedule(name string, collect func()) {
	go func() {
		location, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			log.Printf("%s timezone: %v", name, err)
			return
		}
		for {
			next := nextSteamImportTime(time.Now(), location)
			log.Printf("next automatic %s import scheduled for %s", name, next.Format(time.RFC3339))
			timer := time.NewTimer(time.Until(next))
			<-timer.C
			collect()
		}
	}()
}

func mcpFeedbackIngestionStatus(c *gin.Context) {
	projects := []models.Project{}
	if e := mcpDB(c).Order("id").Limit(201).Find(&projects).Error; e != nil {
		mcpDBError(c, e)
		return
	}
	if len(projects) > 200 {
		c.JSON(413, gin.H{"index_complete": false})
		return
	}
	items := []gin.H{}
	add := func(project, kind, name string, enabled bool, remote string) error {
		s, e := models.GetSyncState(name)
		if e != nil {
			return e
		}
		if s == nil {
			s = &models.SyncState{Status: "never_run"}
		}
		items = append(items, gin.H{"project": project, "source": kind, "source_id": remote, "enabled": enabled, "status": s.Status, "last_success_at": s.LastSuccessAt, "last_attempt_at": s.LastAttemptAt, "last_finished_at": s.LastFinishedAt, "baseline_at": s.BaselineAt, "window_from": s.WindowFrom, "window_to": s.WindowTo, "imported": s.Imported, "skipped": s.Skipped, "last_error": s.LastError})
		return nil
	}
	for _, p := range projects {
		if p.SteamAppID != "" {
			for _, k := range []string{"steam_reviews", "steam_discussions"} {
				if e := add(p.ID, k, k+"_"+p.ID, p.SteamSyncEnabled, p.SteamAppID); e != nil {
					mcpDBError(c, e)
					return
				}
			}
		}
	}
	sources, e := models.SelectDiscordSyncSources("", false)
	if e != nil {
		mcpDBError(c, e)
		return
	}
	for _, s := range sources {
		if e := add(s.ProjectID, s.Kind, models.DiscordSyncName(s), s.Enabled, s.ID); e != nil {
			mcpDBError(c, e)
			return
		}
	}
	loc, _ := time.LoadLocation("America/Los_Angeles")
	c.JSON(200, gin.H{"items": items, "discord_sources": sources, "index_complete": true, "schedule_timezone": "America/Los_Angeles", "schedule_local_time": "05:00", "next_run_at": nextSteamImportTime(time.Now(), loc), "overlap_seconds": 3600, "owner_baseline_at": models.OwnerFeedbackBaseline, "new_source_policy": "baseline at enable/change request; no historical import", "forum_policy": "one issue per starter; replies context only; existing markers and pre-baseline starters handled"})
}
