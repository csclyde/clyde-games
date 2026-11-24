package endpoints

import (
	"api.clyde.games/models"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func discordContentHash(raw []byte) (string, error) {
	var source map[string]json.RawMessage
	if e := json.Unmarshal(raw, &source); e != nil {
		return "", e
	}
	evidence := map[string]json.RawMessage{}
	for _, key := range []string{"content", "edited_timestamp", "attachments", "thread_context", "reply_context", "title"} {
		if value, ok := source[key]; ok {
			evidence[key] = value
		}
	}
	// Reaction count changes are intake metadata, not new player evidence.
	if rawContext, ok := evidence["thread_context"]; ok {
		var replies []map[string]json.RawMessage
		if json.Unmarshal(rawContext, &replies) == nil {
			for _, r := range replies {
				delete(r, "reactions")
			}
			evidence["thread_context"], _ = json.Marshal(replies)
		}
	}
	data, e := json.Marshal(evidence)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
func storeDiscordSnapshot(source models.DiscordSyncSource, raw []byte, manual bool) (bool, error) {
	if len(raw) > 256<<10 {
		return false, fmt.Errorf("Discord evidence exceeds per-issue limit")
	}
	var snapshot discordSnapshot
	if e := json.Unmarshal(raw, &snapshot); e != nil {
		return false, e
	}
	resolver := sourceForSnapshot
	if manual {
		resolver = manualSourceForSnapshot
	}
	verified, e := resolver(snapshot)
	if e != nil {
		return false, e
	}
	if verified.ID != source.ID || verified.ProjectID != source.ProjectID {
		return false, fmt.Errorf("source changed during collection")
	}
	hash, e := discordContentHash(raw)
	if e != nil {
		return false, e
	}
	changed := false
	e = models.AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		config := source
		if manual {
			var project models.Project
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", source.ProjectID).Take(&project).Error; e != nil {
				return e
			}
		} else if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND enabled = ?", source.ID, true).Take(&config).Error; e != nil {
			return e
		}
		var current models.DiscordPendingMessage
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&current, "id = ?", snapshot.ID).Error
		fresh := errors.Is(e, gorm.ErrRecordNotFound)
		if e != nil && !fresh {
			return e
		}
		if !fresh && (current.Project != source.ProjectID || current.ScopeID != source.ID) {
			return fmt.Errorf("source identity conflict")
		}
		handled := !manual && (snapshot.Handled || snapshot.ContextOnly || snapshot.Timestamp.Before(config.BaselineAt))
		if fresh {
			kind := "discord_text"
			if source.Kind == "discord_forum" {
				kind = "forum_thread"
			}
			current = models.DiscordPendingMessage{ID: snapshot.ID, GuildID: snapshot.GuildID, ChannelID: snapshot.ChannelID, ScopeID: source.ID, Project: source.ProjectID, Kind: kind, SourceCreatedAt: snapshot.Timestamp, OriginalSnapshot: string(raw), Disposition: "pending"}
			if handled {
				current.Disposition = "handled"
				current.MarkSuppressed = true
			}
		}
		triggerChanged := false
		if manual {
			nextTrigger := snapshot.Trigger.ID
			if nextTrigger == "" {
				nextTrigger = snapshot.ID
			}
			triggerChanged = current.ManualTriggerID != nextTrigger
			current.ManualTriggerID = snapshot.Trigger.ID
			if current.ManualTriggerID == "" {
				current.ManualTriggerID = snapshot.ID
			}
			current.MarkSuppressed = false
			if triggerChanged {
				current.CheckMarked = false
				current.LastMarkError = ""
			}
		}
		if !fresh && current.ContextHash == hash && !triggerChanged {
			if manual {
				return tx.Model(&current).Updates(map[string]interface{}{"manual_trigger_id": current.ManualTriggerID, "mark_suppressed": false}).Error
			}
			return nil
		}
		current.Snapshot = string(raw)
		current.ContextHash = hash
		current.Revision++
		current.UpdatedAt = time.Now().UTC()
		if current.OriginalSnapshot == "" {
			current.OriginalSnapshot = string(raw)
		}
		if current.Disposition == "noise" {
			current.Disposition = "pending"
		}
		current.NeedsReview = !manual && !snapshot.Historical
		if current.Disposition == "handled" && !snapshot.ContextOnly {
			current.NeedsReview = false
		}
		if handled && current.FeedbackID == 0 {
			current.Disposition = "handled"
			current.MarkSuppressed = true
		}
		if e := tx.Save(&current).Error; e != nil {
			return e
		}
		revision := models.DiscordSnapshotRevision{ID: fmt.Sprintf("%s:%d", current.ID, current.Revision), MessageID: current.ID, Revision: current.Revision, Snapshot: string(raw)}
		if e := tx.Create(&revision).Error; e != nil {
			return e
		}
		changed = true
		return nil
	})
	return changed, e
}
