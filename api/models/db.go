package models

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var AnalyticsDB *gorm.DB
var EtymologyDB *gorm.DB

func GetDB(name string) (*gorm.DB, error) {
	if !validSchemaName(name) {
		return nil, fmt.Errorf("invalid database name")
	}
	dbs := "root:root@tcp(127.0.0.1:3306)/" + name + "?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dbs), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func MigrateAnalyticsDB() error {
	if len(gameMetricStores) == 0 {
		if err := AnalyticsDB.AutoMigrate(&Event{}, &EventItem{}); err != nil {
			return err
		}
	}
	if err := AnalyticsDB.AutoMigrate(&BlockedUser{}, &FeedbackBlock{}, &Project{}, &Crash{}, &CrashGroup{}, &EventSetting{}, &Feedback{}, &DiscordPendingMessage{}, &DiscordSnapshotRevision{}, &DiscordSyncSource{}, &Savegame{}, &SyncState{}, &PackratVersion{}); err != nil {
		return err
	}
	if err := MigrateCrashGroups(); err != nil {
		return err
	}
	if err := BackfillProjects(); err != nil {
		return err
	}
	if err := SeedPathProdigyProject(); err != nil {
		return err
	}
	if err := SeedProjectSources(); err != nil {
		return err
	}

	if err := MigrateFeedbackSyncConfiguration(); err != nil {
		return err
	}
	if err := MigrateProjectCrashCutoffs(); err != nil {
		return err
	}
	if err := MigrateProjectBuildCutoffs(); err != nil {
		return err
	}
	if err := BackfillSteamFeedbackThreads(); err != nil {
		return err
	}
	return CreateSavegameRetentionEvent()
}

func MigrateEtymologyDB() error {
	return EtymologyDB.AutoMigrate(&Word{})
}

func CreateSavegameRetentionEvent() error {
	return AnalyticsDB.Exec(`
		CREATE EVENT IF NOT EXISTS delete_old_savegames
		ON SCHEDULE EVERY 1 DAY
		DO
			DELETE FROM savegames
			WHERE created_at < NOW() - INTERVAL 1 MONTH
	`).Error
}
