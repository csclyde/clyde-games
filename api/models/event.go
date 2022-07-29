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
	db, err := getDB("analytics")
	if err != nil {
		return nil, err
	}

	var allEvents []Event
	result := db.Order("created_at desc").Find(&allEvents)

	if result.Error != nil {
		return nil, result.Error
	}

	return allEvents, nil
}

func AddEvent(event Event) (*Event, error) {
	db, err := getDB("analytics")
	if err != nil {
		return nil, err
	}

	result := db.Create(&event)

	if result.Error != nil {
		return nil, result.Error
	}

	return &event, nil
}
