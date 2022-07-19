package models

import (
	"gorm.io/datatypes"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Event struct {
	gorm.Model
	PID     string `gorm:"type:tinytext"`
	Env     string `gorm:"type:tinytext"`
	Project string `gorm:"type:tinytext"`
	Type    string `gorm:"not null;type:tinytext"`
	Save    datatypes.JSON
}

func SelectAllEvents() ([]Event, error) {
	dbs := "root:root@tcp(127.0.0.1:3306)/analytics?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dbs), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&Event{})
	if err != nil {
		return nil, err
	}

	var allEvents []Event
	result := db.Order("created_at desc").Find(&allEvents)

	if result.Error != nil {
		return nil, result.Error
	}

	return allEvents, nil
}

func AddEvent(event Event) (*Event, error) {
	dbs := "root:root@tcp(127.0.0.1:3306)/analytics?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dbs), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&Event{})

	result := db.Create(&event)

	if result.Error != nil {
		return nil, result.Error
	}

	return &event, nil
}
