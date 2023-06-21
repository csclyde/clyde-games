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
	Resolved bool `gorm:"type:boolean;default:false"`
}

func SelectAllFeedback() ([]Feedback, error) {
	var allFeedback []Feedback
	result := AnalyticsDB.Where("resolved != true").Order("created_at desc").Find(&allFeedback)

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

func ResolveFeedback(id string) (*Feedback, error) {
	var existingFeedback Feedback
	result := AnalyticsDB.Where("id = ?", id).First(&existingFeedback)

	if result.RowsAffected > 0 {
		existingFeedback.Resolved = true
		result = AnalyticsDB.Save(&existingFeedback)
	} else {
		return nil, nil
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return &existingFeedback, nil
}
