package models

import (
	"errors"
	"fmt"
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

type CrashBuild struct {
	Build   string
	Project string
	Count   int64
}

type CrashSettings struct {
	OldestBuild string
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

func SelectCrashBuilds() ([]CrashBuild, error) {
	var builds []CrashBuild
	result := AnalyticsDB.Model(&Crash{}).
		Select("build, project, count(*) as count").
		Where("build <> ''").
		Group("build, project").
		Order("build desc, project asc").
		Find(&builds)

	if result.Error != nil {
		return nil, result.Error
	}

	return builds, nil
}

func GetCrashSettings() (*CrashSettings, error) {
	var setting EventSetting
	result := AnalyticsDB.Where("`key` = ?", oldestCrashBuildSetting).First(&setting)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return &CrashSettings{}, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}

	return &CrashSettings{OldestBuild: setting.Value}, nil
}

func SaveOldestCrashBuild(build string) (*CrashSettings, error) {
	if build != "" {
		if _, err := parseBuildDate(build); err != nil {
			return nil, fmt.Errorf("oldest build is not a valid date: %w", err)
		}

		var crashCount int64
		if err := AnalyticsDB.Model(&Crash{}).Where("build = ?", build).Count(&crashCount).Error; err != nil {
			return nil, err
		}
		if crashCount == 0 {
			return nil, errors.New("oldest build must be an existing crash build")
		}
	}

	setting := EventSetting{Key: oldestCrashBuildSetting}
	result := AnalyticsDB.Where("`key` = ?", oldestCrashBuildSetting).FirstOrCreate(&setting)
	if result.Error != nil {
		return nil, result.Error
	}
	setting.Value = build
	if err := AnalyticsDB.Save(&setting).Error; err != nil {
		return nil, err
	}

	return &CrashSettings{OldestBuild: build}, nil
}

func AddCrash(crash Crash) (*Crash, error) {
	crash.Project = NormalizeProjectName(crash.Project)
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := ensureProject(tx, crash.Project); err != nil {
			return err
		}
		if err := rejectBuildBeforeSetting(tx, oldestCrashBuildSetting, crash.Build, "crash"); err != nil {
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
