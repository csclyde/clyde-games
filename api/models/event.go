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
	Spell     string      `gorm:"type:varchar(32)"`
	Ward      string      `gorm:"type:varchar(32)"`
	Ult       string      `gorm:"type:varchar(32)"`
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

func AddEvent(event Event) (*Event, error) {
	result := AnalyticsDB.Create(&event)

	if result.Error != nil {
		return nil, result.Error
	}

	return &event, nil
}
