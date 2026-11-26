package models

import (
	"time"

	"gorm.io/gorm"
)

type MarketingBrief struct {
	Name       string `json:"name" gorm:"type:varchar(160);not null"`
	AssetKind  string `json:"asset_kind" gorm:"type:varchar(24);not null"`
	Asset      string `json:"asset" gorm:"type:varchar(1000);not null"`
	Brief      string `json:"brief" gorm:"type:text;not null"`
	Hook       string `json:"hook" gorm:"type:text;not null"`
	WhyNow     string `json:"why_now" gorm:"type:text;not null"`
	CTA        string `json:"cta" gorm:"type:text;not null"`
	Goal       string `json:"goal" gorm:"type:varchar(24);not null"`
	GoalTarget uint   `json:"goal_target" gorm:"not null"`
}

type MarketingBeat struct {
	ID           uint   `gorm:"primaryKey"`
	Project      string `gorm:"type:varchar(64);index;not null"`
	Version      uint   `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ArchivedAt   *time.Time
	CallToAction string `gorm:"type:varchar(16);not null;default:''"`
	MarketingBrief
}

type MarketingPlacementDraft struct {
	Platform     string `json:"platform" gorm:"type:varchar(80);not null"`
	Community    string `json:"community" gorm:"type:varchar(160);not null"`
	AudienceFit  string `json:"audience_fit" gorm:"type:text;not null"`
	Timing       string `json:"timing" gorm:"type:text;not null"`
	Title        string `json:"title" gorm:"type:varchar(255);not null"`
	Copy         string `json:"copy" gorm:"type:text;not null"`
	AssetNotes   string `json:"asset_notes" gorm:"type:text;not null"`
	RuleEvidence string `json:"rule_evidence" gorm:"type:text;not null"`
	RulesURL     string `json:"rules_url" gorm:"type:varchar(1000);not null"`
	RulesChecked string `json:"rules_checked" gorm:"type:char(10);not null"`
	Decision     string `json:"decision" gorm:"type:varchar(16);not null"`
}

type MarketingPlacement struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	BeatID        uint       `json:"beat_id" gorm:"index;not null"`
	TargetID      *uint      `json:"target_id"`
	Project       string     `json:"-" gorm:"type:varchar(64);index;not null"`
	Version       uint       `json:"version" gorm:"not null"`
	Rank          uint       `json:"rank" gorm:"not null"`
	UTM           string     `json:"-" gorm:"type:char(32);uniqueIndex;not null"`
	AI            bool       `json:"ai" gorm:"not null"`
	SourceCited   bool       `json:"source_cited" gorm:"not null"`
	PublishedAt   *time.Time `json:"published_at"`
	PublishedURL  string     `json:"published_url" gorm:"type:varchar(1000);not null"`
	PublishedNote string     `json:"published_note" gorm:"type:text;not null"`
	ArchivedAt    *time.Time `json:"archived_at"`
	MarketingPlacementDraft
}

// A report is a complete snapshot of one inclusive range. The import receipts
// make retries of an older file harmless even after that range was refreshed.
type MarketingReport struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Project    string    `json:"-" gorm:"type:varchar(64);uniqueIndex:marketing_report_window;not null"`
	StartDate  string    `json:"start_date" gorm:"type:char(10);uniqueIndex:marketing_report_window;not null"`
	EndDate    string    `json:"end_date" gorm:"type:char(10);uniqueIndex:marketing_report_window;not null"`
	ImportedAt time.Time `json:"imported_at"`
}

type MarketingImport struct {
	ID         uint   `gorm:"primaryKey"`
	ReportID   uint   `gorm:"uniqueIndex:marketing_import_file;not null"`
	FileSHA256 string `gorm:"type:char(64);uniqueIndex:marketing_import_file;not null"`
	CreatedAt  time.Time
}

type MarketingCounts struct {
	Total       *uint64 `json:"total"`
	Trusted     *uint64 `json:"trusted"`
	Tracked     *uint64 `json:"tracked"`
	Wishlists   *uint64 `json:"wishlists"`
	Purchases   *uint64 `json:"purchases"`
	Activations *uint64 `json:"activations"`
}

type MarketingResult struct {
	ReportID    uint `gorm:"primaryKey;autoIncrement:false"`
	PlacementID uint `gorm:"primaryKey;autoIncrement:false;index"`
	MarketingCounts
}

type MarketingTarget struct {
	ID          uint   `gorm:"primaryKey"`
	Project     string `gorm:"type:varchar(64);uniqueIndex:marketing_target_identity;not null"`
	URLKey      string `gorm:"type:char(64);uniqueIndex:marketing_target_identity;not null"`
	Name        string `gorm:"type:varchar(160);not null"`
	IsCreator   bool   `gorm:"not null;default:false"`
	URL         string `gorm:"type:varchar(1000);not null"`
	Version     uint   `gorm:"not null"`
	ResearchURL string `gorm:"type:varchar(1000);not null"`
	Research    string `gorm:"type:mediumtext;not null"`
	CheckedAt   *time.Time
	ArchivedAt  *time.Time
}

type MarketingCreatorEvidence struct {
	TargetID  uint   `gorm:"primaryKey;autoIncrement:false"`
	Version   uint   `gorm:"not null"`
	Coverage  string `gorm:"type:mediumtext;not null"`
	Reply     string `gorm:"type:text;not null"`
	UpdatedAt time.Time
}

// Creator contacts are read only on the authenticated target detail route.
type MarketingCreator struct {
	TargetID    uint   `gorm:"primaryKey;autoIncrement:false"`
	Email       string `gorm:"type:varchar(254);not null"`
	Platform    string `gorm:"type:varchar(16);not null"`
	ChannelURLs string `gorm:"type:text;not null"`
	Games       string `gorm:"type:text;not null"`
}

type MarketingBeatTarget struct {
	BeatID   uint `gorm:"primaryKey;autoIncrement:false"`
	TargetID uint `gorm:"primaryKey;autoIncrement:false;index"`
}

func MigrateMarketing(db *gorm.DB) error {
	return db.AutoMigrate(&MarketingBeat{}, &MarketingPlacement{}, &MarketingReport{}, &MarketingImport{},
		&MarketingResult{}, &MarketingTarget{}, &MarketingBeatTarget{}, &MarketingCreator{}, &MarketingCreatorEvidence{})
}
