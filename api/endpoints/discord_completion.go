package endpoints

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"api.clyde.games/models"
	"gorm.io/gorm"
)

type discordCompletionSender func(context.Context, [3]string) error

func sendDiscordCompletion(ctx context.Context, target [3]string) error {
	var result struct {
		Applied bool `json:"applied"`
	}
	path := "/feedback-completion/" + strings.Join(target[:], "/")
	if err := discordBotCall(ctx, http.MethodPost, path, nil, &result); err != nil {
		return err
	}
	if !result.Applied {
		return fmt.Errorf("Little Clyde did not acknowledge the completion reaction")
	}
	return nil
}

func StartDiscordCompletionWorker() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			if err := deliverDiscordCompletions(models.AnalyticsDB, time.Now().UTC(), sendDiscordCompletion); err != nil {
				fmt.Printf("Discord completion queue unavailable: %v\n", err)
			}
			<-ticker.C
		}
	}()
}

// The feedback row is the outbox. Closing and scheduling share a transaction.
// Leasing uses a conditional write, so overlapping workers cannot claim a row
// together. A stopped worker leaves its lease to expire on its own.
func deliverDiscordCompletions(db *gorm.DB, now time.Time, send discordCompletionSender) error {
	var batch []models.Feedback
	ready := "resolved = ? AND closure_disposition = ? AND source = ? AND completion_reaction_sent_at IS NULL AND completion_reaction_due <= ?"
	query := []interface{}{true, models.FeedbackResolved, "discord", now}
	if err := db.Select("id", "source", "source_id").
		Where(ready, query...).Order("completion_reaction_due, id").Limit(20).Find(&batch).Error; err != nil {
		return err
	}
	for _, row := range batch {
		target, ok := row.DiscordCompletionTarget()
		if !ok {
			continue
		}
		lease := db.Model(&models.Feedback{}).Where("id = ?", row.ID).Where(ready, query...).
			UpdateColumn("completion_reaction_due", time.Now().UTC().Add(5*time.Minute))
		if lease.Error != nil {
			return lease.Error
		}
		if lease.RowsAffected == 0 {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := send(ctx, target)
		cancel()
		if err != nil {
			fmt.Printf("Discord completion feedback %d pending retry: %v\n", row.ID, err)
			continue
		}
		if err := db.Model(&models.Feedback{}).Where("id = ?", row.ID).
			Updates(map[string]interface{}{"completion_reaction_due": nil, "completion_reaction_sent_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
	}
	return nil
}
