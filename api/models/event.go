package models

import (
	"time"

	"gorm.io/gorm"
)

type Event struct {
	gorm.Model
	PID       string      `gorm:"type:varchar(32)"`
	SID       string      `gorm:"type:varchar(32)"`
	Env       string      `gorm:"type:varchar(32)"`
	Build     time.Time   `gorm:"type:datetime"`
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

func SelectAllEvents() ([]Event, error) {
	var allEvents []Event
	result := AnalyticsDB.Order("created_at desc").Find(&allEvents)

	if result.Error != nil {
		return nil, result.Error
	}

	return allEvents, nil
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
