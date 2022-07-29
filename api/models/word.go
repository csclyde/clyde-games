package models

import (
	"gorm.io/gorm"
)

const (
	Unknown  int = 0
	Germanic int = 1
	French   int = 2
	Latin    int = 3
	Greek    int = 4
	Other    int = 5
)

type Word struct {
	gorm.Model
	Text   string `gorm:"type:tinytext"`
	Origin int    `gorm:"default:0"`
}

func SelectWords(from []string) ([]Word, error) {
	db, err := getDB("etymology")
	if err != nil {
		return nil, err
	}

	var allWords []Word
	result := db.Where("Text IN ?", from).Find(&allWords)

	if result.Error != nil {
		return nil, result.Error
	}

	return allWords, nil
}

func SelectUnknownWords() ([]Word, error) {
	db, err := getDB("etymology")
	if err != nil {
		return nil, err
	}

	var unknownWords []Word
	result := db.Where("Origin = ?", 0).Find(&unknownWords)

	if result.Error != nil {
		return nil, result.Error
	}

	return unknownWords, nil
}

func AddWord(word Word) (*Word, error) {
	db, err := getDB("etymology")
	if err != nil {
		return nil, err
	}

	result := db.Create(&word)
	if result.Error != nil {
		return nil, result.Error
	}

	return &word, nil
}

func AddWords(words []Word) ([]Word, error) {
	db, err := getDB("etymology")
	if err != nil {
		return nil, err
	}

	result := db.CreateInBatches(&words, 100)
	if result.Error != nil {
		return nil, result.Error
	}

	return words, nil
}

func UpdateWord(word Word) (*Word, error) {
	db, err := getDB("etymology")
	if err != nil {
		return nil, err
	}

	result := db.Save(&word)
	if result.Error != nil {
		return nil, result.Error
	}

	return &word, nil
}
