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
	var allFeedback []Feedback
	result := AnalyticsDB.Order("created_at desc").Find(&allFeedback)

	if result.Error != nil {
		return nil, result.Error
	}

	return allFeedback, nil
}

func AddFeedback(fb Feedback) (*Feedback, error) {
	result := AnalyticsDB.Create(&fb)

	if result.Error != nil {
		return nil, result.Error
	}

	return &fb, nil
}
