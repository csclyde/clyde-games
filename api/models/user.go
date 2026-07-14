package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrUserBlocked = errors.New("user is blocked")

type BlockedUser struct {
	PID       string    `gorm:"primaryKey;type:varchar(64)"`
	CreatedAt time.Time `gorm:"not null"`
}

// UserHistory contains every analytics record associated with a player ID.
// Resolved feedback and crashes are intentionally included: this is a history,
// rather than another view of the open admin queues.
type UserHistory struct {
	Blocked   bool
	Feedback  []Feedback
	Crashes   []Crash
	Savegames []Savegame
	Events    []Event
}

func SelectUserHistory(pid string) (*UserHistory, error) {
	history := &UserHistory{
		Feedback:  []Feedback{},
		Crashes:   []Crash{},
		Savegames: []Savegame{},
		Events:    []Event{},
	}
	var blockedCount int64
	if err := AnalyticsDB.Model(&BlockedUser{}).Where("p_id = ?", pid).Count(&blockedCount).Error; err != nil {
		return nil, fmt.Errorf("check blocked user: %w", err)
	}
	history.Blocked = blockedCount > 0

	if err := AnalyticsDB.Model(&Feedback{}).Where(&Feedback{PID: pid}).Order("created_at asc").Find(&history.Feedback).Error; err != nil {
		return nil, fmt.Errorf("select user feedback: %w", err)
	}
	if err := attachSavegameMetadata(history.Feedback); err != nil {
		return nil, fmt.Errorf("attach user savegame metadata: %w", err)
	}
	if err := AnalyticsDB.Model(&Crash{}).Where(&Crash{PID: pid}).Order("created_at asc").Find(&history.Crashes).Error; err != nil {
		return nil, fmt.Errorf("select user crashes: %w", err)
	}
	if err := AnalyticsDB.Model(&Savegame{}).Omit("Data").Where(&Savegame{PID: pid}).Order("created_at asc").Find(&history.Savegames).Error; err != nil {
		return nil, fmt.Errorf("select user savegames: %w", err)
	}
	if err := AnalyticsDB.Model(&Event{}).Preload("Items").Where(&Event{PID: pid}).Order("created_at asc").Find(&history.Events).Error; err != nil {
		return nil, fmt.Errorf("select user events: %w", err)
	}

	return history, nil
}

func rejectBlockedUser(tx *gorm.DB, pid string) error {
	var count int64
	if err := tx.Model(&BlockedUser{}).Where("p_id = ?", pid).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrUserBlocked
	}
	return nil
}

func BlockUser(pid string) error {
	pid = strings.TrimSpace(pid)
	if pid == "" {
		return errors.New("user id is required")
	}
	if len(pid) > 64 {
		return errors.New("user id must be 64 characters or fewer")
	}

	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.FirstOrCreate(&BlockedUser{PID: pid}).Error; err != nil {
			return err
		}

		eventIDs := tx.Unscoped().Model(&Event{}).Select("id").Where("p_id = ?", pid)
		if err := tx.Unscoped().Where("event_id IN (?)", eventIDs).Delete(&EventItem{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("p_id = ?", pid).Delete(&Event{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Where("p_id = ?", pid).Delete(&Savegame{}).Error
	})
}

func SelectBlockedUsers() ([]BlockedUser, error) {
	users := []BlockedUser{}
	if err := AnalyticsDB.Order("created_at desc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func UnblockUser(pid string) error {
	return AnalyticsDB.Where("p_id = ?", pid).Delete(&BlockedUser{}).Error
}
