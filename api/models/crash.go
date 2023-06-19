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
	Count    uint   `gorm:"type:int"`
	Resolved bool   `gorm:"type:boolean"`
}

func SelectAllCrash() ([]Crash, error) {
	var allCrash []Crash
	result := AnalyticsDB.Where("Resolved = false").Order("created_at desc").Find(&allCrash)

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
		existingCrash.Resolved = false
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

func ResolveCrash(hash string) (*Crash, error) {
	var existingCrash Crash
	result := AnalyticsDB.Where("Hash = ?", hash).First(&existingCrash)

	if result.RowsAffected > 0 {
		existingCrash.Resolved = true
		result = AnalyticsDB.Save(&existingCrash)
	} else {
		return nil, nil
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return &existingCrash, nil
}
