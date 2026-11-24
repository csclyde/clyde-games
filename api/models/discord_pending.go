package models

import "time"

// OriginalSnapshot and append-only revisions retain source evidence.
// Snapshot is the latest context; disposition is separate from resolution.
type DiscordPendingMessage struct {
	ScopeID          string    `gorm:"type:varchar(32);index" json:"scope_id"`
	Kind             string    `gorm:"type:varchar(32)" json:"kind"`
	OriginalSnapshot string    `gorm:"type:longtext" json:"-"`
	ContextHash      string    `gorm:"type:char(64)" json:"context_hash"`
	Revision         uint      `json:"revision"`
	ReviewedRevision uint      `json:"reviewed_revision"`
	NeedsReview      bool      `json:"needs_review"`
	MarkSuppressed   bool      `json:"mark_suppressed"`
	ManualTriggerID  string    `gorm:"type:varchar(32)" json:"manual_trigger_id,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
	ID               string    `gorm:"primaryKey;type:varchar(32)" json:"id"`
	GuildID          string    `gorm:"type:varchar(32)" json:"guild_id"`
	ChannelID        string    `gorm:"type:varchar(32)" json:"channel_id"`
	Project          string    `gorm:"type:varchar(64)" json:"project"`
	CreatedAt        time.Time `json:"captured_at"`
	SourceCreatedAt  time.Time `json:"source_created_at"`
	Snapshot         string    `gorm:"type:longtext" json:"-"`
	Disposition      string    `gorm:"type:varchar(16);index" json:"disposition"`
	FeedbackID       uint      `json:"feedback_id"`
	BugMarked        bool      `json:"bug_marked"`
	CheckMarked      bool      `json:"check_marked"`
	LastMarkError    string    `gorm:"type:text" json:"last_mark_error,omitempty"`
	Note             string    `gorm:"type:text" json:"note,omitempty"`
}
