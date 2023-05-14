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

	err = db.AutoMigrate(&Crash{}, &Event{}, &Feedback{}, &Word{})
	if err != nil {
		return nil, err
	}

	return db, nil
}
