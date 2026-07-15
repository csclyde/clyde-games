package models

import (
	"time"

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
	Resolved bool   `gorm:"type:boolean;default:false"`
}

func SelectAllCrash() ([]Crash, error) {
	var allCrash []Crash
	result := AnalyticsDB.Where("resolved != true").Order("created_at desc").Find(&allCrash)

	if result.Error != nil {
		return nil, result.Error
	}

	return allCrash, nil
}

func SelectAccessViolationCrashesSince(since time.Time) ([]Crash, error) {
	var crashes []Crash
	result := AnalyticsDB.
		Where("message LIKE ?", "%Access Violation%").
		Where("created_at >= ?", since).
		Order("created_at desc").
		Find(&crashes)

	if result.Error != nil {
		return nil, result.Error
	}

	return crashes, nil
}

func AddCrash(crash Crash) (*Crash, error) {
	crash.Project = NormalizeProjectName(crash.Project)
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := ensureProject(tx, crash.Project); err != nil {
			return err
		}
		var existingCrash Crash
		result := tx.Where("hash = ?", crash.Hash).First(&existingCrash)
		if result.RowsAffected > 0 {
			existingCrash.Count++
			existingCrash.Resolved = false
			return tx.Save(&existingCrash).Error
		}
		crash.Count = 1
		crash.Resolved = false
		return tx.Save(&crash).Error
	})
	if err != nil {
		return nil, err
	}

	return &crash, nil
}

func ResolveCrash(hash string) (*Crash, error) {
	var existingCrash Crash
	result := AnalyticsDB.Where("hash = ?", hash).First(&existingCrash)

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

func ResolveAllCrashes() error {
	result := AnalyticsDB.Model(&Crash{}).Where("resolved != true").Update("resolved", true)
	return result.Error
}
