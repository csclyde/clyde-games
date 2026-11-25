package models

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrFeedbackBlocked = errors.New("feedback submissions are blocked")
var ErrInvalidFeedbackPlayerID = errors.New("player ID must contain 1 to 64 characters")

// FeedbackBlock limits new in-game feedback only. It is separate from the
// metrics block, which can delete analytics records and historical saves.
type FeedbackBlock struct {
	PID       string    `gorm:"primaryKey;type:varchar(64)"`
	CreatedAt time.Time `gorm:"not null"`
}

func isFeedbackBlocked(db *gorm.DB, pid string) (bool, error) {
	pid = strings.TrimSpace(pid)
	if pid == "" {
		return false, nil
	}
	var count int64
	err := db.Model(&FeedbackBlock{}).Where("p_id = ?", pid).Count(&count).Error
	return count > 0, err
}

func SetFeedbackBlock(pid string, blocked bool) error {
	pid = strings.TrimSpace(pid)
	if pid == "" || utf8.RuneCountInString(pid) > 64 {
		return ErrInvalidFeedbackPlayerID
	}
	if blocked {
		return AnalyticsDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&FeedbackBlock{PID: pid}).Error
	}
	return AnalyticsDB.Where("p_id = ?", pid).Delete(&FeedbackBlock{}).Error
}

func SelectFeedbackBlocks() ([]FeedbackBlock, error) {
	blocks := []FeedbackBlock{}
	err := AnalyticsDB.Order("created_at DESC, p_id ASC").Find(&blocks).Error
	return blocks, err
}
