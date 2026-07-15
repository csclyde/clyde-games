package models

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const oldestBuildSetting = "oldest_build"

const (
	maxEventEssence = 100000
	maxEventTears   = 10000
)

var ErrBuildTooOld = errors.New("build is older than the oldest accepted build")

type Event struct {
	gorm.Model
	PID       string         `gorm:"type:varchar(32)"`
	SID       string         `gorm:"type:varchar(32)"`
	Env       string         `gorm:"type:varchar(32)"`
	Build     string         `gorm:"type:varchar(32)"`
	Project   string         `gorm:"type:varchar(32)"`
	Type      string         `gorm:"not null;type:varchar(32)"`
	Level     string         `gorm:"type:varchar(32)"`
	Cursemark int            `gorm:"type:tinyint"`
	Resources EventResources `gorm:"embedded;embeddedPrefix:resource_"`
	Actions   EventActions   `gorm:"embedded;embeddedPrefix:action_"`
	Items     []EventItem    `gorm:"foreignKey:EventID"`
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
	EventID uint   `gorm:"not null;index"`
	Item    string `gorm:"type:char(16);not null"`
	Slot    string `gorm:"type:char(16)"`
	Level   uint8  `gorm:"type:tinyint unsigned;not null"`
	Parent  string `gorm:"type:char(16);"`
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
	var allEvents []Event
	result := AnalyticsDB.Order("created_at desc").Find(&allEvents)

	if result.Error != nil {
		return nil, result.Error
	}

	return allEvents, nil
}

func SelectEventBuilds() ([]EventBuild, error) {
	var builds []EventBuild
	result := AnalyticsDB.Model(&Event{}).
		Select("build, project, count(*) as count").
		Where("build <> ''").
		Group("build, project").
		Order("build desc, project asc").
		Find(&builds)

	if result.Error != nil {
		return nil, result.Error
	}

	return builds, nil
}

func GetEventSettings() (*EventSettings, error) {
	var setting EventSetting
	result := AnalyticsDB.Where("`key` = ?", oldestBuildSetting).First(&setting)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return &EventSettings{}, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}

	return &EventSettings{OldestBuild: setting.Value}, nil
}

func SaveOldestEventBuild(build string) (*EventSettings, error) {
	if build != "" {
		if _, err := parseBuildDate(build); err != nil {
			return nil, fmt.Errorf("oldest build is not a valid date: %w", err)
		}

		var eventCount int64
		if err := AnalyticsDB.Model(&Event{}).Where("build = ?", build).Count(&eventCount).Error; err != nil {
			return nil, err
		}
		var savegameCount int64
		if err := AnalyticsDB.Model(&Savegame{}).Where("build = ?", build).Count(&savegameCount).Error; err != nil {
			return nil, err
		}
		if eventCount+savegameCount == 0 {
			return nil, errors.New("oldest build must be an existing metrics build")
		}
	}

	setting := EventSetting{Key: oldestBuildSetting}
	result := AnalyticsDB.Where("`key` = ?", oldestBuildSetting).FirstOrCreate(&setting)
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

func rejectBuildBeforeOldest(tx *gorm.DB, build string, subject string) error {
	var setting EventSetting
	result := tx.Where("`key` = ?", oldestBuildSetting).First(&setting)
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

// func AddEvent(event Event) (*Event, error) {
// 	result := AnalyticsDB.Create(&event)

// 	if result.Error != nil {
// 		return nil, result.Error
// 	}

// 	return &event, nil
// }

func AddEvent(event Event) (*Event, error) {
	event.Project = NormalizeProjectName(event.Project)
	if eventExceedsResourceLimits(event) {
		if err := BlockUser(event.PID); err != nil {
			return nil, fmt.Errorf("block user with excessive event resources: %w", err)
		}
		return nil, fmt.Errorf("%w: event resource limit exceeded", ErrUserBlocked)
	}

	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := rejectBlockedUser(tx, event.PID); err != nil {
			return err
		}
		if err := rejectBuildBeforeOldest(tx, event.Build, "event"); err != nil {
			return err
		}

		// Create the event itself (only)
		if err := tx.Omit("Items").Create(&event).Error; err != nil {
			return err
		}

		// If there are items, link them manually
		if len(event.Items) > 0 {
			for i := range event.Items {
				event.Items[i].EventID = event.ID
			}

			if err := tx.CreateInBatches(event.Items, len(event.Items)).Error; err != nil {
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

	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		eventQuery := tx.Unscoped().Model(&Event{}).Select("id").Where("build = ?", build)
		deleteQuery := tx.Unscoped().Where("build = ?", build)
		if project != "" {
			eventQuery = eventQuery.Where("project = ?", project)
			deleteQuery = deleteQuery.Where("project = ?", project)
		}

		eventIDs := eventQuery
		if err := tx.Where("event_id IN (?)", eventIDs).Delete(&EventItem{}).Error; err != nil {
			return err
		}

		result := deleteQuery.Delete(&Event{})
		if result.Error != nil {
			return result.Error
		}

		deleted = result.RowsAffected
		return nil
	})

	return deleted, err
}
