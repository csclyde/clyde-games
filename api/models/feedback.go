package models

import (
	"gorm.io/gorm"
)

type Feedback struct {
	gorm.Model
	PID      string `gorm:"type:tinytext"`
	Project  string `gorm:"type:tinytext"`
	Message  string `gorm:"type:text"`
	Rating   uint8
	Env      string `gorm:"type:tinytext"`
	Category string `gorm:"type:tinytext"`
	Platform string `gorm:"type:tinytext"`
	FPS      uint16
}

func SelectAllFeedback() ([]Feedback, error) {
	db, err := getDB("analytics")
	if err != nil {
		return nil, err
	}

	var allFeedback []Feedback
	result := db.Order("created_at desc").Find(&allFeedback)

	if result.Error != nil {
		return nil, result.Error
	}

	return allFeedback, nil
}

func AddFeedback(fb Feedback) (*Feedback, error) {
	db, err := getDB("analytics")
	if err != nil {
		return nil, err
	}

	result := db.Create(&fb)

	if result.Error != nil {
		return nil, result.Error
	}

	return &fb, nil
}
