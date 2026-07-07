package models

import (
	"gorm.io/gorm"
)

type Event struct {
	gorm.Model
	PID       string      `gorm:"type:varchar(32)"`
	SID       string      `gorm:"type:varchar(32)"`
	Env       string      `gorm:"type:varchar(32)"`
	Build     string      `gorm:"type:varchar(32)"`
	Project   string      `gorm:"type:varchar(32)"`
	Type      string      `gorm:"not null;type:varchar(32)"`
	Level     string      `gorm:"type:varchar(32)"`
	Cursemark int         `gorm:"type:tinyint"`
	Items     []EventItem `gorm:"foreignKey:EventID"`
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

// func AddEvent(event Event) (*Event, error) {
// 	result := AnalyticsDB.Create(&event)

// 	if result.Error != nil {
// 		return nil, result.Error
// 	}

// 	return &event, nil
// }

func AddEvent(event Event) (*Event, error) {
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
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
