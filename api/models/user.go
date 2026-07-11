package models

import "fmt"

// UserHistory contains every analytics record associated with a player ID.
// Resolved feedback and crashes are intentionally included: this is a history,
// rather than another view of the open admin queues.
type UserHistory struct {
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
