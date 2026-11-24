package models

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AddSourceFeedback records an external item exactly once per project/source/ID.
// A project lock serializes concurrent collectors, including lazy legacy linking.
// Resolved and soft-deleted items are retained and never reopened by a rerun.
func AddSourceFeedback(fb Feedback, source, externalID, canonicalURL string) (bool, error) {
	fb.Project = NormalizeProjectID(fb.Project)
	var added bool
	err := AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := lockFeedbackProject(tx, fb.Project); err != nil {
			return err
		}
		var err error
		added, err = addSourceFeedbackTx(tx, fb, source, externalID, canonicalURL)
		return err
	})
	return added, err
}

func lockFeedbackProject(tx *gorm.DB, id string) error {
	if id == "" {
		return fmt.Errorf("invalid feedback project")
	}
	if err := ensureProject(tx, id); err != nil {
		return err
	}
	var project Project
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, "id = ?", id).Error
}

func sourceFeedbackKey(project, source, externalID string) string {
	digest := sha256.Sum256([]byte(project + "\x00" + source + "\x00" + externalID))
	return hex.EncodeToString(digest[:])
}

func addSourceFeedbackTx(tx *gorm.DB, fb Feedback, source, externalID, canonicalURL string) (added bool, err error) {
	if source == "" || externalID == "" || canonicalURL == "" || len(externalID) > 255 || len(canonicalURL) > 1024 {
		return false, fmt.Errorf("invalid external feedback identity")
	}
	key := sourceFeedbackKey(fb.Project, source, externalID)
	err = func() error {
		var existing Feedback
		err := tx.Unscoped().Where("source_key = ?", key).Take(&existing).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Legacy rows contain their canonical URL as the final line. Equality on the
		// final line prevents a topic URL matching a reply URL, unlike LIKE %url%.
		var legacy []Feedback
		if err := tx.Unscoped().Where("project = ? AND category = ? AND platform = ? AND source_key IS NULL", fb.Project, fb.Category, fb.Platform).Order("id ASC").Find(&legacy).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		metadata := map[string]interface{}{"source_key": key, "source": source, "source_id": externalID, "source_url": canonicalURL, "ingested_at": now}
		for _, row := range legacy {
			lines := strings.Split(strings.TrimSpace(row.Message), "\n")
			match := strings.TrimSpace(lines[len(lines)-1]) == canonicalURL
			if source == steamDiscussionSource {
				_, legacyEntry := steamFeedbackLinks(row)
				match = match || legacyEntry == canonicalURL
			}
			if source == "discord" {
				for _, line := range lines {
					if strings.TrimSpace(line) == "Discord message: "+canonicalURL {
						match = true
					}
				}
			}
			if match {
				// UpdateColumns preserves historical timestamps, content and resolution.
				return tx.Unscoped().Model(&Feedback{}).Where("id = ?", row.ID).UpdateColumns(metadata).Error
			}
		}
		fb.SourceKey = &key
		fb.Source = source
		fb.SourceID = externalID
		fb.SourceURL = canonicalURL
		fb.IngestedAt = &now
		if err := tx.Create(&fb).Error; err != nil {
			return err
		}
		added = true
		return nil
	}()
	return added, err
}
