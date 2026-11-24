package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNoteInput = errors.New("invalid report note")
var ErrNoteConflict = errors.New("request_key already used with different content")

// Immutable feedback and legacy crash logs. New crashgrp notes address a
// canonical investigation unit; original occurrence identities/history are retained.
// No gorm.Model soft-delete or update endpoint: corrections are subsequent notes.
type ReportNote struct {
	ID           uint                `json:"id" gorm:"primaryKey"`
	Kind         string              `json:"kind" gorm:"type:varchar(8);uniqueIndex:note_request;index:note_scope,priority:1"`
	ReportID     uint                `json:"report_id" gorm:"uniqueIndex:note_request;index:note_scope,priority:2"`
	RequestKey   string              `json:"-" gorm:"type:varbinary(80);uniqueIndex:note_request"`
	PayloadHash  string              `json:"-" gorm:"type:char(64)"`
	Project      string              `json:"project" gorm:"type:varchar(64);index"`
	CreatedAt    time.Time           `json:"created_at"`
	Actor        string              `json:"actor" gorm:"type:varchar(16)"`
	Note         string              `json:"note" gorm:"type:text"`
	State        string              `json:"state,omitempty" gorm:"type:varchar(24)"`
	ReleaseState string              `json:"release_state,omitempty" gorm:"type:varchar(16)"`
	Evidence     *ReportNoteEvidence `json:"evidence,omitempty" gorm:"foreignKey:NoteID;constraint:OnDelete:RESTRICT"`
}

type ReportNoteEvidence struct {
	ID      uint   `json:"id" gorm:"primaryKey"`
	NoteID  uint   `json:"note_id" gorm:"uniqueIndex"`
	Name    string `json:"name" gorm:"type:varchar(160)"`
	SHA256  string `json:"sha256" gorm:"type:char(64)"`
	Bytes   int    `json:"bytes"`
	Content string `json:"content,omitempty" gorm:"type:mediumtext"`
}

type NoteEvidenceInput struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}
type AppendReportNoteInput struct {
	RequestKey   string             `json:"request_key"`
	Note         string             `json:"note"`
	State        string             `json:"state,omitempty"`
	ReleaseState string             `json:"release_state,omitempty"`
	Evidence     *NoteEvidenceInput `json:"evidence,omitempty"`
}

func MigrateReportNotes(db *gorm.DB) error {
	return db.AutoMigrate(&ReportNote{}, &ReportNoteEvidence{})
}

func ValidateReportNote(in AppendReportNoteInput) error {
	if !utf8.ValidString(in.Note) || strings.TrimSpace(in.Note) == "" || utf8.RuneCountInString(in.Note) > 2000 {
		return ErrNoteInput
	}
	if len(in.RequestKey) < 8 || len(in.RequestKey) > 80 {
		return ErrNoteInput
	}
	for _, r := range in.RequestKey {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.:", r)) {
			return ErrNoteInput
		}
	}
	switch in.State {
	case "", "investigating", "blocked", "needs_confirmation", "ready_for_review", "fixed":
	default:
		return ErrNoteInput
	}
	switch in.ReleaseState {
	case "", "local", "deployed", "released":
	default:
		return ErrNoteInput
	}
	if e := in.Evidence; e != nil {
		if e.Name == "" || len(e.Name) > 160 || strings.ContainsAny(e.Name, "/\\\r\n") || !utf8.ValidString(e.Content) || len(e.Content) == 0 || len(e.Content) > 24<<10 {
			return ErrNoteInput
		}
		h := sha256.Sum256([]byte(e.Content))
		if e.SHA256 != hex.EncodeToString(h[:]) {
			return ErrNoteInput
		}
	}
	return nil
}

type NoteReport struct {
	ID       uint   `json:"id"`
	Project  string `json:"project"`
	Resolved bool   `json:"resolved"`
	Message  string `json:"message"`
}

func NoteReportQuery(db *gorm.DB, kind string) (*gorm.DB, error) {
	table := ""
	switch kind {
	case "feedback":
		table = "feedbacks"
	case "crash":
		table = "crashes"
	default:
		return nil, ErrNoteInput
	}
	return db.Table(table).Where("deleted_at IS NULL"), nil
}
func FindNoteReport(db *gorm.DB, kind string, id uint, project string, lock bool) (NoteReport, error) {
	var row NoteReport
	q, err := NoteReportQuery(db, kind)
	if err != nil {
		return row, err
	}
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err = q.Select("id, project, resolved, message").Where("id = ?", id).Take(&row).Error
	if err == nil && NormalizeProjectID(row.Project) != project {
		err = gorm.ErrRecordNotFound
	}
	return row, err
}

func AppendReportNote(db *gorm.DB, kind string, id uint, project, actor string, in AppendReportNoteInput) (ReportNote, bool, error) {
	var note ReportNote
	replay := false
	if id == 0 || project != NormalizeProjectID(project) || len(project) > 64 || (actor != "admin" && actor != "mcp") {
		return note, false, ErrNoteInput
	}
	if err := ValidateReportNote(in); err != nil {
		return note, false, err
	}
	raw, _ := json.Marshal(struct {
		Actor string
		Input AppendReportNoteInput
	}{actor, in})
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := FindNoteReport(tx, kind, id, project, true); err != nil {
			return err
		}
		err := tx.Where("kind = ? AND report_id = ? AND request_key = ?", kind, id, in.RequestKey).Take(&note).Error
		if err == nil {
			if note.PayloadHash != digest {
				return ErrNoteConflict
			}
			replay = true
			return tx.Preload("Evidence").First(&note, note.ID).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if kind == "crash" {
			return ErrCrashGroupRequired
		}
		note = ReportNote{Kind: kind, ReportID: id, Project: project, Actor: actor, CreatedAt: time.Now().UTC(), RequestKey: in.RequestKey, PayloadHash: digest, Note: in.Note, State: in.State, ReleaseState: in.ReleaseState}
		if err := tx.Create(&note).Error; err != nil {
			return err
		}
		if e := in.Evidence; e != nil {
			note.Evidence = &ReportNoteEvidence{NoteID: note.ID, Name: e.Name, SHA256: e.SHA256, Bytes: len(e.Content), Content: e.Content}
			if err := tx.Create(note.Evidence).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if note.Evidence != nil {
		note.Evidence.Content = ""
	}
	return note, replay, err
}
