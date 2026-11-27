package models

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Keep binary data out of feedback queries and JSON. The feedback row is
// the lock shared by insertion and closure.
type FeedbackImage struct {
	FeedbackID uint   `gorm:"primaryKey;autoIncrement:false"`
	Data       []byte `gorm:"type:mediumblob;not null" json:"-"`
}

func SelectFeedbackImage(id uint, project string) ([]byte, error) {
	var image FeedbackImage
	err := AnalyticsDB.Model(&FeedbackImage{}).
		Joins("JOIN feedbacks AS f ON f.id = feedback_images.feedback_id").
		Where("f.id = ? AND f.project = ? AND f.deleted_at IS NULL AND f.resolved = false", id, project).
		Take(&image).Error
	return image.Data, err
}

func saveFeedbackImage(tx *gorm.DB, id uint, data []byte) error {
	if err := tx.Create(&FeedbackImage{FeedbackID: id, Data: data}).Error; err != nil {
		return err
	}
	return tx.Model(&Feedback{}).Where("id = ?", id).UpdateColumn("has_image", true).Error
}

// AttachDiscordFeedbackImage is idempotent and cannot revive a closed report
// even if a retry races with Ignore or Resolve.
func AttachDiscordFeedbackImage(id uint, project string, data []byte) error {
	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		var f Feedback
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "has_image").
			Where("id = ? AND project = ? AND source = ? AND resolved = false", id, project, "discord").Take(&f).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil || f.HasImage {
			return err
		}
		return saveFeedbackImage(tx, id, data)
	})
}
