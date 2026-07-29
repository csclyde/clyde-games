package models

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const cursemarkProject = "cursemark"

// Project is the durable identity and display metadata for an analytics project.
// ID is supplied by clients and is never changed; Name is editable by admins.
type Project struct {
	ID           string `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name         string `json:"name" gorm:"not null;type:varchar(255)"`
	SteamAppID   string `json:"steamAppId" gorm:"type:varchar(32)"`
	SubredditURL string `json:"subredditUrl" gorm:"type:varchar(512)"`
}

type ProjectStats struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	SteamAppID   string            `json:"steamAppId"`
	SubredditURL string            `json:"subredditUrl"`
	SteamReviews *SteamReviewStats `json:"steamReviews,omitempty"`
	Crashes      int64             `json:"crashes"`
	Feedback     int64             `json:"feedback"`
	Savegames    int64             `json:"savegames"`
	Events       int64             `json:"events"`
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
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&project).Error
}

func SelectProjects() ([]ProjectStats, error) {
	var stored []Project
	if err := AnalyticsDB.Order("name ASC, id ASC").Find(&stored).Error; err != nil {
		return nil, err
	}

	projects := make(map[string]*ProjectStats, len(stored))
	for _, project := range stored {
		projects[project.ID] = &ProjectStats{ID: project.ID, Name: project.Name, SteamAppID: project.SteamAppID, SubredditURL: project.SubredditURL}
	}

	queries := []struct {
		model interface{}
		apply func(*ProjectStats, int64)
		count string
	}{
		{&Crash{}, func(project *ProjectStats, count int64) { project.Crashes += count }, "COALESCE(SUM(`count`), 0)"},
		{&Feedback{}, func(project *ProjectStats, count int64) { project.Feedback += count }, "COUNT(*)"},
		{&Savegame{}, func(project *ProjectStats, count int64) { project.Savegames += count }, "COUNT(*)"},
		{&Event{}, func(project *ProjectStats, count int64) { project.Events += count }, "COUNT(*)"},
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

	result := make([]ProjectStats, 0, len(projects))
	for _, project := range projects {
		result = append(result, *project)
	}
	return result, nil
}

func UpdateProject(id, name, steamAppID, subredditURL string) (*Project, error) {
	id = NormalizeProjectID(id)
	name = strings.TrimSpace(name)
	steamAppID = strings.TrimSpace(steamAppID)
	subredditURL = strings.TrimSpace(subredditURL)
	if id == "" || name == "" {
		return nil, errors.New("project id and name are required")
	}
	if len(name) > 255 {
		return nil, errors.New("project name must be 255 characters or fewer")
	}
	if steamAppID != "" {
		for _, character := range steamAppID {
			if character < '0' || character > '9' {
				return nil, errors.New("Steam app ID must contain only numbers")
			}
		}
	}
	if subredditURL != "" {
		parsed, err := url.Parse(subredditURL)
		host := strings.ToLower(parsed.Hostname())
		if err != nil || parsed.Scheme != "https" || (host != "reddit.com" && !strings.HasSuffix(host, ".reddit.com")) || !strings.HasPrefix(parsed.Path, "/r/") {
			return nil, errors.New("subreddit URL must be an HTTPS reddit.com /r/ link")
		}
	}

	var project Project
	if err := AnalyticsDB.Where("id = ?", id).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	updates := map[string]interface{}{"name": name, "steam_app_id": steamAppID, "subreddit_url": subredditURL}
	if err := AnalyticsDB.Model(&project).Updates(updates).Error; err != nil {
		return nil, err
	}
	project.Name = name
	project.SteamAppID = steamAppID
	project.SubredditURL = subredditURL
	return &project, nil
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
	err := AnalyticsDB.Where("TRIM(steam_app_id) <> ''").Order("id").Find(&projects).Error
	return projects, err
}

func SelectProjectsWithSubreddit() ([]Project, error) {
	var projects []Project
	err := AnalyticsDB.Where("TRIM(subreddit_url) <> ''").Order("id").Find(&projects).Error
	return projects, err
}

func BackfillProjects() error {
	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
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

// SeedProjectSources migrates the two values that predated project settings.
// The marker makes this a one-time seed, so admins can later clear either field.
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
			"steam_app_id": "3219180", "subreddit_url": "https://www.reddit.com/r/cursemark",
		}).Error; err != nil {
			return err
		}
		return tx.Create(&SyncState{Name: marker, LastSuccessAt: time.Now()}).Error
	})
}
