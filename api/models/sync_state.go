package models

import (
	"time"

	"gorm.io/gorm"
)

type SyncState struct {
	gorm.Model
	Name          string `gorm:"type:varchar(128);uniqueIndex"`
	LastSuccessAt time.Time
}

func GetSyncState(name string) (*SyncState, error) {
	var state SyncState
	result := AnalyticsDB.Where("name = ?", name).First(&state)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}

		return nil, result.Error
	}

	return &state, nil
}

func SaveSyncSuccess(name string, syncedAt time.Time) (*SyncState, error) {
	state, err := GetSyncState(name)
	if err != nil {
		return nil, err
	}

	if state == nil {
		state = &SyncState{Name: name}
	}

	state.LastSuccessAt = syncedAt
	result := AnalyticsDB.Save(state)
	if result.Error != nil {
		return nil, result.Error
	}

	return state, nil
}
