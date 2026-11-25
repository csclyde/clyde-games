package models

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

const oldestBuildSetting = "oldest_build"
const oldestCrashBuildSetting = "oldest_crash_build"

const (
	maxEventEssence = 100000
	maxEventTears   = 10000
)

var ErrBuildTooOld = errors.New("build is older than the oldest accepted build")

type Event struct {
	gorm.Model
	PID                  string `gorm:"type:varchar(32)"`
	SID                  string `gorm:"type:varchar(32)"`
	Env                  string `gorm:"type:varchar(32)"`
	Build                string `gorm:"type:varchar(32)"`
	Project              string `gorm:"type:varchar(32)"`
	Type                 string `gorm:"not null;type:varchar(32)"`
	Level                string `gorm:"type:varchar(128)"`
	Area                 string `gorm:"type:varchar(128)"`
	Mode                 string `gorm:"type:varchar(24)"`
	Difficulty           *float64
	RunStep              *int
	Commit               string         `gorm:"type:varchar(64)"`
	Platform             string         `gorm:"type:varchar(32)"`
	Cursemark            int            `gorm:"type:tinyint"`
	Resources            EventResources `gorm:"embedded;embeddedPrefix:resource_"`
	Actions              EventActions   `gorm:"embedded;embeddedPrefix:action_"`
	Hopestones           *float64
	Gold                 *float64
	LifetimeClears       *uint64
	LifetimeDeaths       *uint64
	ClearHistoryComplete *bool
	DeathHistoryComplete *bool
	Items                []EventItem `gorm:"foreignKey:EventID"`
}

type EventResources struct {
	Essence int `gorm:"type:int"`
	Tears   int `gorm:"type:int"`
}

type EventActions struct {
	Attack int `gorm:"type:int"`
	Spell  int `gorm:"type:int"`
	Ward   int `gorm:"type:int"`
	Ult    int `gorm:"type:int"`
	Botyl  int `gorm:"type:int"`
}

type EventItem struct {
	EventID   uint   `gorm:"not null;index"`
	Item      string `gorm:"type:varchar(128);not null"`
	Slot      string `gorm:"type:char(16)"`
	Level     int    `gorm:"type:int;not null"`
	Parent    string `gorm:"type:varchar(128)"`
	SlotIndex *int
	MeldPath  string `gorm:"type:varchar(128)"`
}

type EventBuild struct {
	Build   string
	Project string
	Count   int64
}

type EventSetting struct {
	Key   string `gorm:"primaryKey;type:varchar(64)"`
	Value string `gorm:"type:varchar(64)"`
}

type EventSettings struct {
	OldestBuild string
}

func SelectAllEvents() ([]Event, error) {
	return selectEvents(nil)
}

func SelectEventBuilds() ([]EventBuild, error) {
	builds := make([]EventBuild, 0)
	for _, store := range allEventStores() {
		var stored []EventBuild
		if err := store.events(AnalyticsDB).
			Select("build, project, count(*) as count").
			Where("build <> ''").
			Group("build, project").Find(&stored).Error; err != nil {
			return nil, err
		}
		builds = append(builds, stored...)
	}
	sort.Slice(builds, func(i, j int) bool {
		if builds[i].Build == builds[j].Build {
			return builds[i].Project < builds[j].Project
		}
		return builds[i].Build > builds[j].Build
	})
	return builds, nil
}

func selectEvents(pid *string) ([]Event, error) {
	events := make([]Event, 0)
	for _, store := range allEventStores() {
		query := store.events(AnalyticsDB)
		if pid != nil {
			query = query.Where("p_id = ?", *pid).Preload("Items", func(db *gorm.DB) *gorm.DB {
				return db.Table(store.table("event_items"))
			})
		}
		var stored []Event
		if err := query.Order("created_at DESC, id DESC").Find(&stored).Error; err != nil {
			return nil, err
		}
		events = append(events, stored...)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].CreatedAt.Equal(events[j].CreatedAt) {
			return events[i].ID > events[j].ID
		}
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})
	return events, nil
}

func GetEventSettings(projects ...string) (*EventSettings, error) {
	project := settingsProject(projects)
	var setting EventSetting
	result := AnalyticsDB.Where("`key` = ?", projectBuildSettingKey(project)).First(&setting)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return &EventSettings{}, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}

	return &EventSettings{OldestBuild: setting.Value}, nil
}

func SaveOldestEventBuild(build string, projects ...string) (*EventSettings, error) {
	project := settingsProject(projects)
	existing, err := SelectProject(project)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("unknown project")
	}
	if build != "" {
		if _, err := parseBuildDate(build); err != nil {
			return nil, fmt.Errorf("oldest build is not a valid date: %w", err)
		}

		var eventCount int64
		if err := eventStoreFor(project).events(AnalyticsDB).Where("build = ? AND project = ?", build, project).Count(&eventCount).Error; err != nil {
			return nil, err
		}
		var savegameCount int64
		if err := AnalyticsDB.Model(&Savegame{}).Where("build = ? AND project = ?", build, project).Count(&savegameCount).Error; err != nil {
			return nil, err
		}
		if eventCount+savegameCount == 0 {
			return nil, errors.New("oldest build must be an existing metrics build")
		}
	}

	setting := EventSetting{Key: projectBuildSettingKey(project)}
	result := AnalyticsDB.Where("`key` = ?", setting.Key).FirstOrCreate(&setting)
	if result.Error != nil {
		return nil, result.Error
	}
	setting.Value = build
	if err := AnalyticsDB.Save(&setting).Error; err != nil {
		return nil, err
	}

	return &EventSettings{OldestBuild: build}, nil
}

