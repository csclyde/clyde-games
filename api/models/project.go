package models

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const cursemarkProject = "cursemark"
const FarmBuilderProject = "farm-builder"
const farmBuilderName = "Farm Builder"

// Project is the durable identity and display metadata for an analytics project.
// ID is supplied by clients and is never changed; Name is editable by admins.
type Project struct {
	SteamSyncEnabled bool                `json:"steamSyncEnabled" gorm:"default:false"`
	DiscordSources   []DiscordSyncSource `json:"discordSources" gorm:"-"`
	ID               string              `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name             string              `json:"name" gorm:"not null;type:varchar(255)"`
	SteamAppID       string              `json:"steamAppId" gorm:"type:varchar(32)"`
}

type ProjectStats struct {
	SteamSyncEnabled bool                `json:"steamSyncEnabled"`
	DiscordSources   []DiscordSyncSource `json:"discordSources"`
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	SteamAppID       string              `json:"steamAppId"`
	SteamReviews     *SteamReviewStats   `json:"steamReviews,omitempty"`
	Crashes          int64               `json:"crashes"`
	Feedback         int64               `json:"feedback"`
	Savegames        int64               `json:"savegames"`
	Events           int64               `json:"events"`
}

type SteamReviewStats struct {
	CountedReviews     int     `json:"countedReviews"`
	TotalReviews       int     `json:"totalReviews"`
	PositiveReviews    int     `json:"positiveReviews"`
	NegativeReviews    int     `json:"negativeReviews"`
	PositivePercent    int     `json:"positivePercent"`
	PositivePercentRaw float64 `json:"positivePercentRaw"`
	ReviewScore        int     `json:"reviewScore"`
	ReviewScoreTag     string  `json:"reviewScoreTag"`
	UnavailableReason  string  `json:"unavailableReason,omitempty"`
}

type projectCount struct {
	Project string
	Count   int64
}

func NormalizeProjectID(id string) string {
	id = strings.TrimSpace(id)
	if strings.EqualFold(id, cursemarkProject) {
		return cursemarkProject
	}
	if strings.EqualFold(id, necrovaleProject) {
		return necrovaleProject
	}
	if strings.EqualFold(id, FarmBuilderProject) {
		return FarmBuilderProject
	}
	return id
}

// NormalizeProjectName is retained for analytics writers that previously used
// this function. The value stored on an analytics row is now the project ID.
func NormalizeProjectName(name string) string {
	return NormalizeProjectID(name)
}

func ensureProject(tx *gorm.DB, id string) error {
	id = NormalizeProjectID(id)
	if id == "" {
		return nil
	}
	project := Project{ID: id, Name: id}
	if id == FarmBuilderProject {
		project.Name = farmBuilderName
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&project).Error
}

// SelectProjectMetadata serves dropdowns without scanning analytics tables.
func SelectProjectMetadata() ([]Project, error) {
	var projects []Project
	err := AnalyticsDB.Order("name ASC, id ASC").Find(&projects).Error
	return projects, err
}

func SelectProjects() ([]ProjectStats, error) {
	var stored []Project
	if err := AnalyticsDB.Order("name ASC, id ASC").Find(&stored).Error; err != nil {
		return nil, err
	}

	projects := make(map[string]*ProjectStats, len(stored))
	for _, project := range stored {
		projects[project.ID] = &ProjectStats{ID: project.ID, Name: project.Name, SteamAppID: project.SteamAppID, SteamSyncEnabled: project.SteamSyncEnabled}
	}

	queries := []struct {
		model interface{}
		apply func(*ProjectStats, int64)
		count string
	}{
		{&Crash{}, func(project *ProjectStats, count int64) { project.Crashes += count }, "COALESCE(SUM(`count`), 0)"},
		{&Feedback{}, func(project *ProjectStats, count int64) { project.Feedback += count }, "COUNT(*)"},
		{&Savegame{}, func(project *ProjectStats, count int64) { project.Savegames += count }, "COUNT(*)"},
	}

	for _, query := range queries {
		var counts []projectCount
		if err := AnalyticsDB.Model(query.model).Select("project, " + query.count + " AS count").
			Where("TRIM(project) <> ''").Group("project").Find(&counts).Error; err != nil {
			return nil, err
		}
		for _, count := range counts {
			id := NormalizeProjectID(count.Project)
			project := projects[id]
			if project == nil {
				// Covers a record arriving between migration/backfill and this read.
				project = &ProjectStats{ID: id, Name: id}
				projects[id] = project
			}
			query.apply(project, count.Count)
		}
	}

	for _, store := range allEventStores() {
		var counts []projectCount
		if err := store.events(AnalyticsDB).Select("COALESCE(project, '') AS project, COUNT(*) AS count").Group("project").Find(&counts).Error; err != nil {
			return nil, err
		}
		for _, count := range counts {
			id := NormalizeProjectID(count.Project)
			if id == "" {
				id = store.project
				if id == "" {
					continue
				}
			}
			if projects[id] == nil {
				projects[id] = &ProjectStats{ID: id, Name: id}
			}
			projects[id].Events += count.Count
		}
	}

	result := make([]ProjectStats, 0, len(projects))
	for _, project := range projects {
		result = append(result, *project)
	}
	return result, nil
}

func SelectProject(id string) (*Project, error) {
	var project Project
	if err := AnalyticsDB.Where("id = ?", NormalizeProjectID(id)).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &project, nil
}

func SelectProjectsWithSteam() ([]Project, error) {
	var projects []Project
	err := AnalyticsDB.Where("TRIM(steam_app_id) <> '' AND steam_sync_enabled = ?", true).Order("id").Find(&projects).Error
	return projects, err
}

func BackfillProjects() error {
	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		for _, store := range gameMetricStores {
			if err := ensureProject(tx, store.project); err != nil {
				return err
			}
		}
		for _, model := range []interface{}{&Crash{}, &Feedback{}, &Savegame{}, &Event{}} {
			var ids []string
			if err := tx.Model(model).Distinct("project").Where("TRIM(project) <> ''").Pluck("project", &ids).Error; err != nil {
				return err
			}
			for _, id := range ids {
				if err := ensureProject(tx, id); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// SeedProjectSources migrates the Steam app ID that predated project settings.
// The marker makes this a one-time seed, so admins can later clear that setting.
func SeedProjectSources() error {
	const marker = "project_sources_v1"
	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&SyncState{}).Where("name = ?", marker).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if err := tx.Model(&Project{}).Where("id = ?", cursemarkProject).Updates(map[string]interface{}{
			"steam_app_id": "3219180",
		}).Error; err != nil {
			return err
		}
		return tx.Create(&SyncState{Name: marker, LastSuccessAt: time.Now()}).Error
	})
}

// ProjectNavigation contains only metadata and active-feedback counts.
type ProjectNavigation struct {
	FeedbackRevision *time.Time `json:"feedbackRevision,omitempty"`
	Project
	OpenFeedback        int64    `json:"openFeedback"`
	OpenCrashes         int64    `json:"openCrashes"`
	OpenFeedbackIDs     []string `json:"openFeedbackIds"`
	FeedbackIDsComplete bool     `json:"feedbackIdsComplete"`
}

func SelectProjectNavigation() ([]ProjectNavigation, error) {
	stored, err := SelectProjectMetadata()
	if err != nil {
		return nil, err
	}
	var counts []struct {
		Project  string
		Count    int64
		Revision *time.Time
	}
	if err := AnalyticsDB.Model(&Feedback{}).Select("project, COUNT(*) AS count, MAX(updated_at) AS revision").Where("resolved != true").Where("parent_feedback_id IS NULL").Group("project").Find(&counts).Error; err != nil {
		return nil, err
	}
	byID := make(map[string]int64)
	revisions := make(map[string]*time.Time)
	for _, c := range counts {
		key := strings.ToLower(NormalizeProjectID(c.Project))
		byID[key] += c.Count
		if c.Revision != nil && (revisions[key] == nil || c.Revision.After(*revisions[key])) {
			revisions[key] = c.Revision
		}
	}
	var crashes []projectCount
	if err := AnalyticsDB.Model(&Crash{}).Select("project, COUNT(*) AS count").Where("resolved != true").Group("project").Find(&crashes).Error; err != nil {
		return nil, err
	}
	crashByID := make(map[string]int64)
	for _, c := range crashes {
		crashByID[strings.ToLower(NormalizeProjectID(c.Project))] += c.Count
	}
	// IDs only: no message bodies, external lookups or schema changes. Include a
	// sentinel beyond the bound so we never claim a truncated snapshot complete.
	var active []struct {
		ID      uint64
		Project string
	}
	if err := AnalyticsDB.Model(&Feedback{}).Select("id, project").Where("resolved != true").Where("parent_feedback_id IS NULL").Order("id DESC").Limit(5001).Find(&active).Error; err != nil {
		return nil, err
	}
	complete := len(active) <= 5000
	if !complete {
		active = active[:5000]
	}
	ids := make(map[string][]string)
	for _, row := range active {
		key := strings.ToLower(NormalizeProjectID(row.Project))
		ids[key] = append(ids[key], strconv.FormatUint(row.ID, 10))
	}
	result := make([]ProjectNavigation, 0, len(stored))
	for _, p := range stored {
		key := strings.ToLower(p.ID)
		current := ids[key]
		if current == nil {
			current = []string{}
		}
		result = append(result, ProjectNavigation{Project: p, OpenFeedback: byID[key], OpenCrashes: crashByID[key], OpenFeedbackIDs: current, FeedbackRevision: revisions[key], FeedbackIDsComplete: complete && int64(len(current)) == byID[key]})
	}
	return result, nil
}

// Register the explicitly requested project independently from legacy clients.
func SeedPathProdigyProject() error {
	return AnalyticsDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&Project{ID: "pathprodigy", Name: "Path of the Prodigy"}).Error
}

func SeedFarmBuilderProject() error {
	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := ensureProject(tx, FarmBuilderProject); err != nil {
			return err
		}
		// An earlier client could have auto-registered the ID as its label.
		return tx.Model(&Project{}).Where("id = ? AND BINARY name = ?", FarmBuilderProject, FarmBuilderProject).
			Update("name", farmBuilderName).Error
	})
}

// Snapshot the legacy cutoff once. Each existing project then changes independently.
// New projects after this migration have no cutoff. The legacy untagged key stays intact.
func MigrateProjectBuildCutoffs() error {
	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		const marker = "project_build_cutoffs_v1"
		var count int64
		if err := tx.Model(&SyncState{}).Where("name = ?", marker).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		var old EventSetting
		if err := tx.Where("`key` = ?", oldestBuildSetting).First(&old).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var projects []Project
		if err := tx.Find(&projects).Error; err != nil {
			return err
		}
		for _, p := range projects {
			setting := EventSetting{Key: projectBuildSettingKey(p.ID), Value: old.Value}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&setting).Error; err != nil {
				return err
			}
		}
		return tx.Create(&SyncState{Name: marker, LastSuccessAt: time.Now()}).Error
	})
}
