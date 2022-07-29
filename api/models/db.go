package models

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func getDB(name string) (*gorm.DB, error) {
	dbs := "root:root@tcp(127.0.0.1:3306)/" + name + "?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dbs), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&Word{})
	if err != nil {
		return nil, err
	}

	return db, nil
}
