package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	FeedbackResolved = "resolved"
	FeedbackIgnored  = "ignored"
)

var ErrFeedbackDisposition = errors.New("invalid feedback disposition")

type Feedback struct {
	gorm.Model
	ParentFeedbackID         *uint      `gorm:"index" json:"-"`
	ThreadActivityAt         *time.Time `json:",omitempty"`
	Replies                  []Feedback `gorm:"-" json:",omitempty"`
	TriageState              string     `gorm:"type:varchar(16)"`
	TriageNote               string     `gorm:"type:text"`
	TriageAt                 *time.Time
	SourceKey                *string `gorm:"type:char(64);uniqueIndex" json:"-"`
	Source                   string  `gorm:"type:varchar(32)"`
	SourceID                 string  `gorm:"type:varchar(255)"`
	SourceURL                string  `gorm:"type:varchar(1024)"`
	IngestedAt               *time.Time
	PID                      string `gorm:"type:tinytext"`
	Project                  string `gorm:"type:tinytext"`
	Message                  string `gorm:"type:text"`
	Markdown                 string `gorm:"type:mediumtext" json:",omitempty"`
	Translated               string `gorm:"type:text"`
	Rating                   uint8
	Env                      string     `gorm:"type:tinytext"`
	Category                 string     `gorm:"type:tinytext"`
	Platform                 string     `gorm:"type:tinytext"`
	Build                    string     `gorm:"type:tinytext"`
	Commit                   string     `gorm:"type:tinytext"`
	Resolved                 bool       `gorm:"type:boolean;default:false"`
	ClosureDisposition       string     `gorm:"type:varchar(16)" json:",omitempty"`
	CompletionReactionDue    *time.Time `gorm:"index" json:"-"`
	CompletionReactionSentAt *time.Time `json:"-"`
	HasImage                 bool       `gorm:"not null;default:false" json:",omitempty"`
	SavegameID               uint       `gorm:"-"`
	DisplayAuthor            string     `gorm:"-" json:",omitempty"`
	SavegameFilename         string     `gorm:"-"`
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
	result := AnalyticsDB.Where("resolved != true AND parent_feedback_id IS NULL").Order("COALESCE(thread_activity_at, created_at) DESC, id DESC").Find(&allFeedback)

	if result.Error != nil {
		return nil, result.Error
	}

	if err := attachSavegameMetadata(allFeedback); err != nil {
		return nil, err
	}

	return allFeedback, attachFeedbackReplies(allFeedback)
}

func AddFeedback(fb Feedback, screenshots ...[]byte) (*Feedback, error) {
	fb.HasImage = false
	var screenshot []byte
	if len(screenshots) > 0 {
		screenshot = screenshots[0]
	}
	if len(screenshot) > 0 {
		fb.Resolved = false
		fb.DeletedAt = gorm.DeletedAt{}
	}
	fb.ParentFeedbackID, fb.ThreadActivityAt, fb.Replies = nil, nil, nil
	fb.Markdown = ""
	fb.DisplayAuthor = ""
	fb.TriageState = ""
	fb.TriageNote = ""
	fb.TriageAt = nil
	fb.ClosureDisposition = ""
	fb.CompletionReactionDue, fb.CompletionReactionSentAt = nil, nil
	fb.SourceKey = nil
	fb.Source = ""
	fb.SourceID = ""
	fb.SourceURL = ""
	fb.IngestedAt = nil
	fb.Project = NormalizeProjectName(fb.Project)
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		blocked, err := isFeedbackBlocked(tx, fb.PID)
		if err != nil {
			return err
		}
		if blocked {
			return ErrFeedbackBlocked
		}
		if err := ensureProject(tx, fb.Project); err != nil {
			return err
		}
		if err := tx.Create(&fb).Error; err != nil {
			return err
		}
		if len(screenshot) > 0 {
			if err := saveFeedbackImage(tx, fb.ID, screenshot); err != nil {
				return err
			}
			fb.HasImage = true
		}
		return nil
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

// ResolveFeedback preserves the legacy non-notifying path used after ticket creation.
func ResolveFeedback(id string) (*Feedback, error) {
	return closeFeedback(id, FeedbackResolved, false)
}

// CloseFeedback is the explicit player-feedback action in the Admin portal.
func CloseFeedback(id, disposition string) (*Feedback, error) {
	return closeFeedback(id, disposition, true)
}

func closeFeedback(id, disposition string, notifyDiscord bool) (*Feedback, error) {
	if disposition != FeedbackResolved && disposition != FeedbackIgnored {
		return nil, ErrFeedbackDisposition
	}
	var existingFeedback Feedback
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&existingFeedback)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				return nil
			}
			return result.Error
		}

		if existingFeedback.Resolved {
			return nil
		}
		existingFeedback.Resolved = true
		existingFeedback.ClosureDisposition = disposition
		updates := map[string]interface{}{"resolved": true, "closure_disposition": disposition}
		if notifyDiscord && disposition == FeedbackResolved {
			if _, ok := existingFeedback.DiscordCompletionTarget(); ok {
				now := time.Now().UTC()
				existingFeedback.CompletionReactionDue = &now
				updates["completion_reaction_due"] = now
			}
		}
		if err := tx.Model(&existingFeedback).Updates(updates).Error; err != nil {
			return err
		}

		if existingFeedback.HasImage {
			if err := tx.Delete(&FeedbackImage{}, existingFeedback.ID).Error; err != nil {
				return err
			}
			if err := tx.Model(&existingFeedback).UpdateColumn("has_image", false).Error; err != nil {
				return err
			}
			existingFeedback.HasImage = false
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

// DiscordCompletionTarget uses trusted source provenance, never the message text
// or a user-supplied URL. All Discord ingestion paths create this tuple.
func (f Feedback) DiscordCompletionTarget() ([3]string, bool) {
	var target [3]string
	parts := strings.Split(f.SourceID, ":")
	if f.Source != "discord" || len(parts) != len(target) {
		return target, false
	}
	for i, id := range parts {
		if len(id) == 0 || len(id) > 22 {
			return target, false
		}
		for _, c := range id {
			if c < '0' || c > '9' {
				return target, false
			}
		}
		target[i] = id
	}
	return target, true
}
