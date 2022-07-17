package models

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Feedback struct {
	gorm.Model
	Message  string `gorm:"default:''"`
	Rating   int    `gorm:"default:0"`
	Env      string `gorm:"default:'dev'"`
	Category string `gorm:"default:'general"`
	Platform string `gorm:"default:'unknown'"`
	FPS      int    `gorm:"default:0"`
}

func SelectAllFeedback() ([]Feedback, error) {
	dbs := "root:root@tcp(127.0.0.1:3306)/analytics?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dbs), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&Feedback{})

	var allFeedback []Feedback
	result := db.Order("created_at desc").Find(&allFeedback)

	if result.Error != nil {
		return nil, result.Error
	}

	return allFeedback, nil
}

func AddFeedback(fb Feedback) (*Feedback, error) {
	dbs := "root:root@tcp(127.0.0.1:3306)/analytics?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dbs), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&Feedback{})

	result := db.Create(&fb)

	if result.Error != nil {
		return nil, result.Error
	}

	return &fb, nil
}
