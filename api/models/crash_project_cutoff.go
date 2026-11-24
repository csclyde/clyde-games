package models

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

func projectCrashSettingKey(project string) string {
	project = NormalizeProjectID(project)
	if project == "" {
		return oldestCrashBuildSetting
	}
	sum := sha256.Sum256([]byte(strings.ToLower(project)))
	return fmt.Sprintf("oldest_crash:%x", sum[:20])
}
func crashSettingsProject(projects []string) string {
	if len(projects) == 0 {
		return ""
	}
	return NormalizeProjectID(projects[0])
}
func MigrateProjectCrashCutoffs() error {
	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		const marker = "project_crash_cutoffs_v1"
		var count int64
		if e := tx.Model(&SyncState{}).Where("name = ?", marker).Count(&count).Error; e != nil {
			return e
		}
		if count > 0 {
			return nil
		}
		var old EventSetting
		if e := tx.Where("`key` = ?", oldestCrashBuildSetting).First(&old).Error; e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		var projects []Project
		if e := tx.Find(&projects).Error; e != nil {
			return e
		}
		for _, p := range projects {
			s := EventSetting{Key: projectCrashSettingKey(p.ID), Value: old.Value}
			if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&s).Error; e != nil {
				return e
			}
		}
		return tx.Create(&SyncState{Name: marker, LastSuccessAt: time.Now().UTC()}).Error
	})
}
