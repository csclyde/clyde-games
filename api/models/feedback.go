package models

import (
	"gorm.io/gorm"
)

type Feedback struct {
	gorm.Model
	PID              string `gorm:"type:tinytext"`
	Project          string `gorm:"type:tinytext"`
	Message          string `gorm:"type:text"`
	Translated       string `gorm:"type:text"`
	Rating           uint8
	Env              string `gorm:"type:tinytext"`
	Category         string `gorm:"type:tinytext"`
	Platform         string `gorm:"type:tinytext"`
	Build            string `gorm:"type:tinytext"`
	Commit           string `gorm:"type:tinytext"`
	Resolved         bool   `gorm:"type:boolean;default:false"`
	SavegameID       uint   `gorm:"-"`
	SavegameFilename string `gorm:"-"`
}

func UpdateFeedbackTranslation(id string, translated string) (*Feedback, error) {
	var feedback Feedback
	result := AnalyticsDB.Where("id = ?", id).First(&feedback)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}

	feedback.Translated = translated
	if err := AnalyticsDB.Model(&feedback).Update("translated", translated).Error; err != nil {
		return nil, err
	}

	return &feedback, nil
}

func SelectAllFeedback() ([]Feedback, error) {
	var allFeedback []Feedback
	result := AnalyticsDB.Where("resolved != true").Order("created_at desc").Find(&allFeedback)

	if result.Error != nil {
		return nil, result.Error
	}

	if err := attachSavegameMetadata(allFeedback); err != nil {
		return nil, err
	}

	return allFeedback, nil
}

func AddFeedback(fb Feedback) (*Feedback, error) {
	fb.Project = NormalizeProjectName(fb.Project)
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := ensureProject(tx, fb.Project); err != nil {
			return err
		}
		return tx.Create(&fb).Error
	})
	if err != nil {
		return nil, err
	}

	return &fb, nil
}

func FeedbackMessageContains(text string) (bool, error) {
	var count int64
	result := AnalyticsDB.Model(&Feedback{}).Where("message LIKE ?", "%"+text+"%").Count(&count)
	if result.Error != nil {
		return false, result.Error
	}

	return count > 0, nil
}

func SelectFeedback(id string) (*Feedback, error) {
	var feedback Feedback
	result := AnalyticsDB.Where("id = ?", id).First(&feedback)

	if result.Error != nil {
		return nil, result.Error
	}

	return &feedback, nil
}

func ResolveFeedback(id string) (*Feedback, error) {
	var existingFeedback Feedback
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ?", id).First(&existingFeedback)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				return nil
			}
			return result.Error
		}

		existingFeedback.Resolved = true
		if err := tx.Save(&existingFeedback).Error; err != nil {
			return err
		}

		return tx.Where("feedback_id = ?", existingFeedback.ID).Delete(&Savegame{}).Error
	})
	if err != nil {
		return nil, err
	}
	if existingFeedback.ID == 0 {
		return nil, nil
	}

	return &existingFeedback, nil
}

func attachSavegameMetadata(feedback []Feedback) error {
	if len(feedback) == 0 {
		return nil
	}

	feedbackByID := map[uint]*Feedback{}
	feedbackIDs := make([]uint, 0, len(feedback))
	for index := range feedback {
		feedbackByID[feedback[index].ID] = &feedback[index]
		feedbackIDs = append(feedbackIDs, feedback[index].ID)
	}

	var savegames []Savegame
	result := AnalyticsDB.
		Select("id, feedback_id, filename").
		Where("feedback_id IN ?", feedbackIDs).
		Order("created_at desc").
		Find(&savegames)
	if result.Error != nil {
		return result.Error
	}

	for _, savegame := range savegames {
		if feedbackItem, ok := feedbackByID[savegame.FeedbackID]; ok && feedbackItem.SavegameID == 0 {
			feedbackItem.SavegameID = savegame.ID
			feedbackItem.SavegameFilename = savegame.Filename
		}
	}

	return nil
}
