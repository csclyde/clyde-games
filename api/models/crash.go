package models

import (
	"gorm.io/gorm"
)

type Crash struct {
	gorm.Model
	PID      string `gorm:"type:tinytext"`
	Project  string `gorm:"type:tinytext"`
	Message  string `gorm:"type:text"`
	Stack    string `gorm:"type:text"`
	Rating   uint8
	Env      string `gorm:"type:tinytext"`
	Category string `gorm:"type:tinytext"`
	Platform string `gorm:"type:tinytext"`
}

func SelectAllCrash() ([]Crash, error) {
	var allCrash []Crash
	result := AnalyticsDB.Order("created_at desc").Find(&allCrash)

	if result.Error != nil {
		return nil, result.Error
	}

	return allCrash, nil
}

func AddCrash(crash Crash) (*Crash, error) {
	result := AnalyticsDB.Create(&crash)

	if result.Error != nil {
		return nil, result.Error
	}

	return &crash, nil
}
