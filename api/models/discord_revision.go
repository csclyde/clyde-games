package models

import "time"

type DiscordSnapshotRevision struct {
	ID        string    `gorm:"primaryKey;type:varchar(128)" json:"id"`
	MessageID string    `gorm:"type:varchar(32);uniqueIndex:discord_revision" json:"message_id"`
	Revision  uint      `gorm:"uniqueIndex:discord_revision" json:"revision"`
	Snapshot  string    `gorm:"type:longtext" json:"snapshot"`
	CreatedAt time.Time `json:"captured_at"`
}
