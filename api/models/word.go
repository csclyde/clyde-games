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

func SelectAllWords() ([]Word, error) {
	var allWords []Word
	result := EtymologyDB.Find(&allWords)
	if result.Error != nil {
		return nil, result.Error
	}

	return allWords, nil
}

func SelectWords(from []string) ([]Word, error) {
	var allWords []Word
	result := EtymologyDB.Where("Text IN ?", from).Find(&allWords)

	if result.Error != nil {
		return nil, result.Error
	}

	return allWords, nil
}

func SelectUnknownWords() ([]Word, error) {
	var unknownWords []Word
	result := EtymologyDB.Where("Origin = ?", 0).Find(&unknownWords)

	if result.Error != nil {
		return nil, result.Error
	}

	return unknownWords, nil
}

func AddWords(words []Word) ([]Word, error) {
	result := EtymologyDB.CreateInBatches(&words, 100)
	if result.Error != nil {
		return nil, result.Error
	}

	return words, nil
}

func UpdateWords(words []Word) ([]Word, error) {
	for _, w := range words {
		result := EtymologyDB.Save(&w)
		if result.Error != nil {
			return nil, result.Error
		}
	}

	return words, nil
}