func parseBuildDate(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999999",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date %q", value)
}

func buildIsBefore(build string, oldestBuild string) (bool, error) {
	buildDate, err := parseBuildDate(build)
	if err != nil {
		return false, fmt.Errorf("build is not a valid date: %w", err)
	}
	oldestDate, err := parseBuildDate(oldestBuild)
	if err != nil {
		return false, fmt.Errorf("configured oldest build is not a valid date: %w", err)
	}
	return buildDate.Before(oldestDate), nil
}

func rejectBuildBeforeOldest(tx *gorm.DB, build string, subject string, projects ...string) error {
	project := ""
	if len(projects) > 0 {
		project = projects[0]
	}
	return rejectBuildBeforeSetting(tx, projectBuildSettingKey(project), build, subject)
}
func settingsProject(projects []string) string {
	if len(projects) == 0 || strings.TrimSpace(projects[0]) == "" {
		return "cursemark"
	}
	return NormalizeProjectID(projects[0])
}
func projectBuildSettingKey(project string) string {
	project = NormalizeProjectID(project)
	if project == "" {
		return oldestBuildSetting
	} // Legacy clients without a Project retain the original cutoff.
	hash := sha256.Sum256([]byte(strings.ToLower(project)))
	return fmt.Sprintf("oldest_build:%x", hash[:20])
}

func rejectBuildBeforeSetting(tx *gorm.DB, settingKey string, build string, subject string) error {
	var setting EventSetting
	result := tx.Where("`key` = ?", settingKey).First(&setting)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}
	if result.Error != nil || setting.Value == "" {
		return nil
	}

	tooOld, err := buildIsBefore(build, setting.Value)
	if err != nil {
		return fmt.Errorf("%s %w", subject, err)
	}
	if tooOld {
		return fmt.Errorf("%w: %s build %s is before %s", ErrBuildTooOld, subject, build, setting.Value)
	}
	return nil
}

func AddEvent(event Event) (*Event, error) {
	event.Project = NormalizeProjectName(event.Project)
	store := eventStoreFor(event.Project)
	if event.Project == FarmBuilderProject {
		var err error
		store, err = metricsStoreFor(event.Project)
		if err != nil {
			return nil, err
		}
		if err := validateFarmBuilderSeason(event); err != nil {
			return nil, err
		}
	}
	if event.Project != necrovaleProject && event.Project != FarmBuilderProject && eventExceedsResourceLimits(event) {
		if err := BlockUser(event.PID); err != nil {
			return nil, fmt.Errorf("block user with excessive event resources: %w", err)
		}
		return nil, fmt.Errorf("%w: event resource limit exceeded", ErrUserBlocked)
	}

	if event.Project == necrovaleProject {
		for _, balance := range []*float64{event.Hopestones, event.Gold} {
			if balance != nil && (*balance < 0 || math.IsNaN(*balance) || math.IsInf(*balance, 0)) {
				return nil, errors.New("balances must be finite and nonnegative")
			}
		}
	}
	if len(gameMetricStores) > 0 {
		id, err := nextGameEventID()
		if err != nil {
			return nil, err
		}
		event.ID = id
	}

	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := ensureProject(tx, event.Project); err != nil {
			return err
		}
		if err := rejectBlockedUser(tx, event.PID); err != nil {
			return err
		}
		if err := rejectBuildBeforeOldest(tx, event.Build, "event", event.Project); err != nil {
			return err
		}
		if store.project == FarmBuilderProject {
			return store.addFarmBuilderSeason(tx, &event)
		}

		omit := []string{"Items"}
		if len(gameMetricStores) > 0 {
			if store.project == necrovaleProject {
				omit = append(omit, cursemarkColumns...)
			} else {
				for _, column := range necrovaleColumns {
					omit = append(omit, column.name)
				}
			}
		}
		if err := store.events(tx).Omit(omit...).Create(&event).Error; err != nil {
			return err
		}

		// If there are items, link them manually
		if len(event.Items) > 0 {
			for i := range event.Items {
				event.Items[i].EventID = event.ID
			}

			if err := store.items(tx).CreateInBatches(event.Items, len(event.Items)).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &event, nil
}

func eventExceedsResourceLimits(event Event) bool {
	return event.Resources.Essence > maxEventEssence || event.Resources.Tears > maxEventTears
}

func DeleteEventsByBuild(build string, project string) (int64, error) {
	var deleted int64
	project = NormalizeProjectID(project)
	stores := allEventStores()
	if project != "" {
		stores = []metricStore{eventStoreFor(project)}
	}
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		for _, store := range stores {
			query := store.events(tx).Unscoped().Where("build = ?", build)
			if project != "" {
				query = query.Where("project = ?", project)
			}
			count, err := store.deleteEvents(tx, query)
			if err != nil {
				return err
			}
			deleted += count
		}
		return nil
	})
	return deleted, err
}

func (s metricStore) deleteEvents(tx, query *gorm.DB) (int64, error) {
	eventIDs := query.Session(&gorm.Session{}).Select("id")
	if err := s.items(tx).Where("event_id IN (?)", eventIDs).Delete(&EventItem{}).Error; err != nil {
		return 0, err
	}
	result := query.Delete(&Event{})
	return result.RowsAffected, result.Error
}
