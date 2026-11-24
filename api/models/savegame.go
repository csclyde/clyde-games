package models

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrFeedbackSaveAssociation = errors.New("invalid feedback save association")

type Savegame struct {
	gorm.Model
	FeedbackID  uint   `gorm:"index"`
	PID         string `gorm:"type:tinytext"`
	SID         string `gorm:"type:tinytext"`
	Project     string `gorm:"type:tinytext"`
	Env         string `gorm:"type:tinytext"`
	Category    string `gorm:"type:tinytext"`
	Platform    string `gorm:"type:tinytext"`
	Build       string `gorm:"type:tinytext"`
	Commit      string `gorm:"type:tinytext"`
	Reason      string `gorm:"type:tinytext"`
	Filename    string `gorm:"type:tinytext"`
	ContentType string `gorm:"type:tinytext"`
	Hash        string `gorm:"type:char(64);index"`
	Size        uint32 `gorm:"type:int unsigned"`
	Data        []byte `gorm:"type:mediumblob;not null"`
}

type SavegameBuild struct {
	Build   string
	Project string
	Count   int64
}

func AddSavegame(savegame Savegame) (*Savegame, error) {
	savegame.Project = NormalizeProjectName(savegame.Project)
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := ensureProject(tx, savegame.Project); err != nil {
			return err
		}
		if savegame.FeedbackID != 0 {
			if err := validateFeedbackSaveAssociation(tx, savegame); err != nil {
				return err
			}
		} else if err := rejectBlockedUser(tx, savegame.PID); err != nil {
			return err
		}
		if err := rejectBuildBeforeOldest(tx, savegame.Build, "savegame", savegame.Project); err != nil {
			return err
		}

		return tx.Create(&savegame).Error
	})
	if err != nil {
		return nil, err
	}

	return &savegame, nil
}

// A feedback ID alone (or a caller-supplied reason) cannot bypass a player
// block. Validate the existing, open in-game report and its exact installation
// and project before storing any attachment. Lock against concurrent manual
// resolution so a late attachment cannot survive the report's cleanup.
func validateFeedbackSaveAssociation(tx *gorm.DB, savegame Savegame) error {
	if strings.TrimSpace(savegame.PID) == "" || strings.EqualFold(strings.TrimSpace(savegame.PID), "unknown") || savegame.Project == "" {
		return ErrFeedbackSaveAssociation
	}
	var feedback Feedback
	if err := tx.Select("id", "p_id", "project", "source", "resolved").
		Clauses(clause.Locking{Strength: "UPDATE"}).First(&feedback, savegame.FeedbackID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFeedbackSaveAssociation
		}
		return err
	}
	if feedback.PID != savegame.PID || NormalizeProjectName(feedback.Project) != savegame.Project || feedback.Source != "" || feedback.Resolved {
		return ErrFeedbackSaveAssociation
	}
	return nil
}

func SelectAllSavegames() ([]Savegame, error) {
	var savegames []Savegame
	result := AnalyticsDB.
		Omit("Data").
		Where("feedback_id = ?", 0).
		Order("created_at desc").
		Find(&savegames)
	if result.Error != nil {
		return nil, result.Error
	}

	return savegames, nil
}

func SelectAllSavegamesWithData(project string) ([]Savegame, error) {
	var savegames []Savegame
	query := AnalyticsDB.Where("feedback_id = ?", 0)
	if project != "" {
		query = query.Where("LOWER(project) = LOWER(?)", project)
	}
	result := query.Order("created_at desc").Find(&savegames)
	if result.Error != nil {
		return nil, result.Error
	}

	return savegames, nil
}

func SelectSavegameBuilds() ([]SavegameBuild, error) {
	var builds []SavegameBuild
	result := AnalyticsDB.Model(&Savegame{}).
		Select("build, project, count(*) as count").
		Where("build <> ''").
		Group("build, project").
		Order("build desc, project asc").
		Find(&builds)

	if result.Error != nil {
		return nil, result.Error
	}

	return builds, nil
}

func SelectSavegame(id string) (*Savegame, error) {
	var savegame Savegame
	result := AnalyticsDB.Where("id = ?", id).First(&savegame)
	if result.Error != nil {
		return nil, result.Error
	}

	return &savegame, nil
}

func DeleteSavegame(id string) error {
	return AnalyticsDB.Delete(&Savegame{}, id).Error
}

func DeleteSavegamesByBuild(build string, project string) (int64, error) {
	query := AnalyticsDB.Unscoped().Where("build = ?", build)
	if project != "" {
		query = query.Where("project = ?", project)
	}

	result := query.Delete(&Savegame{})
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}
