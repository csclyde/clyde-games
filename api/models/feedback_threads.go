package models

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/url"
	"strings"
	"time"
)

const steamDiscussionSource = "steam_discussions"

func steamFeedbackLinks(f Feedback) (thread, entry string) {
	raw := f.SourceURL
	if raw == "" {
		lines := strings.Split(strings.TrimSpace(f.Message), "\n")
		if len(lines) < 2 || !strings.HasPrefix(lines[0], "Steam discussion: ") {
			return "", ""
		}
		raw = strings.TrimSpace(lines[len(lines)-1])
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "steamcommunity.com" {
		return "", ""
	}
	path := strings.TrimRight(u.Path, "/")
	if !strings.Contains(path, "/discussions/") && !strings.Contains(path, "/eventcomments/") && !strings.Contains(path, "/announcements/detail/") {
		return "", ""
	}
	thread = "https://steamcommunity.com" + path + "/"
	if u.Fragment == "" {
		return thread, thread
	}
	if !strings.HasPrefix(u.Fragment, "c") || len(u.Fragment) < 2 {
		return "", ""
	}
	for _, c := range u.Fragment[1:] {
		if c < '0' || c > '9' {
			return "", ""
		}
	}
	return thread, thread + "#" + u.Fragment
}

func ImportSteamFeedbackThread(root Feedback, replies []Feedback, includeRoot bool, prior ...Feedback) (imported, skipped int, err error) {
	root.Project = NormalizeProjectID(root.Project)
	thread, entry := steamFeedbackLinks(root)
	if thread == "" || entry != thread {
		return 0, 0, nil
	}
	root.Resolved = !includeRoot
	err = AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		if err := lockFeedbackProject(tx, root.Project); err != nil {
			return err
		}
		added, err := addSourceFeedbackTx(tx, root, steamDiscussionSource, thread, thread)
		if err != nil {
			return err
		}
		if includeRoot {
			if added {
				imported++
			} else {
				skipped++
			}
		}
		var parent Feedback
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_key = ?", sourceFeedbackKey(root.Project, steamDiscussionSource, thread)).Take(&parent).Error; err != nil {
			return err
		}

		if parent.DeletedAt.Valid {
			skipped += len(replies)
			return nil
		}
		for _, candidate := range append([]Feedback{root}, prior...) {
			topic, entry := steamFeedbackLinks(candidate)
			if topic == thread {
				if err := updateSteamMarkdown(tx, root.Project, entry, candidate.Markdown); err != nil {
					return err
				}
			}
		}
		for _, reply := range replies {
			topic, entry := steamFeedbackLinks(reply)
			if topic != thread || entry == thread {
				continue
			}
			reply.Project = root.Project
			reply.ParentFeedbackID = &parent.ID
			added, err := addSourceFeedbackTx(tx, reply, steamDiscussionSource, entry, entry)
			if err != nil {
				return err
			}
			if !added {
				if err := updateSteamMarkdown(tx, root.Project, entry, reply.Markdown); err != nil {
					return err
				}
				// Older replies can have a source ID but not yet have a parent. Linking
				// is not new activity and must not reopen an already resolved topic.
				if err := tx.Model(&Feedback{}).Where("source_key = ? AND parent_feedback_id IS NULL", sourceFeedbackKey(root.Project, steamDiscussionSource, entry)).UpdateColumn("parent_feedback_id", parent.ID).Error; err != nil {
					return err
				}
				skipped++
				continue
			}
			imported++
			activity := reply.CreatedAt
			if parent.ThreadActivityAt != nil && parent.ThreadActivityAt.After(activity) {
				activity = *parent.ThreadActivityAt
			}
			parent.ThreadActivityAt = &activity
			if err := tx.Model(&parent).Updates(map[string]interface{}{"resolved": false, "thread_activity_at": activity}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return
}

func updateSteamMarkdown(tx *gorm.DB, project, entry, markdown string) error {
	if markdown == "" {
		return nil
	}
	return tx.Model(&Feedback{}).Where("source_key = ?", sourceFeedbackKey(project, steamDiscussionSource, entry)).Where("markdown IS NULL OR markdown = ?", "").UpdateColumn("markdown", markdown).Error
}

func attachFeedbackReplies(items []Feedback) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(items))
	byID := map[uint]*Feedback{}
	for i := range items {
		ids = append(ids, items[i].ID)
		byID[items[i].ID] = &items[i]
	}
	for start := 0; start < len(ids); start += 500 {
		end := start + 500
		if end > len(ids) {
			end = len(ids)
		}
		var replies []Feedback
		if err := AnalyticsDB.Where("parent_feedback_id IN ?", ids[start:end]).Order("created_at ASC, id ASC").Find(&replies).Error; err != nil {
			return err
		}
		for _, f := range replies {
			root := byID[*f.ParentFeedbackID]
			root.Replies = append(root.Replies, f)
		}
	}
	return nil
}

func BackfillSteamFeedbackThreads() error {
	return AnalyticsDB.Transaction(func(tx *gorm.DB) error {
		const marker = "steam_feedback_threads_v1"
		var n int64
		if err := tx.Model(&SyncState{}).Where("name = ?", marker).Count(&n).Error; err != nil {
			return err
		}
		if n != 0 {
			return nil
		}
		var records []Feedback
		if err := tx.Where("platform = ? AND category IN ?", "Steam", []string{"Steam Discussion", "Steam Announcement Discussion"}).Order("id ASC").Find(&records).Error; err != nil {
			return err
		}
		roots := map[string]uint{}
		hadKey := map[string]bool{}
		for _, f := range records {
			if f.SourceKey != nil {
				hadKey[*f.SourceKey] = true
			}
			thread, entry := steamFeedbackLinks(f)
			if thread == "" || thread != entry {
				continue
			}
			k := sourceFeedbackKey(f.Project, steamDiscussionSource, thread)
			if roots[k] == 0 || (f.SourceKey != nil && *f.SourceKey == k) {
				roots[k] = f.ID
			}
		}
		for _, f := range records {
			thread, entry := steamFeedbackLinks(f)
			if thread == "" {
				continue
			}
			parent := roots[sourceFeedbackKey(f.Project, steamDiscussionSource, thread)]
			if parent == 0 {
				continue
			}
			updates := map[string]interface{}{}
			if entry != thread {
				updates["parent_feedback_id"] = parent
			}
			k := sourceFeedbackKey(f.Project, steamDiscussionSource, entry)
			if !hadKey[k] && (f.SourceKey == nil || f.Source == steamDiscussionSource) && (entry != thread || f.ID == parent) {
				updates["source_key"] = k
				updates["source"] = steamDiscussionSource
				updates["source_id"] = entry
				updates["source_url"] = entry
				hadKey[k] = true
			}
			if len(updates) > 0 {
				if err := tx.Model(&f).UpdateColumns(updates).Error; err != nil {
					return err
				}
			}
		}
		return tx.Create(&SyncState{Name: marker, LastSuccessAt: time.Now().UTC()}).Error
	})
}
