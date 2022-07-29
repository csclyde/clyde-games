package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Event struct {
	gorm.Model
	PID     string `gorm:"type:tinytext"`
	Env     string `gorm:"type:tinytext"`
	Project string `gorm:"type:tinytext"`
	Type    string `gorm:"not null;type:tinytext"`
	Save    datatypes.JSON
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
