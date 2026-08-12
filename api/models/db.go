package models

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var AnalyticsDB *gorm.DB
var EtymologyDB *gorm.DB

func GetDB(name string) (*gorm.DB, error) {
	dbs := "root:root@tcp(127.0.0.1:3306)/" + name + "?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dbs), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func MigrateAnalyticsDB() error {
	if err := AnalyticsDB.AutoMigrate(&BlockedUser{}, &Project{}, &Crash{}, &Event{}, &EventItem{}, &EventSetting{}, &Feedback{}, &Savegame{}, &SyncState{}, &PackratVersion{}); err != nil {
		return err
	}
	if err := BackfillProjects(); err != nil {
		return err
	}
	if err := SeedProjectSources(); err != nil {
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
