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
	Env      string `gorm:"type:tinytext"`
	Category string `gorm:"type:tinytext"`
	Platform string `gorm:"type:tinytext"`
	Build    string `gorm:"type:tinytext"`
	Commit   string `gorm:"type:tinytext"`
	Hash     string `gorm:"type:tinytext"`
	Count    uint
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

	var existingCrash Crash
	result := AnalyticsDB.Where("Hash = ?", crash.Hash).First(&existingCrash)

	if result.RowsAffected > 0 {
		existingCrash.Count += 1
		result = AnalyticsDB.Save(&existingCrash)
	} else {
		crash.Count = 1
		result = AnalyticsDB.Save(&crash)
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return &crash, nil
}
